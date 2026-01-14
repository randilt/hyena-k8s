# Demo Walkthrough

This guide walks through a complete demonstration of Hyena-K8s.

## Prerequisites

Ensure you have completed the setup:

```bash
# 1. Start minikube
make setup-minikube

# 2. Configure Docker to use minikube
eval $(minikube docker-env)

# 3. Build and deploy
make deploy
```

## Demo Scenario

### Scenario 1: Normal Operation (All Servers Available)

**Expectation**: Application starts successfully and can access the secret.

```bash
# Wait for all pods to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/instance=hyena \
  --timeout=120s

# Check pod status
kubectl get pods -l app.kubernetes.io/instance=hyena

# Expected output:
# NAME                          READY   STATUS    RESTARTS   AGE
# hyena-demo-app-xxx            1/1     Running   0          30s
# hyena-share-server-0          1/1     Running   0          30s
# hyena-share-server-1          1/1     Running   0          30s
# hyena-share-server-2          1/1     Running   0          30s
# hyena-share-server-3          1/1     Running   0          30s
# hyena-share-server-4          1/1     Running   0          30s
```

**Access the application**:

```bash
# Get the service URL
minikube service hyena-demo-app --url

# Visit in browser or curl
curl $(minikube service hyena-demo-app --url)/status
```

**Expected JSON response**:
```json
{
  "secret_loaded": true,
  "secret_length": 32,
  "message": "Secret loaded and available"
}
```

**✅ SUCCESS**: All 5 share servers are available, threshold (3) is met.

---

### Scenario 2: Resilience (One Server Down)

**Expectation**: Application still works with 4 servers (above threshold).

```bash
# Kill one share server
kubectl delete pod hyena-share-server-0

# Wait for it to be recreated
kubectl wait --for=condition=ready pod hyena-share-server-0 --timeout=60s

# Restart the demo app to test fetching with one server temporarily unavailable
kubectl delete pod -l app.kubernetes.io/component=demo-app

# Wait for new pod
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/component=demo-app \
  --timeout=120s

# Check status
curl $(minikube service hyena-demo-app --url)/status
```

**Expected result**: Still shows `"secret_loaded": true`

**Why it works**: 4 servers available ≥ threshold (3)

**✅ SUCCESS**: System is resilient to single server failures.

---

### Scenario 3: Failure (Below Threshold)

**Expectation**: Application fails to start with only 2 servers.

```bash
# Kill 3 more servers (total 4 killed, 1 remaining)
# Actually, let's kill 3 so we have exactly 2 left
kubectl delete pod hyena-share-server-1 hyena-share-server-2 hyena-share-server-3

# Wait a moment for deletions
sleep 5

# Restart demo app
kubectl delete pod -l app.kubernetes.io/component=demo-app

# Check pod status after ~10-20 seconds
kubectl get pod -l app.kubernetes.io/component=demo-app
```

**Expected output**:
```
NAME                          READY   STATUS                  RESTARTS   AGE
hyena-demo-app-xxx            0/1     Init:Error              0          15s
```

**Check init container logs**:

```bash
kubectl logs -l app.kubernetes.io/component=demo-app -c sidecar-reconstructor
```

**Expected log output**:
```
FATAL: Failed to fetch shares: failed to collect enough shares: got 2, need 3 (errors: 3)
```

**❌ FAILURE (Expected)**: Only 2 shares available, below threshold of 3.

The pod cannot start because the init container fails.

---

### Scenario 4: Recovery

**Expectation**: System recovers when servers come back.

```bash
# Wait for deleted share servers to be recreated
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/component=share-server \
  --timeout=120s

# Restart demo app
kubectl delete pod -l app.kubernetes.io/component=demo-app

# Wait for it to become ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/component=demo-app \
  --timeout=120s

# Check status
curl $(minikube service hyena-demo-app --url)/status
```

**Expected result**: `"secret_loaded": true`

**✅ SUCCESS**: System recovered once enough servers became available.

---

## Automated Demo Script

Run the complete demo automatically:

```bash
make demo
```

This script will:
1. Verify all pods are ready
2. Show initial status (all working)
3. Kill one server and verify app still works
4. Kill more servers and verify app fails
5. Show summary of results

## Inspecting Components

### View Share Server Logs

```bash
# All share servers
kubectl logs -l app.kubernetes.io/component=share-server --tail=20

# Specific server
kubectl logs hyena-share-server-0

# Follow logs
kubectl logs -f hyena-share-server-0
```

### View Sidecar (Init Container) Logs

```bash
# Get the demo app pod name
POD=$(kubectl get pod -l app.kubernetes.io/component=demo-app -o jsonpath='{.items[0].metadata.name}')

# View sidecar logs
kubectl logs $POD -c sidecar-reconstructor
```

**Example successful output**:
```
Starting sidecar reconstructor...
Configuration loaded:
  Endpoints: [share-server-0...:9000, ...]
  Threshold: 3
Fetching 3 shares from 5 servers...
Successfully fetched share from server share-server-0 (42 bytes)
Successfully fetched share from server share-server-1 (42 bytes)
Successfully fetched share from server share-server-2 (42 bytes)
Successfully collected 3 shares
Secret reconstructed successfully (32 bytes)
Secret written successfully to /secrets/app-secret
Sidecar reconstructor completed successfully
```

### View Demo App Logs

```bash
kubectl logs -l app.kubernetes.io/component=demo-app -c demo-app --tail=20
```

### Inspect Secret Storage

```bash
# View the shares secret (base64 encoded)
kubectl get secret hyena-shares -o yaml

# Decode a share (WARNING: This exposes share data)
kubectl get secret hyena-shares -o jsonpath='{.data.share-0\.bin}' | base64 -d | xxd
```

**⚠️ WARNING**: Don't expose shares in production!

### Check Resource Usage

```bash
# Pod resource usage
kubectl top pods -l app.kubernetes.io/instance=hyena

# Node resource usage
kubectl top nodes
```

## Verifying Security Properties

### 1. Secret Never on Disk

```bash
# Exec into demo app pod
POD=$(kubectl get pod -l app.kubernetes.io/component=demo-app -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $POD -- sh

# Inside pod:
# Check secret is in tmpfs
mount | grep secrets
# Output: tmpfs on /secrets type tmpfs (rw,relatime,size=...)

# Verify it's in memory, not disk
df -h /secrets
# Output: Filesystem Size Used ... Mounted on
#         tmpfs      XMi   4.0K ... /secrets

# The secret file
ls -lh /secrets/
cat /secrets/app-secret
# Output: (secret content)

exit
```

### 2. Authentication Working

```bash
# Try to fetch a share without valid token (should fail)
POD=$(kubectl get pod -l app.kubernetes.io/component=share-server -o jsonpath='{.items[0].metadata.name}')

# Port forward to share server
kubectl port-forward $POD 9000:9000 &
PF_PID=$!

# Try without auth (should fail)
grpcurl -plaintext \
  -d '{"requester_identity": "default:demo-app"}' \
  localhost:9000 \
  shareservice.v1.ShareService/GetShare

# Should get: Unauthenticated error

# Kill port forward
kill $PF_PID
```

## Cleanup

```bash
# Uninstall Helm release
make undeploy

# Or manually:
helm uninstall hyena
kubectl delete secret hyena-shares

# Stop minikube (optional)
minikube stop
```

## Troubleshooting

### Demo App Stuck in Init:0/1

**Problem**: Init container hasn't started or is hanging.

**Debug**:
```bash
kubectl describe pod -l app.kubernetes.io/component=demo-app
kubectl logs -l app.kubernetes.io/component=demo-app -c sidecar-reconstructor
```

**Common causes**:
- Share servers not ready
- Network issues
- Configuration errors

### Share Servers Not Starting

**Problem**: Share server pods crash or restart.

**Debug**:
```bash
kubectl logs hyena-share-server-0
kubectl describe pod hyena-share-server-0
```

**Common causes**:
- Share files not mounted (check secret)
- Invalid configuration
- Port conflicts

### Cannot Access Demo App

**Problem**: `minikube service` command fails or times out.

**Debug**:
```bash
# Check service
kubectl get svc hyena-demo-app

# Check endpoints
kubectl get endpoints hyena-demo-app

# Try direct port-forward
kubectl port-forward svc/hyena-demo-app 8080:8080
# Then access http://localhost:8080
```

## Next Steps

After completing the demo:

1. Review the [Architecture](architecture.md) documentation
2. Read the [Threat Model](threat-model.md)
3. Examine the code in `pkg/` and `cmd/`
4. Experiment with different N and K values
5. Try implementing additional features (see IMPLEMENTATION_PLAN.md)

## Demo Success Criteria

- ✅ All pods start successfully with full share availability
- ✅ App continues working with N-1 servers (above threshold)
- ✅ App fails gracefully with < K servers (below threshold)
- ✅ Secret is stored only in tmpfs (memory)
- ✅ Authentication prevents unauthorized share access
- ✅ System recovers when servers return online

**Congratulations!** You've successfully demonstrated threshold cryptography for Kubernetes secret management.
