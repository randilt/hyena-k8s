package shareserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
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
	serverID  string
	shareData []byte
}

// GetShare returns the share stored by this server
func (s *shareServer) GetShare(ctx context.Context, req *pb.GetShareRequest) (*pb.GetShareResponse, error) {
	log.Printf("GetShare request from: %s", req.RequesterIdentity)
	
	// The authentication is already handled by the interceptor
	// If we reach here, the caller is authorized
	
	return &pb.GetShareResponse{
		Share:    s.shareData,
		ServerId: s.serverID,
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
	log.Printf("Share file: %s", cfg.ShareFile)
	log.Printf("Allowed callers: %v", cfg.AllowedCallers)
	log.Printf("Dev mode: %t", cfg.DevMode)
	log.Printf("TLS enabled: %t", cfg.TLS.Enabled)

	// Load share data
	shareData, err := os.ReadFile(cfg.ShareFile)
	if err != nil {
		log.Fatalf("Failed to read share file: %v", err)
	}
	log.Printf("Loaded share data: %d bytes", len(shareData))

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
		serverID:  cfg.ServerID,
		shareData: shareData,
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
