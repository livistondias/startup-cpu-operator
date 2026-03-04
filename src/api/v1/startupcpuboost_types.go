package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// StartupCPUBoostSpec defines the desired state of StartupCPUBoost
type StartupCPUBoostSpec struct {
	Selector         metav1.LabelSelector `json:"selector"`
	RuntimeCPU       string               `json:"runtimeCPU"`
	RuntimeCPULimit  string               `json:"runtimeCPULimit,omitempty"`
	WarmupSeconds    int32                `json:"warmupSeconds"`
	ContainerName    string               `json:"containerName,omitempty"`
}

// StartupCPUBoostStatus defines the observed state of StartupCPUBoost
type StartupCPUBoostStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	PodsProcessed      int32              `json:"podsProcessed,omitempty"`
	LastReconcileTime  *metav1.Time       `json:"lastReconcileTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=autoscaling
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Runtime CPU",type=string,JSONPath=`.spec.runtimeCPU`
// +kubebuilder:printcolumn:name="Warmup",type=integer,JSONPath=`.spec.warmupSeconds`
// +kubebuilder:printcolumn:name="Pods Processed",type=integer,JSONPath=`.status.podsProcessed`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// StartupCPUBoost is the Schema for the startupcpuboosts API
type StartupCPUBoost struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StartupCPUBoostSpec   `json:"spec,omitempty"`
	Status StartupCPUBoostStatus `json:"status,omitempty"`
}

// DeepCopyObject implements runtime.Object
func (in *StartupCPUBoost) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy creates a deep copy of StartupCPUBoost
func (in *StartupCPUBoost) DeepCopy() *StartupCPUBoost {
	if in == nil {
		return nil
	}
	out := new(StartupCPUBoost)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *StartupCPUBoost) DeepCopyInto(out *StartupCPUBoost) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopyInto copies all properties of StartupCPUBoostSpec
func (in *StartupCPUBoostSpec) DeepCopyInto(out *StartupCPUBoostSpec) {
	*out = *in
	in.Selector.DeepCopyInto(&out.Selector)
}

// DeepCopyInto copies all properties of StartupCPUBoostStatus
func (in *StartupCPUBoostStatus) DeepCopyInto(out *StartupCPUBoostStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastReconcileTime != nil {
		in, out := &in.LastReconcileTime, &out.LastReconcileTime
		*out = (*in).DeepCopy()
	}
}

// +kubebuilder:object:root=true

// StartupCPUBoostList contains a list of StartupCPUBoost
type StartupCPUBoostList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StartupCPUBoost `json:"items"`
}

// DeepCopyObject implements runtime.Object
func (in *StartupCPUBoostList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy creates a deep copy of StartupCPUBoostList
func (in *StartupCPUBoostList) DeepCopy() *StartupCPUBoostList {
	if in == nil {
		return nil
	}
	out := new(StartupCPUBoostList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties of this object into another object of the same type
func (in *StartupCPUBoostList) DeepCopyInto(out *StartupCPUBoostList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]StartupCPUBoost, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func init() {
	SchemeBuilder.Register(&StartupCPUBoost{}, &StartupCPUBoostList{})
}
