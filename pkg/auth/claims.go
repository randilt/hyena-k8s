package auth

import (
	"fmt"
	"strings"
)

// Claims represents parsed JWT claims for Kubernetes ServiceAccount
type Claims struct {
	// Subject is the full subject from the JWT (system:serviceaccount:namespace:sa-name)
	Subject string
	
	// Namespace is the Kubernetes namespace
	Namespace string
	
	// ServiceAccount is the ServiceAccount name
	ServiceAccount string
}

// ParseServiceAccountIdentity parses a Kubernetes ServiceAccount subject
// Expected format: system:serviceaccount:namespace:serviceaccount-name
func ParseServiceAccountIdentity(subject string) (*Claims, error) {
	if subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	// Expected format: system:serviceaccount:namespace:sa-name
	parts := strings.Split(subject, ":")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid service account subject format: %s", subject)
	}

	if parts[0] != "system" || parts[1] != "serviceaccount" {
		return nil, fmt.Errorf("subject is not a service account: %s", subject)
	}

	namespace := parts[2]
	serviceAccount := strings.Join(parts[3:], ":") // Handle SA names with colons

	if namespace == "" || serviceAccount == "" {
		return nil, fmt.Errorf("namespace or service account name is empty")
	}

	return &Claims{
		Subject:        subject,
		Namespace:      namespace,
		ServiceAccount: serviceAccount,
	}, nil
}

// Identity returns the namespace:serviceaccount format
func (c *Claims) Identity() string {
	return fmt.Sprintf("%s:%s", c.Namespace, c.ServiceAccount)
}

// String returns a human-readable representation
func (c *Claims) String() string {
	return c.Identity()
}
