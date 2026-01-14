package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Validator handles JWT token validation
type Validator struct {
	// devMode skips signature verification (for local testing)
	devMode bool
}

// NewValidator creates a new JWT validator
func NewValidator(devMode bool) *Validator {
	return &Validator{
		devMode: devMode,
	}
}

// ValidateToken validates a Kubernetes ServiceAccount JWT token
// In dev mode, it only validates the format and extracts claims
// In production mode, it would verify the signature against K8s API
func (v *Validator) ValidateToken(ctx context.Context, token string) (*Claims, error) {
	if token == "" {
		return nil, fmt.Errorf("token is empty")
	}

	// Remove "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)

	// Parse JWT structure (header.payload.signature)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	// Decode payload (base64url encoded)
	payload, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse claims
	var rawClaims map[string]interface{}
	if err := json.Unmarshal(payload, &rawClaims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	// Extract subject
	subject, ok := rawClaims["sub"].(string)
	if !ok || subject == "" {
		return nil, fmt.Errorf("JWT missing or invalid 'sub' claim")
	}

	// Parse the service account identity
	claims, err := ParseServiceAccountIdentity(subject)
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account identity: %w", err)
	}

	// In production mode, we would verify the signature here
	// For this PoC, dev mode allows testing without K8s API access
	if !v.devMode {
		// TODO: Implement signature verification using K8s public keys
		// This would involve:
		// 1. Fetching the K8s API server's public key
		// 2. Verifying the JWT signature
		// 3. Checking expiration and other standard JWT validations
		return nil, fmt.Errorf("production mode JWT verification not yet implemented")
	}

	return claims, nil
}

// ValidateTokenFromFile reads and validates a token from a file
// This is useful for reading ServiceAccount tokens mounted in pods
func (v *Validator) ValidateTokenFromFile(ctx context.Context, path string) (*Claims, error) {
	token, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	return v.ValidateToken(ctx, string(token))
}

// decodeBase64URL decodes a base64url encoded string
func decodeBase64URL(s string) ([]byte, error) {
	// Base64url uses different padding rules
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}

	// Replace base64url characters with standard base64
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	// Decode using standard base64
	return base64.StdEncoding.DecodeString(s)
}
