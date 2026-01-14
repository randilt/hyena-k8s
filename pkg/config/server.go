package config

import (
	"fmt"
	"os"
	"strings"

	"hyena-k8s/pkg/transport"
)

// ShareServerConfig holds configuration for the share server
type ShareServerConfig struct {
	// ServerID is a unique identifier for this share server
	ServerID string
	
	// Port is the gRPC server port
	Port int
	
	// ShareFile is the path to the file containing the share data
	ShareFile string
	
	// AllowedCallers is a list of authorized identities (namespace:sa-name)
	AllowedCallers []string
	
	// TLS configuration
	TLS transport.TLSConfig
	
	// DevMode enables development mode (skip JWT signature verification)
	DevMode bool
}

// LoadShareServerConfig loads configuration from environment variables
func LoadShareServerConfig() (*ShareServerConfig, error) {
	cfg := &ShareServerConfig{
		ServerID:  getEnvOrDefault("SERVER_ID", "share-server"),
		Port:      getEnvAsIntOrDefault("PORT", 9000),
		ShareFile: getEnvOrDefault("SHARE_FILE", "/secrets/share.bin"),
		DevMode:   getEnvAsBoolOrDefault("DEV_MODE", true),
	}

	// Parse allowed callers
	allowedCallersStr := getEnvOrDefault("ALLOWED_CALLERS", "")
	if allowedCallersStr == "" {
		return nil, fmt.Errorf("ALLOWED_CALLERS environment variable must be set")
	}
	cfg.AllowedCallers = strings.Split(allowedCallersStr, ",")
	for i, caller := range cfg.AllowedCallers {
		cfg.AllowedCallers[i] = strings.TrimSpace(caller)
	}

	// TLS configuration
	cfg.TLS = transport.TLSConfig{
		Enabled:            getEnvAsBoolOrDefault("TLS_ENABLED", false),
		CertFile:           getEnvOrDefault("TLS_CERT_PATH", ""),
		KeyFile:            getEnvOrDefault("TLS_KEY_PATH", ""),
		CAFile:             getEnvOrDefault("TLS_CA_PATH", ""),
		InsecureSkipVerify: false, // Never skip verify on server
	}

	// ShareFile is now optional - shares are stored in memory
	// This allows for dynamic share management

	return cfg, nil
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsIntOrDefault returns the environment variable as int or a default
func getEnvAsIntOrDefault(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	var value int
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsBoolOrDefault returns the environment variable as bool or a default
func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	return valueStr == "true" || valueStr == "1" || valueStr == "yes"
}
