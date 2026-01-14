package splitsecret

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"hyena-k8s/pkg/shamir"
)

func main() {
	// Define command-line flags
	var (
		secret    = flag.String("secret", "", "The secret to split (required)")
		parts     = flag.Int("parts", 5, "Number of shares to create (N)")
		threshold = flag.Int("threshold", 3, "Minimum shares needed to reconstruct (K)")
		output    = flag.String("output", "./shares", "Output directory for share files")
		base64out = flag.Bool("base64", false, "Output shares as base64 (for K8s secrets)")
	)

	flag.Parse()

	// Validate inputs
	if *secret == "" {
		log.Fatal("Error: --secret is required")
	}

	if *parts < 2 || *parts > 255 {
		log.Fatal("Error: --parts must be between 2 and 255")
	}

	if *threshold < 2 || *threshold > 255 {
		log.Fatal("Error: --threshold must be between 2 and 255")
	}

	if *threshold > *parts {
		log.Fatal("Error: --threshold cannot be greater than --parts")
	}

	log.Printf("Splitting secret into %d shares (threshold: %d)...", *parts, *threshold)

	// Split the secret
	shares, err := shamir.Split([]byte(*secret), *parts, *threshold)
	if err != nil {
		log.Fatalf("Failed to split secret: %v", err)
	}

	log.Printf("Secret split successfully into %d shares", len(shares))

	// Create output directory
	if err := os.MkdirAll(*output, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Write shares to files
	for i, share := range shares {
		filename := filepath.Join(*output, fmt.Sprintf("share-%d.bin", i))
		
		var dataToWrite []byte
		if *base64out {
			// Encode as base64 for Kubernetes secrets
			encoded := base64.StdEncoding.EncodeToString(share)
			dataToWrite = []byte(encoded)
			filename = filepath.Join(*output, fmt.Sprintf("share-%d.b64", i))
		} else {
			dataToWrite = share
		}

		if err := os.WriteFile(filename, dataToWrite, 0644); err != nil {
			log.Fatalf("Failed to write share %d: %v", i, err)
		}

		log.Printf("Wrote share %d to %s (%d bytes)", i, filename, len(dataToWrite))
	}

	log.Printf("All shares written to %s", *output)
	log.Printf("\nTo use these shares:")
	log.Printf("  - Distribute each share to a different share server")
	log.Printf("  - Ensure at least %d shares are available for reconstruction", *threshold)
	log.Printf("  - Never store all shares in the same location")
}
