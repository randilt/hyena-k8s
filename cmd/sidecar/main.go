package sidecar

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
	log.Printf("  Output path: %s", cfg.OutputPath)
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

	// Create share fetcher
	fetcher := &client.ShareFetcher{
		ServerEndpoints:   cfg.ServerEndpoints,
		Threshold:         cfg.Threshold,
		Timeout:           cfg.Timeout,
		MaxRetries:        cfg.MaxRetries,
		TLSConfig:         tlsConfig,
		JWTToken:          string(token),
		RequesterIdentity: cfg.RequesterIdentity,
	}

	// Fetch shares
	log.Printf("Fetching %d shares from %d servers...", cfg.Threshold, len(cfg.ServerEndpoints))
	ctx := context.Background()
	shares, err := fetcher.FetchShares(ctx)
	if err != nil {
		log.Printf("FATAL: Failed to fetch shares: %v", err)
		os.Exit(ExitFetchError)
	}

	log.Printf("Successfully fetched %d shares", len(shares))

	// Reconstruct secret
	log.Printf("Reconstructing secret...")
	secret, err := shamir.Combine(shares)
	if err != nil {
		log.Printf("FATAL: Failed to reconstruct secret: %v", err)
		os.Exit(ExitReconstructionError)
	}

	log.Printf("Secret reconstructed successfully (%d bytes)", len(secret))

	// Write secret to output path
	log.Printf("Writing secret to %s...", cfg.OutputPath)
	
	// Create directory if it doesn't exist
	dir := cfg.OutputPath[:len(cfg.OutputPath)-len("/secret")]
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("FATAL: Failed to create output directory: %v", err)
			os.Exit(ExitWriteError)
		}
	}

	// Write with restrictive permissions (read-only for owner)
	if err := os.WriteFile(cfg.OutputPath, secret, 0400); err != nil {
		log.Printf("FATAL: Failed to write secret file: %v", err)
		os.Exit(ExitWriteError)
	}

	log.Printf("Secret written successfully to %s", cfg.OutputPath)
	log.Printf("Sidecar reconstructor completed successfully")
	
	os.Exit(ExitSuccess)
}
