#!/bin/bash
# Setup minikube for hyena-k8s demo

set -e

echo "Setting up minikube for Hyena-K8s..."

# Check if minikube is installed
if ! command -v minikube &> /dev/null; then
    echo "Error: minikube is not installed"
    echo "Please install minikube: https://minikube.sigs.k8s.io/docs/start/"
    exit 1
fi

# Check if minikube is running
if ! minikube status &> /dev/null; then
    echo "Starting minikube..."
    minikube start \
        --cpus=4 \
        --memory=4096 \
        --driver=docker \
        --kubernetes-version=v1.28.0
else
    echo "Minikube is already running"
fi

# Enable addons
echo "Enabling minikube addons..."
minikube addons enable metrics-server
minikube addons enable ingress

# Configure docker to use minikube's docker daemon
echo ""
echo "✓ Minikube setup complete!"
echo ""
echo "To use minikube's docker daemon, run:"
echo "  eval \$(minikube docker-env)"
echo ""
echo "To access services:"
echo "  minikube service list"
echo "  minikube service <service-name>"
