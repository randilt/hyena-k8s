.PHONY: help proto build build-all docker docker-all test clean deploy demo

# Default target
help:
	@echo "Hyena-K8s Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  proto        - Generate protobuf code"
	@echo "  build        - Build all binaries"
	@echo "  docker       - Build all Docker images"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  deploy       - Deploy to current kubectl context"
	@echo "  demo         - Run demo scenario"
	@echo "  setup-minikube - Setup minikube for development"

# Proto generation
proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/shareservice/v1/share.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/secretmanager/v1/secret_manager.proto
	@echo "✓ Proto generation complete"

# Build all binaries
build: build-share-server build-sidecar build-split-secret build-demo-app build-secret-manager

build-share-server:
	@echo "Building share-server..."
	@mkdir -p bin
	go build -o bin/share-server ./cmd/share-server

build-sidecar:
	@echo "Building sidecar..."
	@mkdir -p bin
	go build -o bin/sidecar ./cmd/sidecar

build-split-secret:
	@echo "Building split-secret..."
	@mkdir -p bin
	go build -o bin/split-secret ./cmd/split-secret

build-demo-app:
	@echo "Building demo-app..."
	@mkdir -p bin
	go build -o bin/demo-app ./examples/demo-app

build-secret-manager:
	@echo "Building secret-manager..."
	@mkdir -p bin
	go build -o bin/secret-manager ./cmd/secret-manager

# Docker images
docker: docker-share-server docker-sidecar docker-demo-app docker-secret-manager

docker-share-server:
	@echo "Building share-server Docker image..."
	docker build -t hyena/share-server:latest -f cmd/share-server/Dockerfile .

docker-sidecar:
	@echo "Building sidecar Docker image..."
	docker build -t hyena/sidecar:latest -f cmd/sidecar/Dockerfile .

docker-demo-app:
	@echo "Building demo-app Docker image..."
	docker build -t hyena/demo-app:latest -f examples/demo-app/Dockerfile .

docker-secret-manager:
	@echo "Building secret-manager Docker image..."
	docker build -t hyena/secret-manager:latest -f cmd/secret-manager/Dockerfile .

# Test
test:
	@echo "Running tests..."
	go test -v -race ./...

# Clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf shares/
	@echo "✓ Clean complete"

# Setup minikube
setup-minikube:
	@./scripts/setup-minikube.sh

# Generate shares
generate-shares:
	@./scripts/generate-shares.sh \
		--secret "ThisIsADemoSecretForTesting123!" \
		--parts 5 \
		--threshold 3

# Deploy to Kubernetes
deploy: docker generate-shares
	@echo "Deploying to Kubernetes..."
	@eval $$(minikube docker-env) && $(MAKE) docker
	@echo "Creating shares secret..."
	@kubectl create secret generic hyena-shares \
		--from-file=share-0.bin=shares/share-0.b64 \
		--from-file=share-1.bin=shares/share-1.b64 \
		--from-file=share-2.bin=shares/share-2.b64 \
		--from-file=share-3.bin=shares/share-3.b64 \
		--from-file=share-4.bin=shares/share-4.b64 \
		--dry-run=client -o yaml | kubectl apply -f -
	@echo "Installing Helm chart..."
	helm upgrade --install hyena ./charts/hyena --wait
	@echo "✓ Deployment complete"

# Undeploy
undeploy:
	@echo "Undeploying from Kubernetes..."
	helm uninstall hyena || true
	kubectl delete secret hyena-shares || true
	@echo "✓ Undeploy complete"

# Run demo
demo:
	@./scripts/demo.sh

# Show logs
logs-share-server:
	kubectl logs -l app.kubernetes.io/component=share-server --tail=50 -f

logs-demo-app:
	kubectl logs -l app.kubernetes.io/component=demo-app --tail=50 -f

logs-sidecar:
	kubectl logs -l app.kubernetes.io/component=demo-app -c sidecar-reconstructor

# Quick dev cycle
dev: build docker deploy
	@echo "✓ Development deployment complete"

# Status
status:
	@echo "Kubernetes Resources:"
	@kubectl get all -l app.kubernetes.io/instance=hyena
	@echo ""
	@echo "Secrets:"
	@kubectl get secrets -l app.kubernetes.io/instance=hyena
	@echo ""
	@echo "Demo App URL:"
	@minikube service hyena-demo-app --url || echo "Not available"
