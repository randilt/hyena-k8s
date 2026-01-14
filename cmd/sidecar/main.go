package main

import (
	"context"
	"log"
	"os"

	"hyena-k8s/pkg/client"
	"hyena-k8s/pkg/config"
	"hyena-k8s/pkg/shamir"
)

// Exit codes
const (
	ExitSuccess                = 0
	ExitConfigError            = 1
	ExitTokenReadError         = 2
	ExitFetchError             = 3
	ExitReconstructionError    = 4
	ExitWriteError             = 5
)

func main() {
	log.Printf("Starting sidecar reconstructor...")

	// Load configuration
	cfg, err := config.LoadSidecarConfig()
	if err != nil {
		log.Printf("FATAL: Failed to load configuration: %v", err)
		os.Exit(ExitConfigError)
	}

	log.Printf("Configuration loaded:")
	log.Printf("  Endpoints: %v", cfg.ServerEndpoints)
	log.Printf("  Threshold: %d", cfg.Threshold)
	log.Printf("  Secrets to fetch: %d", len(cfg.Secrets))
	for i, secret := range cfg.Secrets {
		log.Printf("    [%d] %s -> %s", i+1, secret.Name, secret.OutputPath)
	}
	log.Printf("  Timeout: %s", cfg.Timeout)
	log.Printf("  Max retries: %d", cfg.MaxRetries)

	// Read ServiceAccount token
	token, err := os.ReadFile(cfg.ServiceAccountTokenPath)
	if err != nil {
		log.Printf("FATAL: Failed to read ServiceAccount token: %v", err)
		os.Exit(ExitTokenReadError)
	}
	log.Printf("ServiceAccount token loaded from %s", cfg.ServiceAccountTokenPath)

	// Load TLS config
	tlsConfig, err := cfg.LoadTLSConfig()
	if err != nil {
		log.Printf("FATAL: Failed to load TLS config: %v", err)
		os.Exit(ExitConfigError)
	}

	ctx := context.Background()
	
	// Fetch and reconstruct each secret
	for _, secretMapping := range cfg.Secrets {
		log.Printf("\n=== Processing secret: %s ===", secretMapping.Name)
		
		// Create share fetcher for this secret
		fetcher := &client.ShareFetcher{
			ServerEndpoints:   cfg.ServerEndpoints,
			Threshold:         cfg.Threshold,
			SecretName:        secretMapping.Name,
			Timeout:           cfg.Timeout,
			MaxRetries:        cfg.MaxRetries,
			TLSConfig:         tlsConfig,
			JWTToken:          string(token),
			RequesterIdentity: cfg.RequesterIdentity,
		}

		// Fetch shares
		log.Printf("Fetching %d shares from %d servers for secret '%s'...", cfg.Threshold, len(cfg.ServerEndpoints), secretMapping.Name)
		shares, err := fetcher.FetchShares(ctx)
		if err != nil {
			log.Printf("FATAL: Failed to fetch shares for '%s': %v", secretMapping.Name, err)
			os.Exit(ExitFetchError)
		}

		log.Printf("Successfully fetched %d shares for '%s'", len(shares), secretMapping.Name)

		// Reconstruct secret
		log.Printf("Reconstructing secret '%s'...", secretMapping.Name)
		secret, err := shamir.Combine(shares)
		if err != nil {
			log.Printf("FATAL: Failed to reconstruct secret '%s': %v", secretMapping.Name, err)
			os.Exit(ExitReconstructionError)
		}

		log.Printf("Secret '%s' reconstructed successfully (%d bytes)", secretMapping.Name, len(secret))

		// Write secret to output path
		log.Printf("Writing secret '%s' to %s...", secretMapping.Name, secretMapping.OutputPath)
		
		// Create directory if it doesn't exist
		dir := secretMapping.OutputPath[:len(secretMapping.OutputPath)-len("/"+secretMapping.Name)]
		if dir != "" && dir != secretMapping.OutputPath {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("FATAL: Failed to create output directory for '%s': %v", secretMapping.Name, err)
				os.Exit(ExitWriteError)
			}
		}

		// Write with restrictive permissions (read-only for owner)
		if err := os.WriteFile(secretMapping.OutputPath, secret, 0400); err != nil {
			log.Printf("FATAL: Failed to write secret file for '%s': %v", secretMapping.Name, err)
			os.Exit(ExitWriteError)
		}

		log.Printf("Secret '%s' written successfully to %s", secretMapping.Name, secretMapping.OutputPath)
	}

	log.Printf("\n=== All secrets processed successfully ===")
	log.Printf("Sidecar reconstructor completed successfully")
	
	os.Exit(ExitSuccess)
}
