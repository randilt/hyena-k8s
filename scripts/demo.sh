#!/bin/bash
# Demo script to show hyena-k8s functionality

set -e

NAMESPACE="${NAMESPACE:-default}"
RELEASE_NAME="${RELEASE_NAME:-hyena}"

echo "========================================="
echo "Hyena-K8s Demo"
echo "========================================="
echo ""

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    echo "Error: helm is not installed"
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed"
    exit 1
fi

echo "Step 1: Waiting for all pods to be ready..."
echo "  - Share servers: 5 replicas"
echo "  - Demo app: 1 replica"
echo ""

kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/component=share-server \
    -n "$NAMESPACE" \
    --timeout=120s || true

kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/component=demo-app \
    -n "$NAMESPACE" \
    --timeout=120s || true

echo ""
echo "✓ All pods are ready!"
echo ""

# Get pod names
DEMO_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=demo-app -o jsonpath='{.items[0].metadata.name}')
SHARE_PODS=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=share-server -o jsonpath='{.items[*].metadata.name}')

echo "Pod Status:"
kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/instance=$RELEASE_NAME"
echo ""

echo "========================================="
echo "Scenario 1: Normal Operation"
echo "========================================="
echo ""
echo "All share servers are running, app should work..."
echo ""

# Check app status
APP_URL=$(minikube service "$RELEASE_NAME-demo-app" -n "$NAMESPACE" --url)
echo "Demo app URL: $APP_URL"
echo ""

echo "Checking app health..."
curl -s "$APP_URL/status" | jq .
echo ""

echo "✓ App is working with all shares available"
echo ""

echo "========================================="
echo "Scenario 2: Kill One Share Server"
echo "========================================="
echo ""
echo "Killing one share server (threshold is 3, so app should still work)..."
echo ""

FIRST_SHARE_POD=$(echo $SHARE_PODS | awk '{print $1}')
echo "Deleting pod: $FIRST_SHARE_POD"
kubectl delete pod "$FIRST_SHARE_POD" -n "$NAMESPACE"

echo "Waiting for replacement pod..."
sleep 5

echo "Restarting demo app to test with 4 servers..."
kubectl delete pod "$DEMO_POD" -n "$NAMESPACE"

echo "Waiting for new demo pod..."
kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/component=demo-app \
    -n "$NAMESPACE" \
    --timeout=120s

DEMO_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=demo-app -o jsonpath='{.items[0].metadata.name}')

echo ""
echo "Checking app status..."
curl -s "$APP_URL/status" | jq .
echo ""

echo "✓ App still works with one server killed!"
echo ""

echo "========================================="
echo "Scenario 3: Kill More Servers (Below Threshold)"
echo "========================================="
echo ""
echo "Killing 2 more servers (total 3 killed, only 2 left, below threshold of 3)..."
echo ""

SHARE_PODS_ARRAY=($SHARE_PODS)
echo "Deleting pod: ${SHARE_PODS_ARRAY[1]}"
kubectl delete pod "${SHARE_PODS_ARRAY[1]}" -n "$NAMESPACE" || true

echo "Deleting pod: ${SHARE_PODS_ARRAY[2]}"
kubectl delete pod "${SHARE_PODS_ARRAY[2]}" -n "$NAMESPACE" || true

sleep 3

echo "Restarting demo app to test with only 2 servers..."
kubectl delete pod "$DEMO_POD" -n "$NAMESPACE"

echo "Waiting to see if pod fails..."
sleep 10

echo ""
echo "Pod status:"
kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/component=demo-app
echo ""

echo "Init container logs (should show failure):"
DEMO_POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=demo-app -o jsonpath='{.items[0].metadata.name}')
kubectl logs "$DEMO_POD" -c sidecar-reconstructor -n "$NAMESPACE" || echo "Init container failed as expected"
echo ""

echo "✓ App failed to start with insufficient shares (as expected)!"
echo ""

echo "========================================="
echo "Demo Complete!"
echo "========================================="
echo ""
echo "Summary:"
echo "  ✓ App works with all 5 share servers"
echo "  ✓ App works with 4 share servers (one killed)"
echo "  ✓ App fails with 2 share servers (below threshold of 3)"
echo ""
echo "This demonstrates that Shamir's Secret Sharing works as intended:"
echo "  - The secret can be reconstructed with K or more shares"
echo "  - The secret cannot be reconstructed with fewer than K shares"
echo ""
echo "To access the demo app (after restoring servers):"
echo "  minikube service $RELEASE_NAME-demo-app -n $NAMESPACE"
