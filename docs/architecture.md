# Architecture Documentation

## System Overview

Hyena-K8s implements a decentralized secret management system using Shamir's Secret Sharing (SSS) to distribute trust across multiple share servers.

## Core Components

### 1. Share Server

**Responsibility**: Store and serve a single SSS share with authentication.

**Key Design Decisions**:

- **Stateless**: Each server only knows its own share
- **Authentication**: JWT validation using Kubernetes ServiceAccount tokens
- **Protocol**: gRPC for efficiency and type safety
- **Deployment**: StatefulSet for stable network identities

**Data Flow**:

```
Client Request → JWT Validation → Authorization Check → Serve Share
```

**Configuration**:

- `SERVER_ID`: Unique identifier
- `SHARE_FILE`: Path to share data
- `ALLOWED_CALLERS`: List of authorized identities
- `DEV_MODE`: Skip JWT signature verification (dev only)

### 2. Sidecar Reconstructor (Init Container)

**Responsibility**: Fetch K shares and reconstruct the secret at pod startup.

**Key Design Decisions**:

- **Init Container**: Runs once before app starts
- **Parallel Fetching**: Concurrent requests to all servers
- **Retry Logic**: Exponential backoff for transient failures
- **Fail Fast**: Exit with error if < K shares available

**Algorithm**:

```
1. Load configuration from environment
2. Read ServiceAccount JWT token
3. For each server endpoint (in parallel):
   a. Connect via gRPC
   b. Send GetShare request with JWT
   c. Handle retries on failure
4. If collected shares < threshold:
   Exit with error code
5. Else:
   Reconstruct using shamir.Combine()
   Write to tmpfs with mode 0400
   Exit successfully
```

**Exit Codes**:

- `0`: Success
- `1`: Configuration error
- `2`: Token read error
- `3`: Failed to fetch enough shares
- `4`: Reconstruction error
- `5`: Write error

### 3. Demo Application

**Responsibility**: Demonstrate secret consumption without exposing the value.

**Key Features**:

- Reads secret from tmpfs volume
- Provides HTTP endpoints for status
- **Never** logs or displays secret content
- Only reports secret length and availability

## Data Flow

### Secret Splitting (Offline)

```
Original Secret → shamir.Split(N=5, K=3) → 5 Shares → Store in K8s Secrets
```

### Secret Reconstruction (Runtime)

```
Pod Starts
  ↓
Init Container Runs
  ↓
Fetch Shares (parallel)
  ├─→ Server 0 → Share 0 ✓
  ├─→ Server 1 → Share 1 ✓
  ├─→ Server 2 → Share 2 ✓
  ├─→ Server 3 → [timeout] ✗
  └─→ Server 4 → Share 4 ✓
  ↓
Collected 4 shares ≥ threshold (3) ✓
  ↓
shamir.Combine([S0, S1, S2, S4])
  ↓
Write to tmpfs: /secrets/app-secret
  ↓
Init Container Exits (0)
  ↓
App Container Starts
  ↓
App Reads /secrets/app-secret
```

## Security Architecture

### Authentication Flow

```
Application Pod
  └─→ ServiceAccount Token → gRPC Metadata
        └─→ Share Server
              ├─→ Extract JWT from metadata
              ├─→ Validate JWT format
              ├─→ Parse subject claim
              ├─→ Check authorization list
              └─→ Serve share if authorized
```

### Authorization Model

- **Identity**: `namespace:serviceaccount-name`
- **Allow List**: Configured per share server
- **Enforcement**: gRPC interceptor

### Memory-Only Secrets

```
Reconstruction happens in:
  - Init container memory

Written to:
  - tmpfs volume (RAM-backed filesystem)

Accessible by:
  - Application container (same pod)

Never written to:
  - Persistent volumes
  - Container layer
  - Node filesystem
```

## Network Architecture

### Share Server Service (Headless)

```
hyena-share-server (ClusterIP: None)
  ├─→ hyena-share-server-0.hyena-share-server.default.svc.cluster.local:9000
  ├─→ hyena-share-server-1.hyena-share-server.default.svc.cluster.local:9000
  ├─→ hyena-share-server-2.hyena-share-server.default.svc.cluster.local:9000
  ├─→ hyena-share-server-3.hyena-share-server.default.svc.cluster.local:9000
  └─→ hyena-share-server-4.hyena-share-server.default.svc.cluster.local:9000
```

### Why Headless Service?

- Each pod needs a stable DNS name
- Init container connects to specific pods
- Enables targeting individual share servers

## Failure Modes

### Scenario 1: One Server Down

**Configuration**: N=5, K=3

**State**: 4 servers available

**Result**: ✅ Success - 4 shares fetched, secret reconstructed

### Scenario 2: Below Threshold

**Configuration**: N=5, K=3

**State**: 2 servers available

**Result**: ❌ Failure - Only 2 shares, cannot reconstruct

**Pod Behavior**: Init container exits with error, pod enters `Init:Error` state

### Scenario 3: Network Partition

**State**: Init container can't reach any servers

**Result**: ❌ Failure - No shares fetched

**Retry Behavior**: Exponential backoff, max 3 retries per server

## Performance Characteristics

### Share Fetching

- **Parallelism**: All N servers contacted simultaneously
- **Latency**: ~50-200ms per request (local network)
- **Total Time**: Limited by slowest of K servers

### Reconstruction

- **Complexity**: O(K²) operations in GF(2^8)
- **Performance**: < 1ms for typical secret sizes (< 1KB)

### Bottlenecks

1. Network latency to share servers
2. gRPC connection establishment
3. JWT validation overhead

## Scalability Considerations

### Share Servers

- **Horizontal Scaling**: Limited by N (max 255 by SSS)
- **Resource Usage**: Minimal (< 100MB RAM per server)
- **Typical Configuration**: N=5-10, K=3-5

### Sidecar Impact

- **Pod Startup Time**: +1-5 seconds
- **Resource Overhead**: ~64MB RAM during init
- **Cleanup**: Resources released after init completes

## Design Trade-offs

| Aspect                    | Choice            | Rationale                | Trade-off                |
| ------------------------- | ----------------- | ------------------------ | ------------------------ |
| Init Container vs Sidecar | Init Container    | One-time reconstruction  | No rotation support      |
| gRPC vs HTTP              | gRPC              | Type safety, performance | Additional complexity    |
| Dev Mode vs Full JWT      | Dev Mode default  | Easier local testing     | Security in non-prod     |
| Parallel vs Sequential    | Parallel fetching | Faster reconstruction    | More network connections |
| StatefulSet vs Deployment | StatefulSet       | Stable identities        | Slightly slower rollout  |

## Extensibility Points

### Adding New Share Backends

Implement `ShareProvider` interface:

```go
type ShareProvider interface {
    GetShare(ctx context.Context, identity string) ([]byte, error)
}
```

### Custom Authentication

Implement `Validator` interface:

```go
type Validator interface {
    ValidateToken(ctx context.Context, token string) (*Claims, error)
}
```

### Alternative Reconstruction Algorithms

Replace `shamir.Combine()` with compatible SSS implementation.

## Future Enhancements (Out of Scope)

- Verifiable Secret Sharing (VSS)
- Proactive Secret Sharing (rotation)
- Byzantine fault tolerance
- Zero-knowledge proofs
- Multi-party computation (MPC)

## References

- Shamir, A. (1979). "How to share a secret"
- Kubernetes ServiceAccount Token Projection
- gRPC Authentication Guide
- tmpfs: Linux kernel documentation
