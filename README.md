# 🐆 Hyena-K8s

**Decentralized Runtime Secret Management in Kubernetes Using Threshold Cryptography**

> ⚠️ **PROOF OF CONCEPT** - This is a research prototype demonstrating the feasibility of threshold cryptography for Kubernetes secret management. It is **NOT** production-ready and should not be used for real workloads.

## Overview

Hyena-K8s demonstrates a novel approach to secret management in Kubernetes using Shamir's Secret Sharing (SSS). Instead of storing secrets in a central location, secrets are:

1. **Split** into N shares using threshold cryptography (K-of-N)
2. **Distributed** across multiple independent share servers
3. **Reconstructed** at runtime in pod init containers
4. **Injected** into application containers via tmpfs (memory-only) volumes

**Key Property**: The plaintext secret never exists on disk and is only reconstructed when needed with K or more shares.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                        │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │            Share Server StatefulSet (N=5)          │    │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌──┐│    │
│  │  │Share 0 │ │Share 1 │ │Share 2 │ │Share 3 │ │ 4││    │
│  │  └───┬────┘ └───┬────┘ └───┬────┘ └───┬────┘ └─┬┘│    │
│  └──────┼──────────┼──────────┼──────────┼────────┼─┘    │
│         │    gRPC  │          │          │        │       │
│         │   + JWT  │          │          │        │       │
│         └──────────┴──────────┴──────────┴────────┘       │
│                     │                                      │
│         ┌───────────▼─────────────────────────┐           │
│         │  Init Container (Sidecar)           │           │
│         │  • Fetches K=3 shares                │           │
│         │  • Reconstructs secret               │           │
│         │  • Writes to tmpfs                   │           │
│         └──────────────┬───────────────────────┘           │
│                        │                                   │
│         ┌──────────────▼───────────────────────┐           │
│         │  tmpfs Volume (Memory Only)          │           │
│         │  /secrets/app-secret                 │           │
│         └──────────────┬───────────────────────┘           │
│                        │                                   │
│         ┌──────────────▼───────────────────────┐           │
│         │  Application Container               │           │
│         │  Reads secret from tmpfs              │           │
│         └──────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────┘
```

## Features

- ✅ **Shamir's Secret Sharing** - Threshold cryptography (K-of-N)
- ✅ **Distributed Storage** - No single point of failure
- ✅ **Runtime Reconstruction** - Secrets only exist in memory
- ✅ **JWT Authentication** - Kubernetes ServiceAccount tokens
- ✅ **gRPC Communication** - Efficient, type-safe protocol
- ✅ **Tmpfs Volumes** - Secrets never touch disk
- ✅ **Helm Deployment** - Easy installation

## Quick Start

### Prerequisites

- Go 1.25+
- Docker
- Minikube or kind
- kubectl
- Helm 3
- protoc (Protocol Buffers compiler)

### 1. Setup Minikube

```bash
make setup-minikube
eval $(minikube docker-env)
```

### 2. Build and Deploy

```bash
# Build all components
make build

# Build Docker images (in minikube's Docker daemon)
make docker

# Generate shares and deploy to Kubernetes
make deploy
```

### 3. Access the Demo Application

```bash
# Get the service URL
minikube service hyena-demo-app

# Or get the URL directly
minikube service hyena-demo-app --url
```

Visit the URL in your browser to see the demo application.

### 4. Run the Demo Scenario

```bash
make demo
```

This will demonstrate:
1. ✅ App works with all 5 share servers
2. ✅ App works with 4 servers (one killed)
3. ❌ App fails with 2 servers (below threshold)

## Components

### Share Server
- **Purpose**: Stores and serves a single SSS share
- **Authentication**: Validates JWT tokens from ServiceAccounts
- **Protocol**: gRPC with TLS support (optional)
- **Deployment**: StatefulSet with N replicas

### Sidecar Reconstructor (Init Container)
- **Purpose**: Fetches shares and reconstructs the secret
- **Behavior**: Runs once at pod startup
- **Failure**: Pod fails if < K shares available
- **Output**: Writes secret to tmpfs volume

### Demo Application
- **Purpose**: Demonstrates secret consumption
- **Endpoints**:
  - `GET /` - Web UI showing status
  - `GET /health` - Health check
  - `GET /status` - JSON status
- **Security**: Never logs or displays secret value

## Configuration

### Helm Values

```yaml
shareServer:
  replicaCount: 5      # N (total shares)
  threshold: 3         # K (minimum shares needed)
  devMode: true        # Skip JWT signature verification
  
sidecar:
  timeout: "10s"
  maxRetries: 3

demoApp:
  enabled: true
  serviceAccount:
    create: true
```

See [charts/hyena/values.yaml](charts/hyena/values.yaml) for full configuration.

## Development

### Build Binaries

```bash
make build
```

### Generate Protocol Buffers

```bash
make proto
```

### Run Tests

```bash
make test
```

### Clean Build Artifacts

```bash
make clean
```

## Documentation

- [Architecture](docs/architecture.md) - Detailed design and data flow
- [Threat Model](docs/threat-model.md) - Security analysis and limitations
- [Demo Walkthrough](docs/demo.md) - Step-by-step demo guide
- [Helm Chart](charts/hyena/README.md) - Chart documentation

## Project Structure

```
hyena-k8s/
├── cmd/
│   ├── share-server/        # Share server binary
│   ├── sidecar/             # Init container binary
│   └── split-secret/        # Secret splitting tool
├── pkg/
│   ├── shamir/              # Shamir's Secret Sharing
│   ├── auth/                # JWT validation
│   ├── transport/           # gRPC + TLS
│   ├── config/              # Configuration
│   └── client/              # Share fetcher
├── proto/                   # gRPC protocol definitions
├── charts/hyena/            # Helm chart
├── examples/demo-app/       # Demo application
├── scripts/                 # Helper scripts
└── docs/                    # Documentation
```

## How It Works

### 1. Secret Splitting (Offline)

```bash
./bin/split-secret \
  --secret "my-secret-data" \
  --parts 5 \
  --threshold 3 \
  --output ./shares/
```

Creates 5 shares where any 3 can reconstruct the secret.

### 2. Share Distribution (Installation)

Each share is stored in a separate share server pod:
- `share-server-0` → Share 0
- `share-server-1` → Share 1
- `share-server-2` → Share 2
- `share-server-3` → Share 3
- `share-server-4` → Share 4

### 3. Runtime Reconstruction (Pod Startup)

1. Init container starts
2. Reads ServiceAccount JWT token
3. Connects to share servers via gRPC
4. Fetches shares in parallel (with retries)
5. Reconstructs secret using `shamir.Combine()`
6. Writes to tmpfs at `/secrets/app-secret`
7. Exits successfully

### 4. Application Access

Application container reads secret from tmpfs volume.

## Security Considerations

### Trust Assumptions

- **Kubernetes API**: Trusted to validate ServiceAccount tokens
- **Network**: Share server communication protected by network policies
- **Memory**: Secrets in tmpfs are protected by Linux kernel isolation
- **Share Servers**: Each server is trusted to store its share correctly

### Threat Model

See [docs/threat-model.md](docs/threat-model.md) for detailed analysis.

**Key Threats NOT Addressed** (by design scope):
- Memory dumps or kernel exploits
- Compromised share server code
- Side-channel attacks
- Long-term key storage

## Limitations

This is a **proof of concept**, not a production system. Missing features:

- ❌ Secret rotation
- ❌ Audit logging
- ❌ Key versioning
- ❌ HA guarantees
- ❌ Performance optimization
- ❌ Formal security audit
- ❌ Production-grade error handling

## Comparison with Existing Solutions

| Feature | Hyena-K8s | Vault | Sealed Secrets |
|---------|-----------|-------|----------------|
| Centralized Store | ❌ | ✅ | ✅ |
| Threshold Crypto | ✅ | ❌ | ❌ |
| Runtime Reconstruction | ✅ | ❌ | ❌ |
| Disk-less Secrets | ✅ | ❌ | ❌ |
| Production Ready | ❌ | ✅ | ✅ |

## Contributing

This is a research project. Contributions should focus on:
- Bug fixes
- Documentation improvements
- Test coverage
- Demo enhancements

Please maintain the project scope - this is a PoC, not a production system.

## License

This project incorporates code from HashiCorp Vault's Shamir implementation:
- `pkg/shamir/shamir.go` - Copyright IBM Corp. 2016, 2025 (MPL-2.0)

All other code is provided as-is for educational and research purposes.

## Acknowledgments

- **Shamir's Secret Sharing**: Adi Shamir (1979)
- **HashiCorp Vault**: For the excellent SSS implementation
- **Kubernetes Community**: For the amazing platform

## Disclaimer

**This is NOT production-ready software.**

Do not use this for:
- Production workloads
- Sensitive data
- Compliance requirements
- Mission-critical systems

This project exists solely to demonstrate the feasibility of threshold cryptography for Kubernetes secret management.

## Contact

For questions or feedback about this research project, please open an issue.

---

**Remember**: This is a proof of concept. Use production-grade solutions like HashiCorp Vault for real workloads.
