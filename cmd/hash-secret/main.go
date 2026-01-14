package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: hash-secret <secret-string>")
		fmt.Println("Calculates SHA256 hash of the input string for verification")
		os.Exit(1)
	}

	secret := os.Args[1]
	hash := sha256.Sum256([]byte(secret))
	hashStr := hex.EncodeToString(hash[:])

	fmt.Printf("Secret: %s\n", secret)
	fmt.Printf("Length: %d bytes\n", len(secret))
	fmt.Printf("SHA256: %s\n", hashStr)
	
	if len(secret) > 20 {
		fmt.Printf("First 20 chars: %s...\n", secret[:20])
	} else {
		fmt.Printf("First 20 chars: %s\n", secret)
	}
}
