package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "hyena-k8s/proto/shareservice/v1"
)

// ShareFetcher fetches shares from multiple share servers
type ShareFetcher struct {
	// ServerEndpoints is a list of server addresses (host:port)
	ServerEndpoints []string
	
	// Threshold is the minimum number of shares needed (K)
	Threshold int
	
	// Timeout is the timeout for each fetch request
	Timeout time.Duration
	
	// MaxRetries is the maximum number of retry attempts per server
	MaxRetries int
	
	// TLSConfig for secure connections
	TLSConfig *tls.Config
	
	// JWTToken is the ServiceAccount token for authentication
	JWTToken string
	
	// RequesterIdentity is the identity sent in the request
	RequesterIdentity string
}

// ShareResult represents a fetched share or an error
type ShareResult struct {
	Share    []byte
	ServerID string
	Error    error
}

// FetchShares fetches shares from all servers in parallel
// Returns at least K shares or an error if unable to collect enough
func (f *ShareFetcher) FetchShares(ctx context.Context) ([][]byte, error) {
	log.Printf("Fetching shares from %d servers (need %d)...", len(f.ServerEndpoints), f.Threshold)

	if f.Threshold > len(f.ServerEndpoints) {
		return nil, fmt.Errorf("threshold (%d) is greater than number of servers (%d)", f.Threshold, len(f.ServerEndpoints))
	}

	// Channel to collect successful shares
	shareChan := make(chan ShareResult, len(f.ServerEndpoints))
	
	// Use errgroup for parallel fetching
	g, gctx := errgroup.WithContext(ctx)

	// Launch goroutines to fetch from each server
	for _, endpoint := range f.ServerEndpoints {
		endpoint := endpoint // capture loop variable
		g.Go(func() error {
			share, serverID, err := f.fetchFromServer(gctx, endpoint)
			shareChan <- ShareResult{
				Share:    share,
				ServerID: serverID,
				Error:    err,
			}
			return nil // Don't fail the errgroup, collect all results
		})
	}

	// Wait for all goroutines to complete
	g.Wait()
	close(shareChan)

	// Collect successful shares
	var shares [][]byte
	var errors []error
	
	for result := range shareChan {
		if result.Error != nil {
			log.Printf("Failed to fetch from server %s: %v", result.ServerID, result.Error)
			errors = append(errors, result.Error)
		} else {
			log.Printf("Successfully fetched share from server %s (%d bytes)", result.ServerID, len(result.Share))
			shares = append(shares, result.Share)
			
			// Early exit if we have enough shares (optimization)
			if len(shares) >= f.Threshold {
				log.Printf("Collected %d shares (threshold met)", len(shares))
				break
			}
		}
	}

	// Check if we have enough shares
	if len(shares) < f.Threshold {
		return nil, fmt.Errorf("failed to collect enough shares: got %d, need %d (errors: %d)", 
			len(shares), f.Threshold, len(errors))
	}

	log.Printf("Successfully collected %d shares", len(shares))
	return shares[:f.Threshold], nil // Return exactly K shares
}

// fetchFromServer fetches a share from a single server with retries
func (f *ShareFetcher) fetchFromServer(ctx context.Context, endpoint string) ([]byte, string, error) {
	var lastErr error
	
	for attempt := 0; attempt <= f.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying %s (attempt %d/%d)...", endpoint, attempt, f.MaxRetries)
			// Exponential backoff
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, "", ctx.Err()
			}
		}

		share, serverID, err := f.fetchAttempt(ctx, endpoint)
		if err == nil {
			return share, serverID, nil
		}
		
		lastErr = err
		log.Printf("Attempt %d failed for %s: %v", attempt, endpoint, err)
	}

	return nil, "", fmt.Errorf("all retries exhausted: %w", lastErr)
}

// fetchAttempt makes a single fetch attempt to a server
func (f *ShareFetcher) fetchAttempt(ctx context.Context, endpoint string) ([]byte, string, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, f.Timeout)
	defer cancel()

	// Setup gRPC dial options
	var opts []grpc.DialOption
	
	if f.TLSConfig != nil {
		creds := credentials.NewTLS(f.TLSConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Connect to server
	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Create client
	client := pb.NewShareServiceClient(conn)

	// Add JWT token to metadata
	md := metadata.New(map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", f.JWTToken),
	})
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Make request
	req := &pb.GetShareRequest{
		RequesterIdentity: f.RequesterIdentity,
	}

	resp, err := client.GetShare(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("GetShare RPC failed: %w", err)
	}

	return resp.Share, resp.ServerId, nil
}

// HealthCheck checks if a server is healthy
func (f *ShareFetcher) HealthCheck(ctx context.Context, endpoint string) error {
	ctx, cancel := context.WithTimeout(ctx, f.Timeout)
	defer cancel()

	var opts []grpc.DialOption
	if f.TLSConfig != nil {
		creds := credentials.NewTLS(f.TLSConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	client := pb.NewShareServiceClient(conn)

	req := &pb.HealthCheckRequest{}
	resp, err := client.HealthCheck(ctx, req)
	if err != nil {
		return fmt.Errorf("HealthCheck RPC failed: %w", err)
	}

	if !resp.Healthy {
		return fmt.Errorf("server reports unhealthy: %s", resp.Message)
	}

	return nil
}
