package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/parrajustin/pi-controller/pkg/lounge_display/cryptoutil"
	"golang.org/x/oauth2"
)

// applyHeaders adds headers to fix CORS and CSP (Content Security Policy) issues.
// This resolves common "CSF" (CORS/CSP) errors when serving local HTML data.
func applyHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow any origin for CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Disable caching so local changes are immediately visible
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		h.ServeHTTP(w, r)
	})
}

// loadAuth loads and decrypts the OAuth token from token.json.enc
func loadAuth(oauthDir string) (*oauth2.Token, error) {
	encKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if encKey == "" {
		return nil, fmt.Errorf("TOKEN_ENCRYPTION_KEY is required")
	}

	tokPath := filepath.Join(oauthDir, "token.json.enc")
	ciphertext, err := os.ReadFile(tokPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token: %w", err)
	}

	plaintext, err := cryptoutil.Decrypt(ciphertext, encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt token: %w", err)
	}

	tok := &oauth2.Token{}
	if err := json.Unmarshal(plaintext, tok); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return tok, nil
}

func main() {
	dirFlag := flag.String("dir", ".", "the directory to serve")
	portFlag := flag.String("port", "8080", "the port to listen on")
	flag.Parse()

	// Example usage of token decryption
	oauthDir := os.Getenv("OAUTH_DIR")
	if oauthDir == "" {
		oauthDir = "."
	}
	tok, err := loadAuth(oauthDir)
	if err != nil {
		fmt.Printf("Warning: failed to load auth token: %v\n", err)
	} else {
		fmt.Printf("Successfully decrypted auth token for display server usage (valid=%v)\n", tok.Valid())
	}

	dir, err := filepath.Abs(*dirFlag)
	if err != nil {
		log.Fatalf("Invalid directory: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Fatalf("Directory does not exist: %v", dir)
	}

	fmt.Printf("Serving directory %s on http://localhost:%s\n", dir, *portFlag)

	// Create a file server for the specified directory
	fs := http.FileServer(http.Dir(dir))

	// Apply our middleware
	handler := applyHeaders(fs)

	if err := http.ListenAndServe(":"+*portFlag, handler); err != nil {
		log.Fatal(err)
	}
}
