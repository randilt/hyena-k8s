package transport

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"hyena-k8s/pkg/auth"
)

// AuthInterceptor provides authentication for gRPC requests
type AuthInterceptor struct {
	validator      *auth.Validator
	allowedCallers map[string]bool // map of "namespace:sa-name" -> true
}

// NewAuthInterceptor creates a new auth interceptor
func NewAuthInterceptor(validator *auth.Validator, allowedCallers []string) *AuthInterceptor {
	allowed := make(map[string]bool)
	for _, caller := range allowedCallers {
		allowed[caller] = true
	}
	return &AuthInterceptor{
		validator:      validator,
		allowedCallers: allowed,
	}
}

// Unary returns a server interceptor function for unary RPCs
func (i *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip auth for health check and admin methods (StoreShare, DeleteShare)
		if strings.HasSuffix(info.FullMethod, "HealthCheck") ||
			strings.HasSuffix(info.FullMethod, "StoreShare") ||
			strings.HasSuffix(info.FullMethod, "DeleteShare") {
			log.Printf("AUTH SKIPPED for admin method: %s", info.FullMethod)
			return handler(ctx, req)
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			log.Printf("AUTH DENIED: missing metadata")
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			log.Printf("AUTH DENIED: missing authorization header")
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := values[0]
		
		// Validate token
		claims, err := i.validator.ValidateToken(ctx, token)
		if err != nil {
			log.Printf("AUTH DENIED: invalid token: %v", err)
			return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("invalid token: %v", err))
		}

		// Check if caller is authorized
		identity := claims.Identity()
		if !i.allowedCallers[identity] {
			log.Printf("AUTH DENIED: unauthorized caller: %s", identity)
			return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("unauthorized caller: %s", identity))
		}

		log.Printf("AUTH SUCCESS: %s", identity)

		// Add claims to context for handler
		ctx = context.WithValue(ctx, "claims", claims)

		// Call the handler
		return handler(ctx, req)
	}
}

// Stream returns a server interceptor function for streaming RPCs
func (i *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// For this PoC, we don't use streaming RPCs
		// But implementing for completeness
		ctx := ss.Context()
		
		// Skip auth for health check and admin methods
		if strings.HasSuffix(info.FullMethod, "HealthCheck") ||
			strings.HasSuffix(info.FullMethod, "StoreShare") ||
			strings.HasSuffix(info.FullMethod, "DeleteShare") {
			return handler(srv, ss)
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := values[0]
		
		// Validate token
		claims, err := i.validator.ValidateToken(ctx, token)
		if err != nil {
			return status.Error(codes.Unauthenticated, fmt.Sprintf("invalid token: %v", err))
		}

		// Check if caller is authorized
		identity := claims.Identity()
		if !i.allowedCallers[identity] {
			return status.Error(codes.PermissionDenied, fmt.Sprintf("unauthorized caller: %s", identity))
		}

		// Call the handler
		return handler(srv, ss)
	}
}
