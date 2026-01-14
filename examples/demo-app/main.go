package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SecretInfo holds information about a loaded secret
type SecretInfo struct {
	Name   string `json:"name"`
	Length int    `json:"length"`
	Loaded bool   `json:"loaded"`
	Path   string `json:"path"`
}

// StatusResponse represents the JSON response for the /status endpoint
type StatusResponse struct {
	Secrets        []SecretInfo `json:"secrets"`
	TotalSecrets   int          `json:"total_secrets"`
	LoadedSecrets  int          `json:"loaded_secrets"`
	Message        string       `json:"message"`
}

var (
	secrets map[string]SecretInfo
)

func main() {
	log.Printf("Starting demo application...")

	// Initialize secrets map
	secrets = make(map[string]SecretInfo)

	// Read secrets from directory
	secretsDir := os.Getenv("SECRETS_DIR")
	if secretsDir == "" {
		secretsDir = "/secrets"
	}

	log.Printf("Loading secrets from %s...", secretsDir)
	loadSecrets(secretsDir)

	// Setup HTTP handlers
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/secrets", secretsListHandler)
	http.HandleFunc("/secret/", secretGetHandler)
	http.HandleFunc("/", rootHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Demo application listening on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  GET /health         - Health check")
	log.Printf("  GET /status         - Overall status")
	log.Printf("  GET /secrets        - List all secrets")
	log.Printf("  GET /secret/<name>  - Get specific secret info")
	log.Printf("  GET /               - Welcome message")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// loadSecrets scans the secrets directory and loads secret metadata
func loadSecrets(dir string) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get secret name from filename
		secretName := filepath.Base(path)
		
		// Skip hidden files
		if strings.HasPrefix(secretName, ".") {
			return nil
		}

		// Read secret file
		data, err := os.ReadFile(path)
		info := SecretInfo{
			Name: secretName,
			Path: path,
		}

		if err != nil {
			log.Printf("ERROR: Failed to read secret '%s' from %s: %v", secretName, path, err)
			info.Loaded = false
			info.Length = 0
		} else {
			info.Loaded = true
			info.Length = len(data)
			log.Printf("✓ Secret '%s' loaded successfully (length: %d bytes)", secretName, len(data))
			// NEVER log the actual secret value
		}

		secrets[secretName] = info
		return nil
	})

	if err != nil {
		log.Printf("ERROR: Failed to scan secrets directory: %v", err)
	}

	log.Printf("Loaded %d/%d secrets successfully", countLoadedSecrets(), len(secrets))
}

// countLoadedSecrets returns the number of successfully loaded secrets
func countLoadedSecrets() int {
	count := 0
	for _, info := range secrets {
		if info.Loaded {
			count++
		}
	}
	return count
}

// healthHandler returns 200 if the application is running
// Returns 503 if no secrets are loaded
func healthHandler(w http.ResponseWriter, r *http.Request) {
	loadedCount := countLoadedSecrets()
	
	if loadedCount == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "Unhealthy: No secrets loaded\n")
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Healthy: %d secret(s) loaded\n", loadedCount)
}

// statusHandler returns JSON with overall status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	secretsList := make([]SecretInfo, 0, len(secrets))
	for _, info := range secrets {
		secretsList = append(secretsList, info)
	}

	loadedCount := countLoadedSecrets()
	
	response := StatusResponse{
		Secrets:       secretsList,
		TotalSecrets:  len(secrets),
		LoadedSecrets: loadedCount,
	}

	if loadedCount == len(secrets) && len(secrets) > 0 {
		response.Message = fmt.Sprintf("All %d secret(s) loaded successfully", loadedCount)
	} else if loadedCount > 0 {
		response.Message = fmt.Sprintf("%d/%d secret(s) loaded", loadedCount, len(secrets))
	} else {
		response.Message = "No secrets loaded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// secretsListHandler returns a list of all secrets (names only)
func secretsListHandler(w http.ResponseWriter, r *http.Request) {
	secretNames := make([]string, 0, len(secrets))
	for name := range secrets {
		secretNames = append(secretNames, name)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"secrets": secretNames,
		"count":   len(secretNames),
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// secretGetHandler returns information about a specific secret
func secretGetHandler(w http.ResponseWriter, r *http.Request) {
	// Extract secret name from path
	secretName := strings.TrimPrefix(r.URL.Path, "/secret/")
	if secretName == "" {
		http.Error(w, "Secret name required", http.StatusBadRequest)
		return
	}

	info, exists := secrets[secretName]
	if !exists {
		http.Error(w, fmt.Sprintf("Secret '%s' not found", secretName), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// rootHandler returns a welcome message
func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>Hyena-K8s Demo App</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .status { padding: 15px; margin: 20px 0; border-radius: 5px; }
        .success { background-color: #d4edda; border: 1px solid #c3e6cb; color: #155724; }
        .error { background-color: #f8d7da; border: 1px solid #f5c6cb; color: #721c24; }
        .info { background-color: #d1ecf1; border: 1px solid #bee5eb; color: #0c5460; }
        code { background-color: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>🐆 Hyena-K8s Demo Application</h1>
    <div class="info">
        <p><strong>Decentralized Runtime Secret Management</strong></p>
        <p>This application demonstrates Shamir's Secret Sharing in Kubernetes.</p>
    </div>
    `)

	loadedCount := countLoadedSecrets()
	
	if loadedCount > 0 {
		fmt.Fprintf(w, `
    <div class="status success">
        <h2>✓ Secrets Status: %d/%d LOADED</h2>
        <p>Secrets successfully reconstructed from distributed shares.</p>
        <ul>
`, loadedCount, len(secrets))
		
		for name, info := range secrets {
			if info.Loaded {
				fmt.Fprintf(w, `        <li><strong>%s</strong>: %d bytes</li>
`, name, info.Length)
			}
		}
		
		fmt.Fprintf(w, `        </ul>
        <p><em>Note: Actual secret values are never displayed or logged.</em></p>
    </div>
`)
	} else {
		fmt.Fprintf(w, `
    <div class="status error">
        <h2>✗ Secrets Status: NOT LOADED</h2>
        <p>Failed to load secrets. The init container may have failed.</p>
    </div>
`)
	}

	fmt.Fprintf(w, `
    <h2>API Endpoints</h2>
    <ul>
        <li><code>GET /health</code> - Returns 200 if healthy, 503 if no secrets loaded</li>
        <li><code>GET /status</code> - Returns JSON with all secrets status</li>
        <li><code>GET /secrets</code> - Returns list of secret names</li>
        <li><code>GET /secret/&lt;name&gt;</code> - Returns info about specific secret</li>
    </ul>
    
    <h2>Architecture</h2>
    <p>This demonstrates a PoC system where:</p>
    <ol>
        <li>A secret is split into N shares using Shamir's Secret Sharing</li>
        <li>Each share is stored on a separate server</li>
        <li>At runtime, an init container fetches K shares</li>
        <li>The secret is reconstructed in memory (never written to disk)</li>
        <li>The application reads the secret from a tmpfs volume</li>
    </ol>
    
    <p><strong>⚠️ This is a proof-of-concept, not production-ready.</strong></p>
</body>
</html>
`)
}
