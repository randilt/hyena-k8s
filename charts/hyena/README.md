# Hyena Kubernetes Secret Management Helm Chart

This Helm chart deploys the Hyena decentralized secret management system using Shamir's Secret Sharing threshold cryptography.

## Overview

Hyena distributes secrets across multiple share servers using threshold cryptography. Applications reconstruct secrets at runtime using an init container sidecar pattern, with secrets stored only in memory (tmpfs).

**Key Features**:
- **Threshold Security**: Requires K-of-N shares to reconstruct secrets
- **No Persistent Storage**: Secrets never written to disk
- **Runtime Reconstruction**: Secrets assembled only when needed
- **Kubernetes-Native Authentication**: Uses ServiceAccount JWT tokens
- **Fault Tolerance**: Survives N-K server failures

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- At least 3 worker nodes (for production-like deployment)
- `kubectl` configured to access your cluster

## Installation

### Quick Start

```bash
# 1. Generate secret shares
./scripts/generate-shares.sh "my-super-secret-password" 5 3

# 2. Install the chart
helm install hyena ./charts/hyena \
  --set shareServer.shares.share-0="$(cat share-0.bin | base64 -w0)" \
  --set shareServer.shares.share-1="$(cat share-1.bin | base64 -w0)" \
  --set shareServer.shares.share-2="$(cat share-2.bin | base64 -w0)" \
  --set shareServer.shares.share-3="$(cat share-3.bin | base64 -w0)" \
  --set shareServer.shares.share-4="$(cat share-4.bin | base64 -w0)"
```

### Using Values File

Create a `custom-values.yaml`:

```yaml
shareServer:
  replicaCount: 5
  
  shares:
    share-0: "<base64-encoded-share-0>"
    share-1: "<base64-encoded-share-1>"
    share-2: "<base64-encoded-share-2>"
    share-3: "<base64-encoded-share-3>"
    share-4: "<base64-encoded-share-4>"
  
  config:
    threshold: 3
    allowedCallers:
      - "default:demo-app"
    devMode: false  # MUST be false in production!
  
  image:
    tag: "latest"
    pullPolicy: IfNotPresent

demoApp:
  enabled: true
  
  sidecar:
    threshold: 3
  
  image:
    tag: "latest"
```

Then install:

```bash
helm install hyena ./charts/hyena -f custom-values.yaml
```

## Configuration

### Share Server Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `shareServer.replicaCount` | Number of share servers (N) | `5` |
| `shareServer.config.threshold` | Shares needed for reconstruction (K) | `3` |
| `shareServer.config.allowedCallers` | List of `namespace:serviceaccount` allowed to fetch shares | `["default:demo-app"]` |
| `shareServer.config.devMode` | Skip JWT signature validation (DEV ONLY!) | `true` |
| `shareServer.image.repository` | Share server image | `hyena-share-server` |
| `shareServer.image.tag` | Image tag | `latest` |
| `shareServer.image.pullPolicy` | Pull policy | `IfNotPresent` |
| `shareServer.resources.limits.cpu` | CPU limit | `100m` |
| `shareServer.resources.limits.memory` | Memory limit | `64Mi` |
| `shareServer.service.port` | gRPC service port | `9000` |
| `shareServer.shares.share-X` | Base64-encoded share data (X = 0 to N-1) | Required |

### Sidecar Reconstructor Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `demoApp.sidecar.threshold` | Shares needed (must match server threshold) | `3` |
| `demoApp.sidecar.timeout` | Timeout for share fetching | `30s` |
| `demoApp.sidecar.maxRetries` | Retries per server | `3` |
| `demoApp.sidecar.image.repository` | Sidecar image | `hyena-sidecar` |
| `demoApp.sidecar.image.tag` | Image tag | `latest` |
| `demoApp.sidecar.resources.limits.cpu` | CPU limit | `100m` |
| `demoApp.sidecar.resources.limits.memory` | Memory limit | `64Mi` |

### Demo App Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `demoApp.enabled` | Deploy demo application | `true` |
| `demoApp.image.repository` | Demo app image | `demo-app` |
| `demoApp.image.tag` | Image tag | `latest` |
| `demoApp.replicaCount` | Number of replicas | `1` |
| `demoApp.service.type` | Service type | `NodePort` |
| `demoApp.service.port` | HTTP port | `8080` |
| `demoApp.service.nodePort` | NodePort (if type=NodePort) | `30080` |

## Usage Examples

### Example 1: Production Configuration (5-of-7)

```yaml
# production-values.yaml
shareServer:
  replicaCount: 7
  config:
    threshold: 5
    devMode: false  # MUST be false!
    allowedCallers:
      - "production:payment-service"
      - "production:user-service"
  
  resources:
    limits:
      cpu: 200m
      memory: 128Mi
    requests:
      cpu: 100m
      memory: 64Mi
  
  # Distribute across zones
  affinity:
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app.kubernetes.io/component: share-server
        topologyKey: topology.kubernetes.io/zone

demoApp:
  enabled: false  # Disable demo app in production
```

### Example 2: Development Configuration (2-of-3)

```yaml
# dev-values.yaml
shareServer:
  replicaCount: 3
  config:
    threshold: 2
    devMode: true
    allowedCallers:
      - "*:*"  # Allow all (dev only!)
  
  resources:
    limits:
      cpu: 50m
      memory: 32Mi

demoApp:
  enabled: true
  sidecar:
    threshold: 2
```

### Example 3: Custom Application

To use Hyena with your own application:

1. **Add the sidecar init container** to your Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      serviceAccountName: my-app-sa
      
      initContainers:
      - name: sidecar-reconstructor
        image: hyena-sidecar:latest
        env:
        - name: ENDPOINTS
          value: "hyena-share-server-0.hyena-share-server:9000,hyena-share-server-1.hyena-share-server:9000,hyena-share-server-2.hyena-share-server:9000"
        - name: THRESHOLD
          value: "3"
        - name: SECRET_PATH
          value: "/secrets/app-secret"
        - name: TIMEOUT
          value: "30s"
        - name: MAX_RETRIES
          value: "3"
        volumeMounts:
        - name: secrets
          mountPath: /secrets
      
      containers:
      - name: my-app
        image: my-app:latest
        volumeMounts:
        - name: secrets
          mountPath: /secrets
          readOnly: true
        # Your app reads from /secrets/app-secret
      
      volumes:
      - name: secrets
        emptyDir:
          medium: Memory  # tmpfs - never written to disk!
```

2. **Add your ServiceAccount to allowedCallers**:

```bash
helm upgrade hyena ./charts/hyena \
  --reuse-values \
  --set shareServer.config.allowedCallers="{default:demo-app,default:my-app-sa}"
```

## Generating Shares

Use the provided script:

```bash
# Generate 5 shares with threshold 3
./scripts/generate-shares.sh "my-secret-value" 5 3

# This creates:
# - share-0.bin through share-4.bin
# - share-0.b64 through share-4.b64 (base64 encoded)
```

**Manual generation**:

```bash
# Build split-secret tool
make build

# Generate shares
./bin/split-secret \
  --secret "my-secret-value" \
  --threshold 3 \
  --total 5 \
  --base64
```

## Upgrading

### Rotating Shares

To change the secret:

```bash
# 1. Generate new shares
./scripts/generate-shares.sh "new-secret-value" 5 3

# 2. Upgrade with new shares
helm upgrade hyena ./charts/hyena \
  --reuse-values \
  --set shareServer.shares.share-0="$(cat share-0.b64)" \
  --set shareServer.shares.share-1="$(cat share-1.b64)" \
  --set shareServer.shares.share-2="$(cat share-2.b64)" \
  --set shareServer.shares.share-3="$(cat share-3.b64)" \
  --set shareServer.shares.share-4="$(cat share-4.b64)"

# 3. Restart share servers
kubectl rollout restart statefulset/hyena-share-server

# 4. Restart consuming apps to fetch new shares
kubectl rollout restart deployment/hyena-demo-app
```

### Changing N or K

**⚠️ WARNING**: Changing N or K requires regenerating shares!

```bash
# 1. Generate new shares with new N and K
./scripts/generate-shares.sh "my-secret" 7 5

# 2. Update configuration
helm upgrade hyena ./charts/hyena \
  --set shareServer.replicaCount=7 \
  --set shareServer.config.threshold=5 \
  --set demoApp.sidecar.threshold=5 \
  --set shareServer.shares.share-0="$(cat share-0.b64)" \
  --set shareServer.shares.share-1="$(cat share-1.b64)" \
  # ... all 7 shares
```

## Monitoring

### Health Checks

```bash
# Check all pods
kubectl get pods -l app.kubernetes.io/instance=hyena

# Check share server health
kubectl exec hyena-share-server-0 -- \
  grpcurl -plaintext localhost:9000 \
  shareservice.v1.ShareService/HealthCheck
```

### Logs

```bash
# Share server logs
kubectl logs -l app.kubernetes.io/component=share-server --tail=20

# Sidecar logs
kubectl logs -l app.kubernetes.io/component=demo-app -c sidecar-reconstructor

# Demo app logs
kubectl logs -l app.kubernetes.io/component=demo-app -c demo-app
```

### Metrics

Currently, no built-in metrics are exposed. Consider adding:
- Prometheus annotations
- Custom metrics exporter
- OpenTelemetry instrumentation

## Uninstallation

```bash
# Remove Helm release
helm uninstall hyena

# Clean up secrets (if not auto-deleted)
kubectl delete secret hyena-shares

# Verify cleanup
kubectl get all -l app.kubernetes.io/instance=hyena
```

## Security Considerations

### ⚠️ Production Warnings

This is a **PROOF-OF-CONCEPT**. Before production use:

1. **DISABLE devMode**: Set `shareServer.config.devMode: false`
2. **Enable TLS**: Implement proper TLS between components
3. **Rotate Shares Regularly**: Implement secret rotation policy
4. **Audit Logging**: Add comprehensive audit trails
5. **Access Control**: Restrict ServiceAccount permissions
6. **Network Policies**: Isolate share servers
7. **Resource Limits**: Set appropriate limits for your workload
8. **High Availability**: Deploy across multiple zones
9. **Backup Strategy**: Secure offline backup of shares
10. **Compliance Review**: Consult security team for compliance requirements

### Authentication

- Uses Kubernetes ServiceAccount JWT tokens
- Validates `iss`, `sub`, `exp`, `nbf` claims
- In devMode, signature verification is **SKIPPED** ⚠️
- Production MUST set `devMode: false`

### Threat Model

See [docs/threat-model.md](../../docs/threat-model.md) for detailed security analysis.

## Troubleshooting

### Problem: Pods Stuck in Init:0/1

**Symptoms**: Demo app pod shows `Init:0/1` status.

**Causes**:
- Share servers not ready
- Network connectivity issues
- Threshold not met

**Debug**:
```bash
kubectl describe pod -l app.kubernetes.io/component=demo-app
kubectl logs -l app.kubernetes.io/component=demo-app -c sidecar-reconstructor
```

### Problem: "Failed to fetch shares"

**Symptoms**: Sidecar logs show fetch failures.

**Causes**:
- Share servers unreachable
- Authentication failures
- Timeout too short

**Solutions**:
```bash
# Check share server status
kubectl get pods -l app.kubernetes.io/component=share-server

# Test connectivity
kubectl exec -it hyena-demo-app-xxx -c demo-app -- \
  nslookup hyena-share-server-0.hyena-share-server

# Increase timeout
helm upgrade hyena ./charts/hyena --reuse-values \
  --set demoApp.sidecar.timeout=60s
```

### Problem: "Unauthorized" errors

**Symptoms**: Sidecar logs show gRPC Unauthenticated errors.

**Causes**:
- ServiceAccount not in allowedCallers
- JWT token issues
- devMode=false but no proper validation

**Solutions**:
```bash
# Add ServiceAccount to allowedCallers
helm upgrade hyena ./charts/hyena --reuse-values \
  --set shareServer.config.allowedCallers="{default:demo-app,yournamespace:yoursa}"

# Restart share servers
kubectl rollout restart statefulset/hyena-share-server
```

### Problem: Share reconstruction fails

**Symptoms**: "Failed to reconstruct secret" in sidecar logs.

**Causes**:
- Shares from different secret splits
- Corrupted share data
- Threshold mismatch

**Solutions**:
```bash
# Regenerate shares
./scripts/generate-shares.sh "correct-secret" 5 3

# Update all shares
helm upgrade hyena ./charts/hyena --reuse-values \
  --set shareServer.shares.share-0="$(cat share-0.b64)" \
  # ... update all shares
```

## Further Reading

- [Main README](../../README.md) - Project overview
- [Architecture](../../docs/architecture.md) - System design
- [Threat Model](../../docs/threat-model.md) - Security analysis
- [Demo Walkthrough](../../docs/demo.md) - Step-by-step testing
- [Implementation Plan](../../IMPLEMENTATION_PLAN.md) - Development roadmap

## Support

This is a proof-of-concept project for educational purposes. For production use cases, consult with your security team and consider commercial secret management solutions.

## License

See the [LICENSE](../../LICENSE) file in the root of the repository.
