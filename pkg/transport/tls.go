package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TLSConfig holds TLS configuration options
type TLSConfig struct {
	// Enabled indicates if TLS should be used
	Enabled bool
	
	// CertFile is the path to the server certificate
	CertFile string
	
	// KeyFile is the path to the server private key
	KeyFile string
	
	// CAFile is the path to the CA certificate for client verification
	CAFile string
	
	// InsecureSkipVerify skips certificate verification (dev only)
	InsecureSkipVerify bool
}

// LoadServerTLSConfig creates a TLS configuration for gRPC servers
func LoadServerTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Load server certificate and key
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server cert/key: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Optionally require client certificates
	if cfg.CAFile != "" {
		certPool := x509.NewCertPool()
		ca, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		if !certPool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.ClientCAs = certPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

// LoadClientTLSConfig creates a TLS configuration for gRPC clients
func LoadClientTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	// Load CA certificate for server verification
	if cfg.CAFile != "" && !cfg.InsecureSkipVerify {
		certPool := x509.NewCertPool()
		ca, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		if !certPool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.RootCAs = certPool
	}

	// Load client certificate if provided
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
