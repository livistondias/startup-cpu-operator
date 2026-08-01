package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	autoscalingv1 "github.com/platform/startup-cpu-operator/api/v1"
)

const (
	ResizedAnnotation = "startup-cpu-operator/resized"
)

type StartupCPUBoostReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Clientset       *kubernetes.Clientset
	resizeSemaphore chan struct{}
}

func NewStartupCPUBoostReconciler(client client.Client, scheme *runtime.Scheme, config *rest.Config) (*StartupCPUBoostReconciler, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &StartupCPUBoostReconciler{
		Client:          client,
		Scheme:          scheme,
		Clientset:       clientset,
		resizeSemaphore: make(chan struct{}, 10),
	}, nil
}

// +kubebuilder:rbac:groups=autoscaling.platform.io,resources=startupcpuboosts,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling.platform.io,resources=startupcpuboosts/status,verbs=update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=pods/resize,verbs=update

func (r *StartupCPUBoostReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var policy autoscalingv1.StartupCPUBoost
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	podsProcessed := int32(0)

	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.Selector)
	if err != nil {
		log.Error(err, "invalid selector", "policy", policy.Name)
		if err := r.updateStatus(ctx, &policy, "InvalidSelector", metav1.ConditionFalse, err.Error()); err != nil {
			log.Error(err, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods, &client.ListOptions{LabelSelector: selector}); err != nil {
		log.Error(err, "unable to list pods", "policy", policy.Name)
		if err := r.updateStatus(ctx, &policy, "ListPodsFailed", metav1.ConditionFalse, err.Error()); err != nil {
			log.Error(err, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	for _, pod := range pods.Items {
		processed, err := r.processPod(ctx, &pod, &policy)
		if err != nil {
			log.Error(err, "failed to process pod", "pod", pod.Name, "namespace", pod.Namespace)
		}
		if processed {
			podsProcessed++
		}
	}

	now := metav1.Now()
	policyUpdate := policy.DeepCopy()
	policyUpdate.Status.PodsProcessed = policy.Status.PodsProcessed + podsProcessed
	policyUpdate.Status.LastReconcileTime = &now
	policyUpdate.Status.ObservedGeneration = policy.Generation
	if err := r.updateStatus(ctx, policyUpdate, "ReconcileSuccess", metav1.ConditionTrue, "Reconciliation completed"); err != nil {
		log.Error(err, "failed to update status")
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (r *StartupCPUBoostReconciler) updateStatus(ctx context.Context, policy *autoscalingv1.StartupCPUBoost, reason string, status metav1.ConditionStatus, message string) error {
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	found := false
	for i, c := range policy.Status.Conditions {
		if c.Type == "Ready" {
			if c.Status == condition.Status {
				condition.LastTransitionTime = c.LastTransitionTime
			}
			policy.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		policy.Status.Conditions = append(policy.Status.Conditions, condition)
	}

	return r.Status().Update(ctx, policy)
}

func (r *StartupCPUBoostReconciler) processPod(ctx context.Context, pod *corev1.Pod, policy *autoscalingv1.StartupCPUBoost) (bool, error) {
	log := log.FromContext(ctx)

	if pod.Status.Phase != corev1.PodRunning {
		return false, nil
	}

	if !isPodReady(pod) {
		return false, nil
	}

	if pod.Annotations[ResizedAnnotation] == "true" {
		return false, nil
	}

	if pod.Status.StartTime == nil {
		return false, nil
	}

	elapsed := time.Since(pod.Status.StartTime.Time).Seconds()
	if elapsed < float64(policy.Spec.WarmupSeconds) {
		return false, nil
	}

	cpuRequest, err := resource.ParseQuantity(policy.Spec.RuntimeCPU)
	if err != nil {
		return false, err
	}

	cpuLimit := cpuRequest
	if policy.Spec.RuntimeCPULimit != "" {
		cpuLimit, err = resource.ParseQuantity(policy.Spec.RuntimeCPULimit)
		if err != nil {
			return false, err
		}
	}

	containerIdx := 0
	if policy.Spec.ContainerName != "" {
		found := false
		for i, c := range pod.Spec.Containers {
			if c.Name == policy.Spec.ContainerName {
				containerIdx = i
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Errorf("container %s not found in pod %s", policy.Spec.ContainerName, pod.Name)
		}
	}

	currentCPU := pod.Spec.Containers[containerIdx].Resources.Requests.Cpu()
	if currentCPU != nil && currentCPU.Cmp(cpuRequest) == 0 {
		return false, nil
	}

	r.resizeSemaphore <- struct{}{}
	defer func() { <-r.resizeSemaphore }()

	podToUpdate, err := r.Clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if podToUpdate.Spec.Containers[containerIdx].Resources.Requests == nil {
		podToUpdate.Spec.Containers[containerIdx].Resources.Requests = corev1.ResourceList{}
	}
	if podToUpdate.Spec.Containers[containerIdx].Resources.Limits == nil {
		podToUpdate.Spec.Containers[containerIdx].Resources.Limits = corev1.ResourceList{}
	}
	podToUpdate.Spec.Containers[containerIdx].Resources.Requests[corev1.ResourceCPU] = cpuRequest
	podToUpdate.Spec.Containers[containerIdx].Resources.Limits[corev1.ResourceCPU] = cpuLimit

	result := r.Clientset.CoreV1().RESTClient().
		Put().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("resize").
		Body(podToUpdate).
		Do(ctx)

	if err := result.Error(); err != nil {
		return false, err
	}

	updatedPod, err := r.Clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "failed to get pod after resize", "pod", pod.Name)
		log.Info("CPU resized", "pod", pod.Name, "namespace", pod.Namespace, "cpu", policy.Spec.RuntimeCPU)
		return true, nil
	}

	if updatedPod.Annotations == nil {
		updatedPod.Annotations = make(map[string]string)
	}
	updatedPod.Annotations[ResizedAnnotation] = "true"

	_, err = r.Clientset.CoreV1().Pods(pod.Namespace).Update(ctx, updatedPod, metav1.UpdateOptions{})
	if err != nil {
		log.Error(err, "failed to add annotation after resize", "pod", pod.Name)
	}

	log.Info("CPU resized", "pod", pod.Name, "namespace", pod.Namespace, "cpu", policy.Spec.RuntimeCPU)
	return true, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// podToPolicy maps a Pod event to the StartupCPUBoost policies that select it.
func (r *StartupCPUBoostReconciler) podToPolicy(ctx context.Context, obj client.Object) []reconcile.Request {
	var policies autoscalingv1.StartupCPUBoostList
	if err := r.List(ctx, &policies); err != nil {
		return nil
	}
	var requests []reconcile.Request
	for _, p := range policies.Items {
		sel, err := metav1.LabelSelectorAsSelector(&p.Spec.Selector)
		if err != nil {
			continue
		}
		if sel.Matches(labels.Set(obj.GetLabels())) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: p.Name},
			})
		}
	}
	return requests
}

func (r *StartupCPUBoostReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&autoscalingv1.StartupCPUBoost{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.podToPolicy),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
			RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(
				time.Second,
				time.Minute,
			),
		}).
		Complete(r)
}
