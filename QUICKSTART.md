# 🚀 Quick Start Guide - Hyena-K8s

This guide will walk you through deploying and testing Hyena-K8s on a **fresh minikube cluster**.

⏱️ **Time Required**: ~10 minutes

## Prerequisites

Ensure you have these tools installed:

- **minikube** - `brew install minikube` or [install guide](https://minikube.sigs.k8s.io/docs/start/)
- **kubectl** - `brew install kubectl` or comes with minikube
- **helm** - `brew install helm` or [install guide](https://helm.sh/docs/intro/install/)
- **Go 1.25+** - For building binaries (optional if using pre-built images)
- **Docker** - Comes with minikube or [install guide](https://docs.docker.com/get-docker/)

## Step 1: Start Minikube

```bash
# Start minikube with sufficient resources
minikube start --memory=4096 --cpus=2

# Verify it's running
kubectl get nodes
```

Expected output:

```
NAME       STATUS   ROLES           AGE   VERSION
minikube   Ready    control-plane   1m    v1.28.3
```

## Step 2: Build Docker Images

We'll build the images directly in minikube's Docker daemon to avoid pushing to a registry.

```bash
# Configure your shell to use minikube's Docker daemon
eval $(minikube docker-env)

# Verify you're using minikube's Docker
docker ps | head -n 1  # Should show minikube containers

# Build all images (from the project root)
cd /path/to/hyena-k8s

# Build share-server
docker build -t hyena/share-server:latest \
  -f cmd/share-server/Dockerfile .

# Build sidecar
docker build -t hyena/sidecar:latest \
  -f cmd/sidecar/Dockerfile .

# Build secret-manager
docker build -t hyena/secret-manager:latest \
  -f cmd/secret-manager/Dockerfile .

# Build demo-app
docker build -t hyena/demo-app:latest \
  -f examples/demo-app/Dockerfile .

# Verify images
docker images | grep hyena
```

Expected output:

```
hyena/share-server    latest   ...   ...   ...
hyena/sidecar         latest   ...   ...   ...
hyena/secret-manager  latest   ...   ...   ...
hyena/demo-app        latest   ...   ...   ...
```

## Step 3: Deploy with Helm

```bash
# Install the Helm chart
helm install hyena ./charts/hyena

# Wait for share servers to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/component=share-server \
  --timeout=60s

# Wait for secret manager to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/component=secret-manager \
  --timeout=60s

# Check pod status
kubectl get pods
```

Expected output:

```
NAME                                  READY   STATUS     RESTARTS   AGE
hyena-demo-app-xxxxx-xxxxx           0/1     Init:0/1   0          30s  ← Will crash until secrets stored
hyena-secret-manager-xxxxx-xxxxx     1/1     Running    0          30s
hyena-share-server-0                 1/1     Running    0          30s
hyena-share-server-1                 1/1     Running    0          30s
hyena-share-server-2                 1/1     Running    0          30s
hyena-share-server-3                 1/1     Running    0          30s
hyena-share-server-4                 1/1     Running    0          30s
```

**Note**: The demo-app pod will be in `Init:0/1` or `CrashLoopBackOff` status because no secrets have been stored yet. This is expected!

## Step 4: Store Secrets

Get the secret manager URL and store some secrets:

```bash
# Get the secret manager URL
SECRET_MANAGER_URL=$(minikube service hyena-secret-manager --url | head -1)
echo "Secret Manager URL: $SECRET_MANAGER_URL"

# Store first secret: db-password
curl -X POST "$SECRET_MANAGER_URL/store" \
  -d "name=db-password&data=my-super-secret-database-password-12345"

# Store second secret: api-key
curl -X POST "$SECRET_MANAGER_URL/store" \
  -d "name=api-key&data=my-api-key-xyz123-very-secret"

# Store third secret: jwt-token
curl -X POST "$SECRET_MANAGER_URL/store" \
  -d "name=jwt-token&data=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ"
```

Expected output for each:

```json
{
  "success": true,
  "message": "Secret 'db-password' stored successfully",
  "servers": 5
}
```

This means:

- ✅ Secret was split into 5 shares using Shamir's Secret Sharing
- ✅ All 5 shares were distributed to the share servers
- ✅ Shares are stored in-memory only (never written to disk)

## Step 5: Restart Demo App

Now that secrets are stored, restart the demo app so it can fetch them:

```bash
# Delete the demo app pod (it will be recreated automatically)
kubectl delete pod -l app.kubernetes.io/component=demo-app

# Wait for it to become ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/component=demo-app \
  --timeout=60s

# Check the sidecar logs to see secret reconstruction
kubectl logs -l app.kubernetes.io/component=demo-app \
  -c sidecar-reconstructor
```

Expected output:

```
2026/01/14 10:33:50 Starting sidecar reconstructor...
2026/01/14 10:33:50   Secrets to fetch: 3
2026/01/14 10:33:50     [1] db-password -> /secrets/db-password
2026/01/14 10:33:50     [2] api-key -> /secrets/api-key
2026/01/14 10:33:50     [3] jwt-token -> /secrets/jwt-token
...
2026/01/14 10:33:50 ✓ Secret 'db-password' loaded successfully (39 bytes)
2026/01/14 10:33:50 ✓ Secret 'api-key' loaded successfully (29 bytes)
2026/01/14 10:33:51 ✓ Secret 'jwt-token' loaded successfully (111 bytes)
2026/01/14 10:33:51 === All secrets processed successfully ===
```

## Step 6: Access the Demo App

```bash
# Get the demo app URL
DEMO_APP_URL=$(minikube service hyena-demo-app --url)
echo "Demo App URL: $DEMO_APP_URL"

# Open in browser
minikube service hyena-demo-app

# Or check status via curl
curl "$DEMO_APP_URL/status" | python3 -m json.tool
```

Expected output:

```json
{
  "secrets": [
    {
      "name": "api-key",
      "length": 29,
      "loaded": true,
      "path": "/secrets/api-key",
      "sha256": "8d3ac5f65e8603d1d7ad891a702f5284593d0d86d4665a357538d96ec34ae5d8",
      "first_bytes": "my-api-key-xyz123-ve..."
    },
    {
      "name": "db-password",
      "length": 39,
      "loaded": true,
      "path": "/secrets/db-password",
      "sha256": "80b5988a790f9a71ab5bae92b931a9b9e1db82f54357dd7470e7e7fb818d50c3",
      "first_bytes": "my-super-secret-data..."
    },
    {
      "name": "jwt-token",
      "length": 111,
      "loaded": true,
      "path": "/secrets/jwt-token",
      "sha256": "12683707dcb79426c9e1d0cafc6d37c2fedb075543ed1f1c9140e7f6c68e7f12",
      "first_bytes": "eyJhbGciOiJIUzI1NiIs..."
    }
  ],
  "total_secrets": 3,
  "loaded_secrets": 3,
  "message": "All 3 secret(s) loaded successfully"
}
```

## Step 7: Verify Secret Reconstruction

The demo app provides SHA256 hashes to cryptographically prove the secrets were correctly reconstructed:

```bash
# Build the hash verification tool
go build -o /tmp/hash-secret ./cmd/hash-secret

# Compute hash of original db-password
/tmp/hash-secret "my-super-secret-database-password-12345"

# Get hash of reconstructed secret
curl "$DEMO_APP_URL/verify/db-password" | python3 -m json.tool
```

**Compare the SHA256 hashes** - they should match exactly!

Original:

```
SHA256: 80b5988a790f9a71ab5bae92b931a9b9e1db82f54357dd7470e7e7fb818d50c3
```

Reconstructed:

```json
{
  "sha256": "80b5988a790f9a71ab5bae92b931a9b9e1db82f54357dd7470e7e7fb818d50c3",
  "verification_note": "Compare this SHA256 hash with the hash of your original secret to verify correct reconstruction"
}
```

✅ **Hashes match = Perfect reconstruction from distributed shares!**

## Step 8: Verify Secrets Are NOT on Disk

Let's prove secrets are only in memory (tmpfs), never on disk:

```bash
# Get the demo app pod name
DEMO_POD=$(kubectl get pod -l app.kubernetes.io/component=demo-app -o jsonpath='{.items[0].metadata.name}')

# Check the mount type for /secrets
kubectl exec $DEMO_POD -- mount | grep /secrets
```

Expected output:

```
tmpfs on /secrets type tmpfs (rw,nosuid,nodev,noexec,relatime,size=1024k)
```

**`tmpfs`** = RAM-backed filesystem. Secrets are never written to persistent storage!

```bash
# Try to find secrets on disk (should return nothing)
kubectl exec $DEMO_POD -- find / -name "*password*" -o -name "*api-key*" 2>/dev/null | grep -v "/secrets"
```

No results = secrets don't exist anywhere on disk! ✅

## Step 9: Test Threshold Property

The system requires K=3 shares to reconstruct. Let's verify this works even with servers down:

```bash
# Kill 2 share servers (we should still have 3/5 = OK)
kubectl delete pod hyena-share-server-3 hyena-share-server-4

# Restart demo app to test fetching with only 3 servers
kubectl delete pod -l app.kubernetes.io/component=demo-app
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=demo-app --timeout=60s

# Check status - should still work!
curl "$DEMO_APP_URL/status" | python3 -m json.tool
```

✅ Demo app successfully reconstructed secrets with only 3 servers!

```bash
# Now kill one more server (down to 2/5 = FAIL)
kubectl delete pod hyena-share-server-2

# Restart demo app - it should FAIL
kubectl delete pod -l app.kubernetes.io/component=demo-app

# Check pod status
kubectl get pod -l app.kubernetes.io/component=demo-app
```

Expected: Pod stuck in `Init:0/1` or `CrashLoopBackOff` because only 2 shares available (below threshold K=3).

```bash
# Check logs to see the failure
kubectl logs -l app.kubernetes.io/component=demo-app -c sidecar-reconstructor
```

Expected output:

```
ERROR: Failed to fetch enough shares for 'db-password': only got 2 shares, need 3
```

This proves the **threshold property**: Need K=3 shares minimum!

## Step 10: Cleanup

When you're done testing:

```bash
# Uninstall the Helm release
helm uninstall hyena

# Stop minikube (optional)
minikube stop

# Delete minikube cluster (optional)
minikube delete
```

## 🎉 Success!

You've successfully:

1. ✅ Deployed the Hyena-K8s infrastructure
2. ✅ Stored secrets dynamically via HTTP API
3. ✅ Verified secrets were split into 5 shares and distributed
4. ✅ Reconstructed secrets from distributed shares at runtime
5. ✅ Verified secrets only exist in RAM (tmpfs), never on disk
6. ✅ Proved correct reconstruction using SHA256 hashes
7. ✅ Tested the threshold property (K-of-N)

## Next Steps

- Read [Architecture Documentation](docs/architecture.md) for technical details
- Review [Threat Model](docs/threat-model.md) for security analysis
- Check [Demo Guide](docs/demo.md) for more advanced scenarios
- Explore the [Helm Chart](charts/hyena/README.md) for configuration options

## Troubleshooting

### Demo app stuck in Init:0/1

**Cause**: No secrets stored yet, or fewer than K shares available.

**Fix**:

```bash
# Check if secrets were stored
curl "$SECRET_MANAGER_URL/store" -d "name=test&data=testvalue"

# Check share server status
kubectl get pods -l app.kubernetes.io/component=share-server

# Restart demo app
kubectl delete pod -l app.kubernetes.io/component=demo-app
```

### "Connection refused" to secret-manager

**Cause**: Service not ready or minikube tunnel needed.

**Fix**:

```bash
# Check service status
kubectl get svc hyena-secret-manager

# Get URL again
minikube service hyena-secret-manager --url
```

### Docker images not found

**Cause**: Images built in wrong Docker context.

**Fix**:

```bash
# Make sure you're using minikube's Docker
eval $(minikube docker-env)

# Rebuild images
docker build -t hyena/share-server:latest -f cmd/share-server/Dockerfile .
# ... repeat for other images

# Verify
docker images | grep hyena
```

## Questions?

Open an issue on GitHub or check the main [README.md](README.md) for more information.

---

**Remember**: This is a proof-of-concept for educational purposes. Do not use in production!
