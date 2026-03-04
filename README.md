# Startup CPU Operator

Kubernetes Operator que reduz automaticamente o CPU request/limit de Pods após o período de warmup usando In-Place Pod Resize (Kubernetes ≥ 1.33).

## Problema

Aplicações como Java/JVM consomem muito CPU durante startup (~800-1200m) mas pouco em runtime (~100-200m).

## Solução

Este operator aplica o padrão **Startup CPU Boost**:
- Pod inicia com CPU alto
- Após warmup, CPU é reduzido automaticamente
- Sem restart do Pod

## Requisitos

- Kubernetes ≥ 1.33
- Feature gate `InPlacePodVerticalScaling=true` (beta por padrão)
- Go ≥ 1.22 (para desenvolvimento)

## Instalação

```bash
# Instalar CRD
kubectl apply -f src/config/crd/bases/startupcpuboosts.yaml

# Instalar RBAC
kubectl apply -f src/config/rbac/role.yaml

# Deploy do operator
kubectl apply -f src/config/manager/deployment.yaml
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
kubectl apply -f example-policy.yaml
```

### 2. Configurar workload

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    app: oferta
spec:
  containers:
  - name: oferta
    image: your-app:latest
    resources:
      requests:
        cpu: "1000m"
      limits:
        cpu: "1000m"
    resizePolicy:
    - resourceName: cpu
      restartPolicy: NotRequired
```

```bash
kubectl apply -f example-workload.yaml
```

### 3. Verificar status

```bash
# Listar políticas
kubectl get startupcpuboosts
kubectl get scpub  # shortname

# Output exemplo:
# NAME                 RUNTIME CPU   CPU LIMIT   WARMUP   PODS PROCESSED   AGE
# oferta-cpu-policy    200m          400m        120      5                10m

# Ver detalhes
kubectl describe startupcpuboost oferta-cpu-policy

# Verificar pods processados
kubectl get scpub -o wide
```

## Desenvolvimento

```bash
cd src

# Build local
make build

# Run local (requer kubeconfig)
make run

# Build Docker
make docker-build

# Deploy completo
make deploy
```

## Pipeline CI/CD

O repositório inclui Azure Pipeline (`azure-pipelines.yml`) com 3 stages:

1. **Build** - Compila e testa o operator
2. **Docker** - Build e push da imagem
3. **Deploy** - Aplica no cluster Kubernetes

**Variáveis necessárias:**
- `DOCKER_REGISTRY_CONNECTION` - Service connection do registry
- `K8S_CONNECTION` - Service connection do cluster

## Como funciona

1. Operator lista todas as políticas `StartupCPUBoost`
2. Para cada política, seleciona Pods usando label selector
3. Valida se Pod está Running e Ready
4. Aguarda `warmupSeconds` após Pod iniciar
5. Executa patch em `spec.containers[].resources`
6. Marca Pod com annotation `startup-cpu-operator/resized=true`
7. Atualiza status da política com métricas
8. Reconcilia a cada 30s ou quando há mudanças

## Observabilidade

### Status Conditions

```bash
kubectl get scpub oferta-cpu-policy -o jsonpath='{.status.conditions}'
```

Tipos:
- `Ready=True` - Reconciliação bem-sucedida
- `Ready=False` - Erro (ver reason/message)

### Métricas no Status

- `podsProcessed` - Total de pods com resize aplicado
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

### Container não encontrado

Se especificar `containerName`, garantir que existe no Pod:
```bash
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].name}'
```

### Resize não funciona

Verificar se Pod tem `resizePolicy`:
```bash
kubectl get pod <pod-name> -o jsonpath='{.spec.containers[0].resizePolicy}'
```

## Limitações

- Apenas CPU é suportado (não memory)
- Resize ocorre apenas uma vez por Pod (via annotation)
- Requer `resizePolicy.restartPolicy=NotRequired` no Pod
- Scope Cluster (não Namespaced)
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

**3. Múltiplos containers**
```yaml
spec:
  containerName: "app"  # especifica qual container
  runtimeCPU: "100m"
```

## Arquitetura

```
StartupCPUBoost (CRD)
        ↓
Operator observa Pods
        ↓
Pod Ready + warmup
        ↓
PATCH spec.containers[].resources
        ↓
CPU reduzido (no restart)
```

## Critérios de aceitação

✅ Pods iniciam com CPU alto  
✅ Após warmup CPU é reduzido automaticamente  
✅ Nenhum restart ocorre  
✅ Resize ocorre apenas uma vez por Pod  
✅ Operator suporta múltiplas políticas  
✅ Status reporta métricas e conditions  
✅ Validações robustas no CRD  
✅ RBAC com permissões mínimas


