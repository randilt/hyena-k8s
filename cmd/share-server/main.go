package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"hyena-k8s/pkg/auth"
	"hyena-k8s/pkg/config"
	"hyena-k8s/pkg/transport"
	pb "hyena-k8s/proto/shareservice/v1"
)

// shareServer implements the ShareService gRPC service
type shareServer struct {
	pb.UnimplementedShareServiceServer
	serverID string
	shares   map[string][]byte // secretName -> shareData
	mu       sync.RWMutex
}

// GetShare returns the share stored by this server for a given secret
func (s *shareServer) GetShare(ctx context.Context, req *pb.GetShareRequest) (*pb.GetShareResponse, error) {
	log.Printf("GetShare request for secret '%s' from: %s", req.SecretName, req.RequesterIdentity)
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	share, exists := s.shares[req.SecretName]
	if !exists {
		return nil, fmt.Errorf("secret '%s' not found on this server", req.SecretName)
	}
	
	return &pb.GetShareResponse{
		Share:    share,
		ServerId: s.serverID,
	}, nil
}

// StoreShare stores a share for a given secret (admin only)
func (s *shareServer) StoreShare(ctx context.Context, req *pb.StoreShareRequest) (*pb.StoreShareResponse, error) {
	log.Printf("StoreShare request for secret '%s' from: %s", req.SecretName, req.RequesterIdentity)
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.shares[req.SecretName] = req.Share
	
	log.Printf("Stored share for secret '%s' (%d bytes)", req.SecretName, len(req.Share))
	
	return &pb.StoreShareResponse{
		Success: true,
		Message: fmt.Sprintf("Share stored successfully on %s", s.serverID),
	}, nil
}

// DeleteShare removes a share for a given secret (admin only)
func (s *shareServer) DeleteShare(ctx context.Context, req *pb.DeleteShareRequest) (*pb.DeleteShareResponse, error) {
	log.Printf("DeleteShare request for secret '%s' from: %s", req.SecretName, req.RequesterIdentity)
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.shares, req.SecretName)
	
	log.Printf("Deleted share for secret '%s'", req.SecretName)
	
	return &pb.DeleteShareResponse{
		Success: true,
		Message: fmt.Sprintf("Share deleted successfully from %s", s.serverID),
	}, nil
}

// HealthCheck returns the health status of the server
func (s *shareServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Healthy:  true,
		ServerId: s.serverID,
		Message:  "OK",
	}, nil
}

func main() {
	log.Printf("Starting share server...")

	// Load configuration
	cfg, err := config.LoadShareServerConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Server ID: %s", cfg.ServerID)
	log.Printf("Port: %d", cfg.Port)
	log.Printf("Allowed callers: %v", cfg.AllowedCallers)
	log.Printf("Dev mode: %t", cfg.DevMode)
	log.Printf("TLS enabled: %t", cfg.TLS.Enabled)

	// Create auth validator
	validator := auth.NewValidator(cfg.DevMode)

	// Create auth interceptor
	authInterceptor := transport.NewAuthInterceptor(validator, cfg.AllowedCallers)

	// Create gRPC server options
	var opts []grpc.ServerOption
	
	// Add auth interceptors
	opts = append(opts,
		grpc.UnaryInterceptor(authInterceptor.Unary()),
		grpc.StreamInterceptor(authInterceptor.Stream()),
	)

	// Add TLS if enabled
	if cfg.TLS.Enabled {
		tlsConfig, err := transport.LoadServerTLSConfig(&cfg.TLS)
		if err != nil {
			log.Fatalf("Failed to load TLS config: %v", err)
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.Creds(creds))
		log.Printf("TLS enabled")
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(opts...)

	// Register service
	server := &shareServer{
		serverID: cfg.ServerID,
		shares:   make(map[string][]byte),
	}
	pb.RegisterShareServiceServer(grpcServer, server)

	// Create listener
	addr := fmt.Sprintf(":%d", cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}

	log.Printf("Share server listening on %s", addr)

	// Start server in a goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Printf("Shutting down gracefully...")
	grpcServer.GracefulStop()
	log.Printf("Server stopped")
}
