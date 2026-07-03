package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// applyHeaders adds headers to fix CORS and CSP (Content Security Policy) issues.
func applyHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		h.ServeHTTP(w, r)
	})
}

// getLocalIP returns the local IPv4 address of the host machine.
func getLocalIP() string {
	if hostIP := os.Getenv("HOST_IP"); hostIP != "" {
		return hostIP
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// Channels and shared state for OAuth orchestration
var credChan = make(chan []byte)
var authCodeChan = make(chan string)
var authURLStr string
var setupReady bool

type tokenPayload struct {
	Code string `json:"code"`
}

// runReceiver spawns the receiver binary and returns the extracted ticket.
// It also starts a goroutine to read the received code and send it to authCodeChan.
func runReceiver(binPath string) (string, error) {
	cmd := exec.Command(binPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(stdout)
	var ticket string

	// Read until we find the ticket
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println("[receiver]", line)
		if strings.HasPrefix(line, "Ticket: ") {
			ticket = strings.TrimSpace(strings.TrimPrefix(line, "Ticket: "))
			break
		}
	}

	if ticket == "" {
		return "", fmt.Errorf("failed to extract ticket from receiver")
	}

	// Continue reading for the code in a goroutine
	go func() {
		defer cmd.Wait()
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("[receiver]", line)
			if strings.HasPrefix(line, "RECEIVED CODE: ") {
				code := strings.TrimSpace(strings.TrimPrefix(line, "RECEIVED CODE: "))
				authCodeChan <- code
				return // we can stop reading
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("[receiver] error reading stdout:", err)
		}
	}()

	if err := scanner.Err(); err != nil {
		return ticket, fmt.Errorf("error reading receiver stdout: %v", err)
	}

	return ticket, nil
}

func startHTTPServer(dir, port string) {
	localIP := getLocalIP()
	fmt.Printf("Local IP Address: %s\n", localIP)
	fmt.Printf("Serving directory %s on http://localhost:%s\n", dir, port)

	mux := http.NewServeMux()

	// Serve the static directory
	fs := http.FileServer(http.Dir(dir))
	mux.Handle("/", fs)

	// /api/ip endpoint
	mux.HandleFunc("/api/ip", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ip": localIP})
	})

	// /api/status endpoint
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		status := "pending"
		if setupReady {
			status = "ready"
		}
		json.NewEncoder(w).Encode(map[string]string{"status": status})
	})

	// /api/has_wifi endpoint
	mux.HandleFunc("/api/has_wifi", func(w http.ResponseWriter, r *http.Request) {
		client := http.Client{Timeout: 3 * time.Second}
		_, err := client.Get("https://google.com")
		access := err == nil
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"internetAccess": access})
	})

	// /api/has_cred endpoint
	mux.HandleFunc("/api/has_cred", func(w http.ResponseWriter, r *http.Request) {
		oauthDir := os.Getenv("OAUTH_DIR")
		if oauthDir == "" {
			oauthDir = "."
		}
		credPath := filepath.Join(oauthDir, "credentials.json")
		hasCred := false
		if _, err := os.Stat(credPath); err == nil {
			hasCred = true
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"hasCreds": hasCred})
	})

	// /api/has_auth endpoint
	mux.HandleFunc("/api/has_auth", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		oauthDir := os.Getenv("OAUTH_DIR")
		if oauthDir == "" {
			oauthDir = "."
		}
		tokPath := filepath.Join(oauthDir, "token.json")
		hasAuth := false

		if f, err := os.Open(tokPath); err == nil {
			defer f.Close()
			tok := &oauth2.Token{}
			if err := json.NewDecoder(f).Decode(tok); err == nil && tok.AccessToken != "" {
				hasAuth = true
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"hasAuth": hasAuth})
	})

	// /api/cred endpoint
	mux.HandleFunc("/api/cred", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		credChan <- body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	// /api/auth_url endpoint
	mux.HandleFunc("/api/auth_url", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": authURLStr})
	})

	// /api/token endpoint
	mux.HandleFunc("/api/token", func(w http.ResponseWriter, r *http.Request) {
		code := ""
		if r.Method == http.MethodGet {
			// Support Google direct redirect back to this endpoint
			code = r.URL.Query().Get("code")
		} else if r.Method == http.MethodPost {
			// Support frontend POSTing the code/token
			var payload tokenPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			code = payload.Code
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if code == "" {
			http.Error(w, "Code is required", http.StatusBadRequest)
			return
		}

		// Pass the code down the channel and return
		authCodeChan <- code
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	// Apply CORS and CSP headers
	handler := applyHeaders(mux)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

func main() {
	dirFlag := flag.String("dir", "startup", "the directory to serve")
	portFlag := flag.String("port", "8080", "the port to listen on")
	receiverFlag := flag.String("receiver", "/app/receiver", "the path to the receiver binary")
	flag.Parse()

	dir, err := filepath.Abs(*dirFlag)
	if err != nil {
		log.Fatalf("Invalid directory: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}

	// 1. Run HTTP Server in a goroutine
	go startHTTPServer(dir, *portFlag)

	// 2. Determine OAUTH_DIR
	oauthDir := os.Getenv("OAUTH_DIR")
	if oauthDir == "" {
		oauthDir = "."
	}

	credPath := filepath.Join(oauthDir, "credentials.json")
	tokPath := filepath.Join(oauthDir, "token.json")

	// 3. Credentials Phase
	var credBytes []byte
	var isNew bool
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		fmt.Println("Waiting for credentials on POST /api/cred...")
		credBytes = <-credChan
		isNew = true
	} else {
		credBytes, err = os.ReadFile(credPath)
		if err != nil {
			log.Fatalf("Unable to read credentials file: %v", err)
		}
	}

	// 4. Token Phase
	config, err := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope)
	if err != nil {
		os.Remove(credPath)
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	if isNew {
		err := os.WriteFile(credPath, credBytes, 0600)
		if err != nil {
			log.Fatalf("Unable to save credentials.json: %v", err)
		}
		fmt.Println("Saved credentials.json")
	}
	// Dynamically set the redirect URI
	baseURL := os.Getenv("OAUTH_REDIRECT_URL")
	if baseURL == "" {
		log.Fatalf("Env OAUTH_REDIRECT_URL is required.")
	}

	ticket, err := runReceiver(*receiverFlag)
	if err != nil {
		log.Fatalf("Failed to run receiver: %v", err)
	}

	config.RedirectURL = fmt.Sprintf("%s/auth/%s", strings.TrimRight(baseURL, "/"), ticket)
	ctx := context.Background()
	var tok *oauth2.Token

	// Check if token.json exists
	if f, err := os.Open(tokPath); err == nil {
		defer f.Close()
		tok = &oauth2.Token{}
		if err := json.NewDecoder(f).Decode(tok); err != nil {
			tok = nil
		}
	}

	if tok == nil {
		// Ask for an auth code dynamically via API
		authURLStr = config.AuthCodeURL("state-token",
			oauth2.AccessTypeOffline,
			oauth2.SetAuthURLParam("device_id", "lounge-display"),
			oauth2.SetAuthURLParam("device_name", "Lounge Display"),
		)
		fmt.Printf("Authorization URL generated: %s\nWaiting for auth code on /api/token or via receiver...\n", authURLStr)

		authCode := <-authCodeChan
		tok, err = config.Exchange(ctx, authCode)
		if err != nil {
			log.Fatalf("Unable to retrieve token from web: %v", err)
		}

		// Save token
		f, err := os.OpenFile(tokPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatalf("Unable to cache oauth token: %v", err)
		}
		defer f.Close()
		json.NewEncoder(f).Encode(tok)
		fmt.Println("Saved token.json")
	}

	// 5. Final Logic (Calendar API)
	client := config.Client(ctx, tok)
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	t := time.Now().Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
	}
	fmt.Println("Upcoming events:")
	if len(events.Items) == 0 {
		fmt.Println("No upcoming events found.")
	} else {
		for _, item := range events.Items {
			date := item.Start.DateTime
			if date == "" {
				date = item.Start.Date
			}
			fmt.Printf("%v (%v)\n", item.Summary, date)
		}
	}

	fmt.Println("Setup logic complete! The server will exit in 3 seconds to hand over to display_server.")
	setupReady = true
	time.Sleep(3 * time.Second)
}
