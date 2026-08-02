# Startup CPU Boost Operator

Kubernetes Operator que reduz automaticamente o CPU request/limit de Pods após o período de warmup usando In-Place Pod Resize (Kubernetes ≥ 1.33).

## Problema

Aplicações como Java/JVM consomem muito CPU durante startup (~800-1200m) mas pouco em runtime (~100-200m).

## Solução

Este operator aplica o padrão **Startup CPU Boost**:
- Pod inicia com CPU alto
- Após warmup, CPU é reduzido automaticamente via subresource `/resize`
- Sem restart do Pod

## Pré-requisitos

- Kubernetes ≥ 1.33
- Feature gate `InPlacePodVerticalScaling=true` (beta, habilitado por padrão no K8s 1.33+)
- `kubectl` configurado com acesso ao cluster

### Verificar se o feature gate está ativo

```bash
kubectl get node -o jsonpath='{.items[0].status.allocatable}' | grep -c cpu
# Se o cluster for < 1.33, o resize não funcionará

# Verificar versão do cluster
kubectl version --short
```

Se precisar habilitar manualmente (kubeadm):
```bash
# Em kube-apiserver e kubelet, adicionar:
--feature-gates=InPlacePodVerticalScaling=true
```

## Instalação

```bash
# Instalar CRD
kubectl apply -f https://raw.githubusercontent.com/livistondias/startup-cpu-operator/main/src/config/crd/bases/startupcpuboosts.yaml

# Instalar RBAC
kubectl apply -f https://raw.githubusercontent.com/livistondias/startup-cpu-operator/main/src/config/rbac/role.yaml

# Deploy do operator
kubectl apply -f https://raw.githubusercontent.com/livistondias/startup-cpu-operator/main/src/config/manager/deployment.yaml
```

### Verificar se o operator subiu

```bash
kubectl rollout status deployment/startup-cpu-operator -n kube-system

# Verificar logs
kubectl logs -n kube-system deployment/startup-cpu-operator --tail=20
```

Saída esperada:
```
{"level":"info","msg":"starting manager"}
{"level":"info","msg":"Starting EventSource"}
{"level":"info","msg":"Starting Controller"}
```

## Uso

### 1. Criar política

```yaml
apiVersion: autoscaling.platform.io/v1
kind: StartupCPUBoost
metadata:
  name: oferta-cpu-policy
spec:
  selector:
    matchLabels:
      app: oferta
  runtimeCPU: "200m"          # CPU request após warmup
  runtimeCPULimit: "400m"     # CPU limit após warmup (opcional)
  warmupSeconds: 120
  containerName: oferta       # opcional, usa primeiro container se omitido
```

**Campos:**
- `runtimeCPU` - CPU request após warmup (obrigatório)
- `runtimeCPULimit` - CPU limit após warmup (opcional, se omitido usa valor de runtimeCPU)
- `warmupSeconds` - Tempo de espera após Pod ficar Ready
- `containerName` - Nome do container alvo (opcional)

```bash
kubectl apply -f https://raw.githubusercontent.com/livistondias/startup-cpu-operator/main/src/example-policy.yaml
```

### 2. Configurar workload

> ⚠️ O Pod precisa declarar `resizePolicy` com `restartPolicy: NotRequired` para o resize funcionar sem reiniciar o container.

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: oferta
spec:
  containers:
  - name: oferta
    image: eclipse-temurin:21-jre
    resources:
      requests:
        cpu: "1000m"
        memory: "512Mi"
      limits:
        cpu: "1000m"
        memory: "512Mi"
    resizePolicy:
    - resourceName: cpu
      restartPolicy: NotRequired
```

```bash
kubectl apply -f https://raw.githubusercontent.com/livistondias/startup-cpu-operator/main/src/example-workload.yaml
```

### 3. Verificar status

```bash
# Listar políticas
kubectl get startupcpuboosts
kubectl get scpub  # shortname

# Output exemplo:
# NAME                 RUNTIME CPU   WARMUP   PODS PROCESSED   AGE
# oferta-cpu-policy    200m          120      5                10m

# Ver detalhes e conditions
kubectl describe startupcpuboost oferta-cpu-policy
```

### 4. Confirmar que o resize foi aplicado

```bash
# Ver CPU atual do pod após o warmup
kubectl get pod oferta-app -o jsonpath='{.spec.containers[0].resources.requests.cpu}'
# Esperado: 200m

# Verificar annotation de controle
kubectl get pod oferta-app -o jsonpath='{.metadata.annotations.startup-cpu-operator/resized}'
# Esperado: true
```

## Desinstalação

```bash
kubectl delete -f https://raw.githubusercontent.com/livistondias/startup-cpu-operator/main/src/config/manager/deployment.yaml
kubectl delete -f https://raw.githubusercontent.com/livistondias/startup-cpu-operator/main/src/config/rbac/role.yaml
kubectl delete -f https://raw.githubusercontent.com/livistondias/startup-cpu-operator/main/src/config/crd/bases/startupcpuboosts.yaml
```

> Deletar o CRD remove automaticamente todos os objetos `StartupCPUBoost` do cluster.

## Desenvolvimento

```bash
cd src

# Build local
make build

# Run local (requer kubeconfig)
make run

# Testes
make test

# Build Docker
make docker-build

# Deploy completo
make deploy
```

## CI/CD

O repositório usa **GitHub Actions** para build e push automático da imagem no GHCR:

- Push na `main` → publica `ghcr.io/livistondias/startup-cpu-operator:latest`
- Tag `v1.2.3` → publica `1.2.3`, `1.2`, `1` e `latest`
- Pull Request → apenas build (sem push)

Build multi-arch: `linux/amd64` e `linux/arm64`.

## Como funciona

1. Operator lista todas as políticas `StartupCPUBoost`
2. Para cada política, seleciona Pods usando label selector
3. Valida se Pod está Running e Ready
4. Aguarda `warmupSeconds` após Pod iniciar
5. Executa resize via subresource `/resize` (sem restart)
6. Marca Pod com annotation `startup-cpu-operator/resized=true`
7. Atualiza status da política com métricas
8. Reconcilia a cada 1 minuto ou em mudanças de Pod

## Observabilidade

### Status Conditions

```bash
kubectl get scpub oferta-cpu-policy -o jsonpath='{.status.conditions}'
```

Tipos:
- `Ready=True` - Reconciliação bem-sucedida
- `Ready=False` - Erro (ver reason/message)

### Métricas no Status

- `podsProcessed` - Total acumulado de pods com resize aplicado
- `observedGeneration` - Última geração reconciliada
- `lastReconcileTime` - Timestamp da última reconciliação

### Logs

```bash
kubectl logs -n kube-system deployment/startup-cpu-operator -f
```

## Troubleshooting

### Pod não teve CPU reduzido

1. Verificar se Pod tem label correto:
```bash
kubectl get pod <pod-name> --show-labels
```

2. Verificar se passou o warmup:
```bash
kubectl get pod <pod-name> -o jsonpath='{.status.startTime}'
```

3. Verificar annotation:
```bash
kubectl get pod <pod-name> -o jsonpath='{.metadata.annotations}'
```

4. Verificar logs do operator:
```bash
kubectl logs -n kube-system deployment/startup-cpu-operator
```

### Resize não funciona (sem erro nos logs)

Verificar se o feature gate está habilitado no cluster:
```bash
kubectl get node -o yaml | grep -i inplace
```

Verificar se Pod tem `resizePolicy` configurado:
```bash
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[0].resizePolicy}'
```

### Container não encontrado

Se especificar `containerName`, garantir que existe no Pod:
```bash
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].name}'
```

## Limitações

- Apenas CPU é suportado (não memory)
- Resize ocorre apenas uma vez por Pod (via annotation)
- Requer `resizePolicy.restartPolicy=NotRequired` no Pod
- Scope Cluster (não Namespaced) — uma política afeta pods em qualquer namespace
- Se `runtimeCPULimit` não especificado, limit = request

## Casos de uso

**1. Request e Limit iguais (QoS Guaranteed)**
```yaml
spec:
  runtimeCPU: "200m"
  # runtimeCPULimit omitido = usa 200m
```

**2. Request menor que Limit (QoS Burstable)**
```yaml
spec:
  runtimeCPU: "200m"
  runtimeCPULimit: "500m"
```

**3. Container específico em pod multi-container**
```yaml
spec:
  containerName: "app"
  runtimeCPU: "100m"
```

## Arquitetura

```
StartupCPUBoost (CRD, cluster-scoped)
        ↓
Operator observa Pods via label selector
        ↓
Pod Running + Ready + warmupSeconds decorridos
        ↓
PUT /resize → spec.containers[].resources
        ↓
CPU reduzido sem restart
        ↓
Annotation resized=true + status atualizado
```