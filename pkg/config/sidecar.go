package config

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"
)

// SidecarConfig holds configuration for the sidecar reconstructor
type SidecarConfig struct {
	// ServerEndpoints is a list of share server addresses
	ServerEndpoints []string
	
	// Threshold is the minimum number of shares needed (K)
	Threshold int
	
	// OutputPath is where the reconstructed secret will be written
	OutputPath string
	
	// Timeout for each share fetch request
	Timeout time.Duration
	
	// MaxRetries per server
	MaxRetries int
	
	// TLSCAPath is the path to the CA certificate
	TLSCAPath string
	
	// TLSInsecure skips certificate verification
	TLSInsecure bool
	
	// ServiceAccountTokenPath is the path to the K8s SA token
	ServiceAccountTokenPath string
	
	// RequesterIdentity for the GetShare request
	RequesterIdentity string
}

// LoadSidecarConfig loads configuration from environment variables
func LoadSidecarConfig() (*SidecarConfig, error) {
	cfg := &SidecarConfig{
		OutputPath:              getEnvOrDefault("SECRET_OUTPUT_PATH", "/secrets/secret"),
		Threshold:               getEnvAsIntOrDefault("THRESHOLD", 3),
		MaxRetries:              getEnvAsIntOrDefault("MAX_RETRIES", 3),
		TLSInsecure:             getEnvAsBoolOrDefault("TLS_INSECURE", true),
		TLSCAPath:               getEnvOrDefault("TLS_CA_PATH", ""),
		ServiceAccountTokenPath: getEnvOrDefault("SERVICE_ACCOUNT_TOKEN_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token"),
		RequesterIdentity:       getEnvOrDefault("REQUESTER_IDENTITY", ""),
	}

	// Parse timeout
	timeoutStr := getEnvOrDefault("TIMEOUT", "10s")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid TIMEOUT value: %w", err)
	}
	cfg.Timeout = timeout

	// Parse server endpoints
	endpointsStr := getEnvOrDefault("SHARE_SERVER_ENDPOINTS", "")
	if endpointsStr == "" {
		return nil, fmt.Errorf("SHARE_SERVER_ENDPOINTS environment variable must be set")
	}
	cfg.ServerEndpoints = strings.Split(endpointsStr, ",")
	for i, endpoint := range cfg.ServerEndpoints {
		cfg.ServerEndpoints[i] = strings.TrimSpace(endpoint)
	}

	if len(cfg.ServerEndpoints) == 0 {
		return nil, fmt.Errorf("no server endpoints provided")
	}

	if cfg.Threshold < 2 {
		return nil, fmt.Errorf("threshold must be at least 2")
	}

	if cfg.Threshold > len(cfg.ServerEndpoints) {
		return nil, fmt.Errorf("threshold (%d) cannot exceed number of servers (%d)", 
			cfg.Threshold, len(cfg.ServerEndpoints))
	}

	return cfg, nil
}

// LoadTLSConfig creates a TLS configuration from the sidecar config
func (cfg *SidecarConfig) LoadTLSConfig() (*tls.Config, error) {
	if cfg.TLSInsecure && cfg.TLSCAPath == "" {
		// No TLS or insecure mode
		return nil, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.TLSInsecure,
		MinVersion:         tls.VersionTLS12,
	}

	// Load CA certificate if provided
	if cfg.TLSCAPath != "" && !cfg.TLSInsecure {
		// This would load the CA cert, but for simplicity in PoC we'll skip
		// The actual implementation would use x509.CertPool
	}

	return tlsConfig, nil
}
