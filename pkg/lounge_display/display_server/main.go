package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/display_server/setup"
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

// loggingMiddleware logs incoming API requests and their parameters, masking sensitive endpoints
func loggingMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
			if r.URL.Path == "/api/password" || strings.Contains(r.URL.Path, "password") {
				log.Printf("[API] %s %s (body hidden)\n", r.Method, r.URL.Path)
			} else {
				var bodyBytes []byte
				if r.Body != nil {
					bodyBytes, _ = io.ReadAll(r.Body)
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
				params := r.URL.Query().Encode()
				bodyStr := string(bodyBytes)
				if bodyStr != "" || params != "" {
					log.Printf("[API] %s %s | query: %s | body: %s\n", r.Method, r.URL.Path, params, bodyStr)
				} else {
					log.Printf("[API] %s %s\n", r.Method, r.URL.Path)
				}
			}
		}
		h.ServeHTTP(w, r)
	})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local display
	},
}



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

func main() {
	dirFlag := flag.String("dir", ".", "the directory to serve")
	portFlag := flag.String("port", "8080", "the port to listen on")
	receiverFlag := flag.String("receiver", "", "path to oauth receiver binary (optional)")
	emailFlag := flag.String("email", "lounge.room@mountainviewmasoniclodge.com", "Google email address")
	flag.Parse()

	encKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if encKey == "" {
		log.Fatalf("TOKEN_ENCRYPTION_KEY is required")
	}

	oauthDir := os.Getenv("OAUTH_DIR")
	if oauthDir == "" {
		oauthDir = "."
	}

	dir, err := filepath.Abs(*dirFlag)
	if err != nil {
		log.Fatalf("Invalid directory: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Fatalf("Directory does not exist: %v", dir)
	}

	mux := http.NewServeMux()

	stateCtx := &setup.StateContext{
		Mux:              mux,
		DirFlag:          dir,
		PortFlag:         *portFlag,
		EncKey:           encKey,
		OauthDir:         oauthDir,
		NodeTimeout:      10 * time.Minute,
		PasswordChan:     make(chan string, 1),
		Email:            *emailFlag,
		ReceiverFlag:     *receiverFlag,
		RegisteredRoutes: make(map[string]bool),
	}

	// Start the server in a goroutine
	go func() {
		fmt.Printf("Serving directory %s on http://localhost:%s\n", dir, *portFlag)
		fs := http.FileServer(http.Dir(dir))
		mux.Handle("/", fs)
		handler := applyHeaders(mux)
		handler = loggingMiddleware(handler)
		if err := http.ListenAndServe(":"+*portFlag, handler); err != nil {
			log.Fatal(err)
		}
	}()

	fmt.Println("Starting application APIs...")

	// Register WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WS Upgrade error: %v", err)
			return
		}
		
		stateCtx.AddWSConn(conn)
		defer stateCtx.RemoveWSConn(conn)
		
		// Send initial state
		stateCtx.BroadcastState()

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WS Read error: %v", err)
				}
				break
			}

			var req struct {
				ID      string          `json:"id"`
				Type    string          `json:"type"`
				Payload json.RawMessage `json:"payload"`
			}
			if err := json.Unmarshal(msgBytes, &req); err != nil {
				log.Printf("WS decode error: %v", err)
				continue
			}

			var resPayload interface{}
			var resErr error

			// Global WS handlers
			switch req.Type {
			case "get_state":
				log.Printf("[WS] Received message %q (ID: %s) -> Routing to Global handler", req.Type, req.ID)
				resPayload = map[string]interface{}{
					"current_node": stateCtx.GetNodeName(),
					"meeting_code": stateCtx.MeetingCode,
					"setup_ready":  stateCtx.GetSetupReady(),
					"phase":        stateCtx.GetPhase(),
					"setup_phase":  stateCtx.GetSetupPhase(),
				}
			case "get_ip":
				log.Printf("[WS] Received message %q (ID: %s) -> Routing to Global handler", req.Type, req.ID)
				ip := os.Getenv("HOST_IP")
				if ip == "" {
					ip = getLocalIP()
				}
				resPayload = map[string]string{"ip": ip}
			case "get_status":
				log.Printf("[WS] Received message %q (ID: %s) -> Routing to Global handler", req.Type, req.ID)
				status := "pending"
				if stateCtx.GetSetupReady() {
					status = "ready"
				}
				resPayload = map[string]string{"status": status}
			case "calendar_events":
				log.Printf("[WS] Received message %q (ID: %s) -> Routing to Global handler", req.Type, req.ID)
				if stateCtx.CalendarClient != nil {
					events, err := stateCtx.CalendarClient.FetchEvents()
					if err != nil {
						resErr = err
					} else {
						resPayload = events
					}
				} else {
					resErr = fmt.Errorf("calendar client not initialized")
				}
			default:
				// Node-specific WS handlers
				handler, ok := stateCtx.GetWSHandler(req.Type)

				if ok {
					log.Printf("[WS] Received message %q (ID: %s) -> Routing to Node handler", req.Type, req.ID)
					resPayload, resErr = handler(req.Payload)
				} else {
					log.Printf("[WS] Received message %q (ID: %s) -> ERROR: Unknown handler", req.Type, req.ID)
					resErr = fmt.Errorf("unknown message type: %s", req.Type)
				}
			}

			// Send response if this request expects one (has an ID)
			if req.ID != "" {
				res := map[string]interface{}{
					"id":   req.ID,
					"type": "response",
				}
				if resErr != nil {
					res["error"] = resErr.Error()
				} else {
					res["payload"] = resPayload
				}
				conn.WriteJSON(res)
			}
		}
	})

	mux.HandleFunc("/auth/finalize", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true}`))
		stateCtx.SetSetupReady(true)
	})

	mux.HandleFunc("/api/setup_done", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"setup_ready": stateCtx.GetSetupReady()})
	})

	// Run the setup flow
	startNode := setup.InitNodes()
	setup.RunEngine(startNode, stateCtx)
}
