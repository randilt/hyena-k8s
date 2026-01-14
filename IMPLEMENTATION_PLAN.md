# Hyena-K8s Implementation Plan

## Project Overview

**Decentralized Runtime Secret Management in Kubernetes Using Threshold Cryptography**

A proof-of-concept system that uses Shamir's Secret Sharing to distribute secret shares across multiple servers, with runtime reconstruction via init containers.

---

## Architecture Summary

```
┌─────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                      │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │              Share Server Deployment               │    │
│  │  ┌──────────┐  ┌──────────┐       ┌──────────┐   │    │
│  │  │ Server 1 │  │ Server 2 │  ...  │ Server N │   │    │
│  │  │ (Share1) │  │ (Share2) │       │ (ShareN) │   │    │
│  │  └────┬─────┘  └────┬─────┘       └────┬─────┘   │    │
│  └───────┼─────────────┼──────────────────┼─────────┘    │
│          │             │                  │               │
│          │   gRPC + mTLS + JWT Auth       │               │
│          │             │                  │               │
│  ┌───────▼─────────────▼──────────────────▼─────────┐    │
│  │         Application Pod with Init Container       │    │
│  │  ┌─────────────────────────────────────────────┐ │    │
│  │  │  Init Container (Sidecar Reconstructor)     │ │    │
│  │  │  1. Fetches K of N shares                   │ │    │
│  │  │  2. Reconstructs secret in memory           │ │    │
│  │  │  3. Writes to tmpfs volume                  │ │    │
│  │  └──────────────────┬──────────────────────────┘ │    │
│  │                     │                             │    │
│  │  ┌──────────────────▼──────────────────────────┐ │    │
│  │  │  tmpfs volume (medium: Memory)              │ │    │
│  │  │  /secrets/app-secret                        │ │    │
│  │  └──────────────────┬──────────────────────────┘ │    │
│  │                     │                             │    │
│  │  ┌──────────────────▼──────────────────────────┐ │    │
│  │  │  Application Container                      │ │    │
│  │  │  Reads secret from /secrets/app-secret      │ │    │
│  │  └─────────────────────────────────────────────┘ │    │
│  └───────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

---

## Directory Structure

```
hyena-k8s/
├── cmd/
│   ├── share-server/          # Share server main
│   │   └── main.go
│   ├── sidecar/               # Init container main
│   │   └── main.go
│   └── split-secret/          # CLI tool to split secrets
│       └── main.go
├── pkg/
│   ├── shamir/                # ✓ Already implemented
│   │   └── shamir.go
│   ├── auth/                  # JWT validation
│   │   ├── validator.go       # K8s ServiceAccount JWT validation
│   │   └── claims.go          # JWT claims parsing
│   ├── transport/             # gRPC transport layer
│   │   ├── tls.go             # TLS configuration
│   │   └── interceptors.go    # Auth interceptors
│   ├── config/                # Configuration management
│   │   ├── server.go          # Share server config
│   │   └── sidecar.go         # Sidecar config
│   └── client/                # Share fetching client
│       └── fetcher.go         # Parallel share fetching with retries
├── proto/
│   └── shareservice/
│       └── v1/
│           ├── share.proto    # gRPC service definition
│           └── share.pb.go    # Generated code
├── charts/
│   └── hyena/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── templates/
│       │   ├── _helpers.tpl
│       │   ├── share-server-statefulset.yaml
│       │   ├── share-server-service.yaml
│       │   ├── share-server-configmap.yaml
│       │   ├── share-server-secrets.yaml
│       │   ├── rbac.yaml
│       │   ├── demo-app-deployment.yaml
│       │   └── demo-app-serviceaccount.yaml
│       └── README.md
├── scripts/
│   ├── generate-shares.sh     # Helper to split secrets
│   ├── setup-minikube.sh      # Minikube setup
│   └── demo.sh                # Demo scenario runner
├── docs/
│   ├── architecture.md        # Detailed architecture
│   ├── threat-model.md        # Security analysis
│   └── demo.md                # Demo walkthrough
├── examples/
│   └── demo-app/              # Simple demo application
│       ├── Dockerfile
│       └── main.go
├── go.mod
├── go.sum
├── Makefile                   # Build automation
└── README.md
```

---

## Implementation Phases

### Phase 1: Core gRPC Protocol (proto + code generation)

**Files to create:**
- `proto/shareservice/v1/share.proto`

**Protocol definition:**
```protobuf
syntax = "proto3";
package shareservice.v1;

service ShareService {
  rpc GetShare(GetShareRequest) returns (GetShareResponse);
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

message GetShareRequest {
  string requester_identity = 1;  // ServiceAccount name/namespace
}

message GetShareResponse {
  bytes share = 1;
  string server_id = 2;  // Which share server this is
}

message HealthCheckRequest {}

message HealthCheckResponse {
  bool healthy = 1;
  string server_id = 2;
}
```

**Action:** Generate Go code with `protoc`

---

### Phase 2: Authentication Layer

**Files to create:**
- `pkg/auth/validator.go`
- `pkg/auth/claims.go`

**Key components:**

1. **JWT Validator** (`pkg/auth/validator.go`)
   - Validate Kubernetes ServiceAccount JWTs
   - Extract subject (system:serviceaccount:namespace:name)
   - Verify signature using K8s public keys (or accept valid tokens in dev mode)

2. **Claims Parser** (`pkg/auth/claims.go`)
   - Parse namespace and ServiceAccount name
   - Struct for holding caller identity

**Authorization logic:**
- Share server has a configurable list of allowed ServiceAccounts
- Format: `namespace:serviceaccount-name`
- Returns share only if caller is in the allowlist

---

### Phase 3: Transport Layer

**Files to create:**
- `pkg/transport/tls.go`
- `pkg/transport/interceptors.go`

**Key components:**

1. **TLS Configuration** (`pkg/transport/tls.go`)
   - Server: Load cert/key from files or generate self-signed for dev
   - Client: Load CA cert, support insecure mode for local testing

2. **Auth Interceptor** (`pkg/transport/interceptors.go`)
   - Extract JWT from gRPC metadata (`authorization: Bearer <token>`)
   - Validate using `pkg/auth`
   - Inject identity into context
   - Reject requests with invalid/missing tokens

---

### Phase 4: Share Server

**Files to create:**
- `pkg/config/server.go`
- `cmd/share-server/main.go`

**Configuration** (`pkg/config/server.go`):
```go
type ShareServerConfig struct {
    ServerID         string   // Unique identifier (e.g., "share-0")
    Port             int      // gRPC port (default: 9000)
    ShareData        []byte   // The actual share to serve
    AllowedCallers   []string // List of "namespace:sa-name"
    TLSCertPath      string
    TLSKeyPath       string
    TLSEnabled       bool
}
```

**Server implementation** (`cmd/share-server/main.go`):
- Load config from env vars and file
- Initialize gRPC server with TLS + auth interceptor
- Implement `GetShare` RPC:
  - Check if caller is in allowlist
  - Return share if authorized
  - Log all access attempts
- Implement `HealthCheck` RPC
- Graceful shutdown on SIGTERM

**Share storage:**
- Share loaded from a Kubernetes Secret mounted as a file
- Read once at startup into memory
- Never log or expose the share value

---

### Phase 5: Share Fetcher Client

**Files to create:**
- `pkg/client/fetcher.go`

**Key functionality:**

```go
type ShareFetcher struct {
    ServerEndpoints []string  // List of "host:port"
    Threshold       int       // K (minimum shares needed)
    Timeout         time.Duration
    MaxRetries      int
    TLSConfig       *tls.Config
    JWTToken        string    // ServiceAccount token
}

func (f *ShareFetcher) FetchShares(ctx context.Context) ([][]byte, error)
```

**Behavior:**
- Fetch shares **in parallel** from all N servers
- Use context with timeout per request
- Retry transient failures (connection refused, timeout)
- Stop fetching once K shares are collected (optimization)
- Return error if fewer than K shares collected after retries
- Log which servers succeeded/failed

---

### Phase 6: Sidecar Reconstructor (Init Container)

**Files to create:**
- `pkg/config/sidecar.go`
- `cmd/sidecar/main.go`

**Configuration** (`pkg/config/sidecar.go`):
```go
type SidecarConfig struct {
    ServerEndpoints []string  // From env: SHARE_SERVER_ENDPOINTS
    Threshold       int       // From env: THRESHOLD
    OutputPath      string    // From env: SECRET_OUTPUT_PATH (default: /secrets/secret)
    Timeout         time.Duration
    MaxRetries      int
    TLSCAPath       string
    TLSInsecure     bool
}
```

**Init container logic** (`cmd/sidecar/main.go`):
1. Load configuration from environment variables
2. Read ServiceAccount JWT from `/var/run/secrets/kubernetes.io/serviceaccount/token`
3. Create `ShareFetcher` client
4. Fetch K shares from share servers
5. Reconstruct secret using `shamir.Combine(shares)`
6. Write reconstructed secret to `OutputPath` (tmpfs volume)
7. Set restrictive file permissions (0400)
8. Exit with code 0 on success, non-zero on failure

**Error handling:**
- Fail fast if K shares cannot be obtained
- Log errors clearly for debugging
- Return distinct exit codes for different failure modes

---

### Phase 7: Secret Splitting Tool

**Files to create:**
- `cmd/split-secret/main.go`

**Functionality:**
- CLI tool to split a secret into N shares with threshold K
- Usage: `split-secret --secret="my-secret" --parts=5 --threshold=3 --output=./shares/`
- Outputs N files: `share-0.bin`, `share-1.bin`, ..., `share-N-1.bin`
- Used during Helm chart installation to generate shares

---

### Phase 8: Helm Chart

**Files to create:**
- `charts/hyena/Chart.yaml`
- `charts/hyena/values.yaml`
- `charts/hyena/templates/*.yaml`

**values.yaml structure:**
```yaml
shareServer:
  replicaCount: 5          # N
  threshold: 3             # K
  image:
    repository: hyena/share-server
    tag: latest
  port: 9000
  tls:
    enabled: false         # For local dev
  # Shares will be injected as secrets during installation

sidecar:
  image:
    repository: hyena/sidecar
    tag: latest
  timeout: 10s
  maxRetries: 3

demoApp:
  enabled: true
  image:
    repository: hyena/demo-app
    tag: latest
  secretPath: /secrets/app-secret
```

**Templates:**

1. **share-server-statefulset.yaml**
   - StatefulSet with N replicas
   - Each replica has a unique identity (share-server-0, share-server-1, ...)
   - Each mounts its own share from a Secret (share-server-0-secret, etc.)
   - Environment variables: SERVER_ID, PORT, ALLOWED_CALLERS

2. **share-server-service.yaml**
   - Headless service for share servers
   - Individual pod DNS: `share-server-0.share-server.default.svc.cluster.local`

3. **share-server-secrets.yaml**
   - One Secret per share server replica
   - Secret name: `share-server-{i}-secret`
   - Data: base64-encoded share

4. **rbac.yaml**
   - ServiceAccount for demo app
   - RoleBinding if needed (likely not for this PoC)

5. **demo-app-deployment.yaml**
   - Deployment with 1 replica
   - Init container (sidecar) with share fetcher
   - Shared tmpfs volume between init and app container
   - Environment variables for sidecar:
     - `SHARE_SERVER_ENDPOINTS`: comma-separated list of server addresses
     - `THRESHOLD`: K value
     - `SECRET_OUTPUT_PATH`: /secrets/app-secret
   - Application container mounts the same tmpfs volume

6. **demo-app-serviceaccount.yaml**
   - ServiceAccount for demo app pods
   - Used for JWT authentication with share servers

---

### Phase 9: Demo Application

**Files to create:**
- `examples/demo-app/main.go`
- `examples/demo-app/Dockerfile`

**Application behavior:**
- Read secret from `/secrets/app-secret`
- **Do NOT print the secret value**
- Print confirmation: "Secret loaded successfully: [length=X bytes]"
- Start a simple HTTP server on port 8080
- Endpoint `/health` returns 200 if secret is loaded
- Endpoint `/status` returns JSON: `{"secret_loaded": true, "secret_length": X}`

**Dockerfile:**
- Multi-stage build
- Minimal base image (distroless or alpine)

---

### Phase 10: Scripts and Automation

**Files to create:**

1. **scripts/generate-shares.sh**
   - Helper to split a secret and output base64-encoded shares
   - Used during Helm install: `./scripts/generate-shares.sh "my-secret" 5 3`

2. **scripts/setup-minikube.sh**
   - Start minikube with appropriate resources
   - Enable necessary addons

3. **scripts/demo.sh**
   - Automated demo scenario:
     1. Install Helm chart
     2. Wait for pods to be ready
     3. Check app status
     4. Kill one share server pod
     5. Restart app pod, verify success
     6. Kill more share servers (below threshold)
     7. Restart app pod, verify failure

4. **Makefile**
   - `make proto`: Generate protobuf code
   - `make build`: Build all binaries
   - `make docker`: Build Docker images
   - `make deploy`: Deploy to minikube
   - `make demo`: Run demo scenario
   - `make clean`: Clean up

---

### Phase 11: Documentation

**Files to create:**

1. **README.md** (root)
   - Project overview
   - Quick start guide
   - Architecture summary
   - Disclaimer (NOT production-ready)

2. **docs/architecture.md**
   - Detailed component descriptions
   - Data flow diagrams
   - Sequence diagrams for share fetching and reconstruction

3. **docs/threat-model.md**
   - Trust assumptions
   - Attack vectors and mitigations
   - Limitations and scope boundaries

4. **docs/demo.md**
   - Step-by-step demo walkthrough
   - Expected output at each step
   - Troubleshooting guide

5. **charts/hyena/README.md**
   - Helm chart documentation
   - Configuration options
   - Installation instructions

---

## Implementation Order

### Iteration 1: Core Functionality (No Auth, No TLS)
1. Implement proto definitions and generate code
2. Implement share server (no auth)
3. Implement share fetcher client (no auth)
4. Implement sidecar reconstructor
5. Implement split-secret tool
6. Test locally with hardcoded shares

### Iteration 2: Kubernetes Integration
7. Create Helm chart (basic)
8. Create demo app
9. Build Docker images
10. Deploy to minikube and test

### Iteration 3: Security & Polish
11. Add JWT authentication
12. Add TLS support (optional for PoC)
13. Add scripts and automation
14. Complete documentation

---

## Testing Strategy

### Unit Tests
- `pkg/auth`: JWT validation logic
- `pkg/client`: Share fetching with mocked servers

### Integration Tests
- Local test with 5 share servers (using goroutines)
- Test reconstruction with K, K+1, K-1 shares
- Test auth rejection

### E2E Tests (Manual)
- Deploy to minikube
- Run demo scenario
- Verify failure cases

---

## Configuration Management

### Share Server Environment Variables
```
SERVER_ID=share-0
PORT=9000
SHARE_FILE=/secrets/share.bin
ALLOWED_CALLERS=default:demo-app
TLS_ENABLED=false
TLS_CERT_PATH=/certs/tls.crt
TLS_KEY_PATH=/certs/tls.key
```

### Sidecar Environment Variables
```
SHARE_SERVER_ENDPOINTS=share-server-0.share-server:9000,share-server-1.share-server:9000,...
THRESHOLD=3
SECRET_OUTPUT_PATH=/secrets/app-secret
TIMEOUT=10s
MAX_RETRIES=3
TLS_CA_PATH=/certs/ca.crt
TLS_INSECURE=true
```

---

## Key Design Decisions

### 1. StatefulSet vs Deployment for Share Servers
**Choice:** StatefulSet
**Reason:** Each server needs a stable identity and unique share

### 2. Init Container vs Sidecar
**Choice:** Init Container
**Reason:** Secret only needs to be fetched once at startup

### 3. gRPC vs HTTP/REST
**Choice:** gRPC
**Reason:** Better performance, typed contracts, built-in streaming (future)

### 4. Share Storage
**Choice:** Kubernetes Secrets mounted as files
**Reason:** Native K8s primitive, easy to manage with Helm

### 5. Auth Mechanism
**Choice:** ServiceAccount JWT
**Reason:** Native K8s auth, no additional infrastructure

### 6. TLS
**Choice:** Optional (disabled by default for PoC)
**Reason:** Complexity vs. benefit trade-off for local demo

---

## Validation Criteria

The implementation is complete when:

1. ✅ Secret can be split into N shares with threshold K
2. ✅ Share servers serve shares only to authorized callers
3. ✅ Sidecar reconstructs secret with ≥K shares
4. ✅ Sidecar fails with <K shares
5. ✅ Reconstructed secret never touches disk (tmpfs only)
6. ✅ Demo app starts successfully with working shares
7. ✅ Demo app fails to start when <K servers are available
8. ✅ Killing one server doesn't prevent app from starting
9. ✅ All code is well-commented and readable
10. ✅ Documentation is complete and accurate

---

## Non-Goals (Explicitly Out of Scope)

- ❌ Secret rotation
- ❌ Verifiable secret sharing
- ❌ Custom Kubernetes controllers
- ❌ Admission webhooks
- ❌ Multi-cluster deployments
- ❌ Production-grade HA
- ❌ Performance optimization beyond basic parallel fetching
- ❌ Secret versioning
- ❌ Audit logging (beyond basic access logs)
- ❌ Integration with external KMS

---

## Estimated Complexity

- **Proto & Code Generation:** 1 hour
- **Auth Layer:** 2-3 hours
- **Share Server:** 3-4 hours
- **Client & Sidecar:** 3-4 hours
- **Helm Chart:** 4-5 hours
- **Demo App & Scripts:** 2-3 hours
- **Documentation:** 3-4 hours
- **Testing & Debugging:** 4-6 hours

**Total:** ~25-35 hours for a complete, working prototype

---

## Risk Mitigation

### Risk: JWT validation complexity
**Mitigation:** Start with simple token validation, add K8s API verification if needed

### Risk: Helm chart complexity with multiple secrets
**Mitigation:** Use templating and clear naming conventions (share-server-{i}-secret)

### Risk: Parallel gRPC client complexity
**Mitigation:** Use errgroup for simple parallel execution with context cancellation

### Risk: Demo doesn't clearly show failure scenarios
**Mitigation:** Add detailed logging and clear status outputs

---

## Success Metrics

A successful implementation will demonstrate:

1. **Functional correctness:** All validation criteria met
2. **Code quality:** Clean, readable, well-structured
3. **Documentation quality:** Clear, complete, honest about limitations
4. **Demo quality:** Easy to run, clearly shows intended behavior
5. **Scope discipline:** No out-of-scope features added

---

This plan provides a complete, achievable path to a working prototype that meets all requirements while maintaining strict scope discipline.
