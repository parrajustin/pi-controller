package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/gorilla/websocket"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/display_server/setup"
)

var (
	wsConnectionsActive metric.Int64UpDownCounter
	wsRequestCount      metric.Int64Counter
)

func init() {
	meter := otel.Meter("display_server")
	var err error
	wsConnectionsActive, err = meter.Int64UpDownCounter("websocket.connections.active", metric.WithDescription("Active websocket connections"))
	if err != nil { panic(err) }

	wsRequestCount, err = meter.Int64Counter("websocket.request.count", metric.WithDescription("Count of websocket method calls"))
	if err != nil { panic(err) }
}

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
				slog.Info("[API]", "method", r.Method, "path", r.URL.Path, "info", "body hidden")
			} else {
				var bodyBytes []byte
				if r.Body != nil {
					bodyBytes, _ = io.ReadAll(r.Body)
					r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
				params := r.URL.Query().Encode()
				bodyStr := string(bodyBytes)
				if bodyStr != "" || params != "" {
					slog.Info("[API]", "method", r.Method, "path", r.URL.Path, "query", params, "body", bodyStr)
				} else {
					slog.Info("[API]", "method", r.Method, "path", r.URL.Path)
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
	shutdown, err := InitTelemetry(context.Background())
	if err != nil {
		log.Fatalf("failed to init telemetry: %v", err)
	}
	defer shutdown(context.Background())

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
		slog.Info("Serving directory", "dir", dir, "port", *portFlag)
		fs := http.FileServer(http.Dir(dir))
		mux.Handle("/", fs)
		handler := applyHeaders(mux)
		handler = loggingMiddleware(handler)
		otelHandler := otelhttp.NewHandler(handler, "display_server")
		if err := http.ListenAndServe(":"+*portFlag, otelHandler); err != nil {
			log.Fatal(err)
		}
	}()

	slog.Info("Starting application APIs...")

	// Register WebSocket endpoint
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("WS Upgrade error", "error", err)
			return
		}
		
		wsConnectionsActive.Add(context.Background(), 1)
		defer wsConnectionsActive.Add(context.Background(), -1)
		
		stateCtx.AddWSConn(conn)
		defer stateCtx.RemoveWSConn(conn)
		
		// Send initial state
		stateCtx.BroadcastState()

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					slog.Error("WS Read error", "error", err)
				}
				break
			}

			var req struct {
				ID          string          `json:"id"`
				Type        string          `json:"type"`
				Payload     json.RawMessage `json:"payload"`
				Traceparent string          `json:"traceparent,omitempty"`
				Tracestate  string          `json:"tracestate,omitempty"`
			}
			if err := json.Unmarshal(msgBytes, &req); err != nil {
				slog.Error("WS decode error", "error", err)
				continue
			}

			carrier := propagation.MapCarrier{
				"traceparent": req.Traceparent,
				"tracestate":  req.Tracestate,
			}
			ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
			ctx, span := otel.Tracer("display_server").Start(ctx, "websocket.process."+req.Type, trace.WithSpanKind(trace.SpanKindServer))

			clientIP := r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				clientIP = host
			}

			slog.Info("Received WS message", "type", req.Type, "id", req.ID)

			var resPayload interface{}
			var resErr error

			// Global WS handlers
			switch req.Type {
			case "get_state":
				resPayload = map[string]interface{}{
					"current_node": stateCtx.GetNodeName(),
					"meeting_code": stateCtx.MeetingCode,
					"setup_ready":  stateCtx.GetSetupReady(),
					"phase":        stateCtx.GetPhase(),
					"setup_phase":  stateCtx.GetSetupPhase(),
				}
			case "get_ip":
				ip := os.Getenv("HOST_IP")
				if ip == "" {
					ip = getLocalIP()
				}
				resPayload = map[string]string{"ip": ip}
			case "get_status":
				status := "pending"
				if stateCtx.GetSetupReady() {
					status = "ready"
				}
				resPayload = map[string]string{"status": status}
			case "calendar_events":
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
					resPayload, resErr = handler(req.Payload)
				} else {
					slog.Error("Unknown WS handler", "type", req.Type, "id", req.ID)
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
				err := stateCtx.WriteWSJSON(conn, res)
				if err != nil {
					slog.Error("Failed to write websocket response", "error", err)
				}
			}
			
			statusAttr := "success"
			if resErr != nil {
				statusAttr = "error"
			}
			wsRequestCount.Add(context.Background(), 1, metric.WithAttributes(
				attribute.String("client_ip", clientIP),
				attribute.String("type", req.Type),
				attribute.String("status", statusAttr),
			))
			span.End()
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
