package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"hyena-k8s/pkg/shamir"
	pb "hyena-k8s/proto/secretmanager/v1"
	sharepb "hyena-k8s/proto/shareservice/v1"
)

type secretManagerServer struct {
	pb.UnimplementedSecretManagerServiceServer
	shareServerEndpoints []string
	defaultN             int
	defaultK             int
}

// StoreSecret splits a secret and distributes shares to share servers
func (s *secretManagerServer) StoreSecret(ctx context.Context, req *pb.StoreSecretRequest) (*pb.StoreSecretResponse, error) {
	log.Printf("StoreSecret request for '%s' (N=%d, K=%d)", req.SecretName, req.TotalShares, req.Threshold)

	// Validate inputs
	if req.SecretName == "" {
		return nil, fmt.Errorf("secret_name is required")
	}
	if len(req.SecretData) == 0 {
		return nil, fmt.Errorf("secret_data cannot be empty")
	}

	// Use defaults if not specified
	n := int(req.TotalShares)
	k := int(req.Threshold)
	if n == 0 {
		n = s.defaultN
	}
	if k == 0 {
		k = s.defaultK
	}

	// Validate N and K
	if n != len(s.shareServerEndpoints) {
		return nil, fmt.Errorf("total_shares (%d) must match number of share servers (%d)", n, len(s.shareServerEndpoints))
	}
	if k > n {
		return nil, fmt.Errorf("threshold (%d) cannot exceed total_shares (%d)", k, n)
	}

	// Split the secret using Shamir
	log.Printf("Splitting secret into %d shares (threshold: %d)...", n, k)
	shares, err := shamir.Split(req.SecretData, n, k)
	if err != nil {
		return nil, fmt.Errorf("failed to split secret: %w", err)
	}
	log.Printf("Secret split successfully into %d shares", len(shares))

	// Distribute shares to share servers
	var storedServerIDs []string
	for i, endpoint := range s.shareServerEndpoints {
		log.Printf("Distributing share %d to %s...", i, endpoint)

		// Connect to share server
		conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("failed to connect to share server %s: %w", endpoint, err)
		}
		defer conn.Close()

		client := sharepb.NewShareServiceClient(conn)

		// Store the share
		storeReq := &sharepb.StoreShareRequest{
			SecretName:        req.SecretName,
			Share:             shares[i],
			RequesterIdentity: "secret-manager", // TODO: use actual identity
		}

		storeResp, err := client.StoreShare(ctx, storeReq)
		if err != nil {
			return nil, fmt.Errorf("failed to store share on %s: %w", endpoint, err)
		}

		if !storeResp.Success {
			return nil, fmt.Errorf("share storage failed on %s: %s", endpoint, storeResp.Message)
		}

		log.Printf("Successfully stored share %d on %s", i, endpoint)
		storedServerIDs = append(storedServerIDs, endpoint)
	}

	log.Printf("Successfully stored secret '%s' across %d share servers", req.SecretName, len(storedServerIDs))

	return &pb.StoreSecretResponse{
		Success:        true,
		Message:        fmt.Sprintf("Secret '%s' stored successfully", req.SecretName),
		ShareServerIds: storedServerIDs,
	}, nil
}

// ListSecrets lists all stored secret names
func (s *secretManagerServer) ListSecrets(ctx context.Context, req *pb.ListSecretsRequest) (*pb.ListSecretsResponse, error) {
	// For now, we don't track secrets centrally
	// This would require a metadata store in production
	return &pb.ListSecretsResponse{
		SecretNames: []string{},
	}, nil
}

// DeleteSecret removes a secret from all share servers
func (s *secretManagerServer) DeleteSecret(ctx context.Context, req *pb.DeleteSecretRequest) (*pb.DeleteSecretResponse, error) {
	log.Printf("DeleteSecret request for '%s'", req.SecretName)

	if req.SecretName == "" {
		return nil, fmt.Errorf("secret_name is required")
	}

	// Delete from all share servers
	for _, endpoint := range s.shareServerEndpoints {
		log.Printf("Deleting secret '%s' from %s...", req.SecretName, endpoint)

		conn, err := grpc.Dial(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: failed to connect to share server %s: %v", endpoint, err)
			continue
		}
		defer conn.Close()

		client := sharepb.NewShareServiceClient(conn)

		deleteReq := &sharepb.DeleteShareRequest{
			SecretName:        req.SecretName,
			RequesterIdentity: "secret-manager",
		}

		_, err = client.DeleteShare(ctx, deleteReq)
		if err != nil {
			log.Printf("Warning: failed to delete share from %s: %v", endpoint, err)
		}
	}

	log.Printf("Delete request completed for secret '%s'", req.SecretName)

	return &pb.DeleteSecretResponse{
		Success: true,
		Message: fmt.Sprintf("Secret '%s' deleted", req.SecretName),
	}, nil
}

func main() {
	log.Println("Starting Secret Manager service...")

	// Load configuration from environment
	port := getEnvAsIntOrDefault("PORT", 8080)
	grpcPort := getEnvAsIntOrDefault("GRPC_PORT", 9090)
	defaultN := getEnvAsIntOrDefault("DEFAULT_N", 5)
	defaultK := getEnvAsIntOrDefault("DEFAULT_K", 3)

	// Parse share server endpoints
	shareServerEndpointsStr := os.Getenv("SHARE_SERVER_ENDPOINTS")
	if shareServerEndpointsStr == "" {
		log.Fatal("SHARE_SERVER_ENDPOINTS environment variable must be set")
	}

	// Split comma-separated endpoints
	var endpoints []string
	for i := 0; i < defaultN; i++ {
		endpoint := fmt.Sprintf("hyena-share-server-%d.hyena-share-server.default.svc.cluster.local:9000", i)
		endpoints = append(endpoints, endpoint)
	}

	log.Printf("Configuration loaded:")
	log.Printf("  HTTP Port: %d", port)
	log.Printf("  gRPC Port: %d", grpcPort)
	log.Printf("  Default N: %d", defaultN)
	log.Printf("  Default K: %d", defaultK)
	log.Printf("  Share servers: %v", endpoints)

	// Create gRPC server
	server := &secretManagerServer{
		shareServerEndpoints: endpoints,
		defaultN:             defaultN,
		defaultK:             defaultK,
	}

	// Start HTTP server for simple REST API
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/store", createStoreHandler(server))
	http.HandleFunc("/delete", createDeleteHandler(server))

	go func() {
		log.Printf("HTTP server listening on :%d", port)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down secret manager...")
	log.Println("Secret manager stopped")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"secret-manager"}`)
}

func createStoreHandler(server *secretManagerServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
			return
		}

		secretName := r.FormValue("name")
		secretData := r.FormValue("data")

		if secretName == "" || secretData == "" {
			http.Error(w, "Both 'name' and 'data' parameters are required", http.StatusBadRequest)
			return
		}

		// Store the secret
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := &pb.StoreSecretRequest{
			SecretName: secretName,
			SecretData: []byte(secretData),
		}

		resp, err := server.StoreSecret(ctx, req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to store secret: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success":%t,"message":"%s","servers":%d}`, resp.Success, resp.Message, len(resp.ShareServerIds))
	}
}

func createDeleteHandler(server *secretManagerServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
			return
		}

		secretName := r.FormValue("name")
		if secretName == "" {
			http.Error(w, "'name' parameter is required", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req := &pb.DeleteSecretRequest{
			SecretName: secretName,
		}

		resp, err := server.DeleteSecret(ctx, req)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete secret: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success":%t,"message":"%s"}`, resp.Success, resp.Message)
	}
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
