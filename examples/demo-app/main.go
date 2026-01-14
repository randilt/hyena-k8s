package demoapp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

// StatusResponse represents the JSON response for the /status endpoint
type StatusResponse struct {
	SecretLoaded bool   `json:"secret_loaded"`
	SecretLength int    `json:"secret_length"`
	Message      string `json:"message"`
}

var (
	secretData   []byte
	secretLoaded bool
)

func main() {
	log.Printf("Starting demo application...")

	// Read secret from file
	secretPath := os.Getenv("SECRET_PATH")
	if secretPath == "" {
		secretPath = "/secrets/app-secret"
	}

	log.Printf("Reading secret from %s...", secretPath)
	data, err := os.ReadFile(secretPath)
	if err != nil {
		log.Printf("ERROR: Failed to read secret: %v", err)
		log.Printf("Application starting in degraded mode (no secret)")
		secretLoaded = false
	} else {
		secretData = data
		secretLoaded = true
		log.Printf("✓ Secret loaded successfully (length: %d bytes)", len(secretData))
		// NEVER log the actual secret value
	}

	// Setup HTTP handlers
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/", rootHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Demo application listening on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  GET /health  - Health check")
	log.Printf("  GET /status  - Status and secret info")
	log.Printf("  GET /        - Welcome message")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// healthHandler returns 200 if the application is running
// Returns 503 if secret is not loaded
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if !secretLoaded {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "Unhealthy: Secret not loaded\n")
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Healthy\n")
}

// statusHandler returns JSON with secret status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	response := StatusResponse{
		SecretLoaded: secretLoaded,
		SecretLength: len(secretData),
	}

	if secretLoaded {
		response.Message = "Secret loaded and available"
	} else {
		response.Message = "Secret not loaded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
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

	if secretLoaded {
		fmt.Fprintf(w, `
    <div class="status success">
        <h2>✓ Secret Status: LOADED</h2>
        <p>Secret successfully reconstructed from distributed shares.</p>
        <p>Secret length: <strong>%d bytes</strong></p>
        <p><em>Note: The actual secret value is never displayed or logged.</em></p>
    </div>
`, len(secretData))
	} else {
		fmt.Fprintf(w, `
    <div class="status error">
        <h2>✗ Secret Status: NOT LOADED</h2>
        <p>Failed to load secret. The init container may have failed.</p>
    </div>
`)
	}

	fmt.Fprintf(w, `
    <h2>API Endpoints</h2>
    <ul>
        <li><code>GET /health</code> - Returns 200 if healthy, 503 if secret not loaded</li>
        <li><code>GET /status</code> - Returns JSON with secret status</li>
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
