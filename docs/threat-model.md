# Threat Model

## Scope

This threat model analyzes the security properties and limitations of Hyena-K8s as a **proof-of-concept** system.

## Trust Assumptions

### Trusted Components

1. **Kubernetes Control Plane**

   - Assumption: K8s API server correctly validates ServiceAccount tokens
   - Risk: If compromised, attacker can impersonate any identity

2. **Container Runtime**

   - Assumption: Container isolation is effective
   - Risk: Container escape could expose tmpfs volumes

3. **Linux Kernel**

   - Assumption: tmpfs provides memory-only storage
   - Risk: Kernel vulnerabilities could leak memory

4. **Share Server Code**

   - Assumption: Share servers correctly implement authorization
   - Risk: Bugs could allow unauthorized access

5. **Network**
   - Assumption: In-cluster network is not compromised
   - Risk: MITM attacks without TLS

### Untrusted Components

1. **Application Code**

   - No assumptions about application security
   - Secret exposed via tmpfs is application's responsibility

2. **Cluster Administrators**
   - Can access etcd, node filesystems, and debug pods
   - Not protected against admin access

## Threat Analysis

### T1: Compromise of Individual Share Servers

**Attack**: Attacker gains access to < K share servers

**Impact**: Cannot reconstruct secret (threshold property)

**Mitigation**: ✅ Inherent to Shamir's Secret Sharing

**Residual Risk**: LOW

---

### T2: Compromise of K or More Share Servers

**Attack**: Attacker gains access to ≥ K shares

**Impact**: ⚠️ **SECRET COMPROMISED** - Can reconstruct full secret

**Mitigation**:

- Network segmentation
- RBAC to prevent pod exec
- Monitoring and alerting

**Residual Risk**: MEDIUM-HIGH (depends on K and security controls)

---

### T3: JWT Token Theft

**Attack**: Attacker steals ServiceAccount token from authorized pod

**Impact**: Can fetch shares as legitimate client

**Mitigation**:

- Short token expiration (configured to 1 hour)
- Token audience validation (not implemented in dev mode)
- Network policies to limit share server access

**Residual Risk**: MEDIUM

---

### T4: Memory Dumps

**Attack**: Attacker dumps init container or application memory

**Impact**: Can extract reconstructed secret from memory

**Mitigation**: ❌ None - Out of scope

**Residual Risk**: HIGH (if attacker has memory access)

---

### T5: etcd Access

**Attack**: Cluster admin accesses etcd directly

**Impact**: Can read shares from Secrets, reconstruct if ≥ K obtained

**Mitigation**:

- etcd encryption at rest
- etcd access controls
- Audit logging

**Residual Risk**: HIGH (trusted administrators)

---

### T6: Man-in-the-Middle (MITM)

**Attack**: Intercept gRPC communication between sidecar and servers

**Impact**: Can steal shares in transit

**Mitigation**:

- TLS for gRPC (not enabled in default PoC)
- Network policies

**Residual Risk**: MEDIUM (without TLS), LOW (with TLS)

---

### T7: Replay Attacks

**Attack**: Replay captured JWT token to fetch shares

**Impact**: Can fetch shares if token not expired

**Mitigation**:

- Short token expiration
- Nonce-based authentication (not implemented)

**Residual Risk**: LOW-MEDIUM

---

### T8: Unauthorized Pod Creation

**Attack**: Create malicious pod with authorized ServiceAccount

**Impact**: Can fetch shares and reconstruct secret

**Mitigation**:

- RBAC to restrict pod creation
- Admission controllers (not implemented)
- ServiceAccount token audience validation

**Residual Risk**: MEDIUM

---

### T9: Side-Channel Attacks

**Attack**: Timing attacks, cache attacks, or speculative execution

**Impact**: Potential secret leakage through timing or cache patterns

**Mitigation**: ❌ None - Out of scope

**Residual Risk**: UNKNOWN (not analyzed)

---

### T10: Share Server DoS

**Attack**: Overwhelm share servers with requests

**Impact**: Application pods fail to start

**Mitigation**:

- Rate limiting (not implemented)
- Resource quotas
- Multiple server replicas

**Residual Risk**: MEDIUM

---

## Security Properties

### ✅ Provided

1. **Threshold Security**: Secret cannot be reconstructed with < K shares
2. **Distributed Trust**: No single point of compromise (for < K servers)
3. **Memory-Only Secrets**: No plaintext secret on disk
4. **Authentication**: JWT-based access control to shares
5. **Least Privilege**: Application only gets secret, not shares

### ❌ NOT Provided

1. **Perfect Forward Secrecy**: Old shares can reconstruct old secrets
2. **Verifiable Shares**: Cannot detect if shares are tampered
3. **Byzantine Fault Tolerance**: Assumes honest share servers
4. **Secret Rotation**: No mechanism to update secrets
5. **Audit Trails**: Minimal logging of share access
6. **Memory Protection**: No protection against memory dumps
7. **Formal Verification**: No formal security proof

## Attack Surface

### External

- **gRPC Endpoints**: Port 9000 on share servers

  - Attack Vector: Unauthorized share requests
  - Mitigation: JWT authentication, network policies

- **HTTP Endpoint**: Port 8080 on demo app
  - Attack Vector: DoS, info disclosure
  - Mitigation: Does not expose secret value

### Internal (Cluster)

- **Pod-to-Pod**: Init container → Share servers

  - Attack Vector: Token theft, MITM
  - Mitigation: ServiceAccount tokens, TLS (optional)

- **etcd**: Shares stored as Secrets

  - Attack Vector: Direct etcd access
  - Mitigation: etcd encryption, access controls

- **Kubelet API**: Pod inspection via kubelet
  - Attack Vector: Memory inspection, log access
  - Mitigation: Kubelet authentication/authorization

## Compliance Considerations

### ❌ NOT Suitable For:

- **PCI-DSS**: Lacks required audit logging and key management
- **HIPAA**: No BAA compliance, insufficient access controls
- **SOC 2**: Lacks monitoring, logging, and change management
- **GDPR**: No data minimization or right-to-erasure support

### Potential Use Cases:

- **Research and Education**: Demonstrating threshold cryptography
- **Development Environments**: Non-sensitive test data
- **POC/MVPs**: Proof-of-concept applications

## Recommendations for Production Use

If this were to evolve to production (NOT CURRENT SCOPE):

1. **Enable TLS**:

   - mTLS between all components
   - Certificate management via cert-manager

2. **Implement Audit Logging**:

   - Log all share access attempts
   - Central log aggregation
   - Alert on suspicious patterns

3. **Add Secret Rotation**:

   - Periodic share regeneration
   - Proactive secret sharing

4. **Enhance Authentication**:

   - Full JWT signature verification
   - Token audience validation
   - Nonce-based replay protection

5. **Formal Security Audit**:

   - Third-party penetration testing
   - Code security review
   - Cryptographic analysis

6. **Implement VSS**:

   - Verifiable Secret Sharing
   - Detect tampered shares

7. **Memory Protection**:

   - Encrypted memory regions
   - Secure enclaves (SGX, SEV)

8. **Network Security**:

   - Strict NetworkPolicies
   - Service mesh (mutual TLS)

9. **Monitoring and Alerting**:

   - Prometheus metrics
   - Failed authentication alerts
   - Anomaly detection

10. **Disaster Recovery**:
    - Backup and restore procedures
    - Key escrow mechanisms

## Conclusion

Hyena-K8s demonstrates the **feasibility** of threshold cryptography for Kubernetes secrets but is **not secure enough for production use** due to:

- Lack of comprehensive logging
- Optional TLS
- No secret rotation
- No verifiable shares
- Minimal monitoring
- Dev mode defaults

**Use this only for research, education, and non-sensitive testing.**

For production workloads, use established solutions like:

- HashiCorp Vault
- AWS Secrets Manager
- GCP Secret Manager
- Azure Key Vault
