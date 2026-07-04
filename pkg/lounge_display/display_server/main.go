package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/parrajustin/pi-controller/pkg/lounge_display/cryptoutil"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
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

type EventInfo struct {
	Name        string `json:"name"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Accepted    string `json:"acceptedStatus"`
	Description string `json:"description"`
	MeetLink    string `json:"meetLink"`
}

func fetchCalendarEvents(srv *calendar.Service) ([]EventInfo, error) {
	tMin := time.Now().Add(-15 * time.Minute).Format(time.RFC3339)
	tMax := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(tMin).TimeMax(tMax).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		return nil, err
	}

	var results []EventInfo
	for _, item := range events.Items {
		startDate := item.Start.DateTime
		if startDate == "" {
			startDate = item.Start.Date
		}
		endDate := ""
		if item.End != nil {
			endDate = item.End.DateTime
			if endDate == "" {
				endDate = item.End.Date
			}
		}

		acceptedStatus := "unknown"
		for _, attendee := range item.Attendees {
			if attendee.Self {
				acceptedStatus = attendee.ResponseStatus
				break
			}
		}
		if len(item.Attendees) == 0 {
			acceptedStatus = "accepted"
		}

		if acceptedStatus == "declined" {
			continue
		}

		results = append(results, EventInfo{
			Name:        item.Summary,
			StartTime:   startDate,
			EndTime:     endDate,
			Accepted:    acceptedStatus,
			Description: item.Description,
			MeetLink:    item.HangoutLink,
		})
	}
	return results, nil
}

func main() {
	dirFlag := flag.String("dir", ".", "the directory to serve")
	portFlag := flag.String("port", "8080", "the port to listen on")
	flag.Parse()

	oauthDir := os.Getenv("OAUTH_DIR")
	if oauthDir == "" {
		oauthDir = "."
	}
	tok, err := loadAuth(oauthDir)
	if err != nil {
		log.Fatalf("Failed to load auth token: %v", err)
	}

	fmt.Printf("Successfully decrypted auth token for display server usage (valid=%v)\n", tok.Valid())

	credPath := filepath.Join(oauthDir, "credentials.json")
	credBytes, err := os.ReadFile(credPath)
	if err != nil {
		log.Fatalf("Unable to read credentials file: %v", err)
	}
	config, err := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	ctx := context.Background()
	client := config.Client(ctx, tok)
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	// Fail if we can't fetch test calendar events
	_, err = fetchCalendarEvents(srv)
	if err != nil {
		log.Fatalf("Failed to fetch initial calendar events: %v", err)
	}

	dir, err := filepath.Abs(*dirFlag)
	if err != nil {
		log.Fatalf("Invalid directory: %v", err)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Fatalf("Directory does not exist: %v", dir)
	}

	fmt.Printf("Serving directory %s on http://localhost:%s\n", dir, *portFlag)

	mux := http.NewServeMux()
	
	mux.HandleFunc("/api/calendar_events", func(w http.ResponseWriter, r *http.Request) {
		events, err := fetchCalendarEvents(srv)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	})

	// Create a file server for the specified directory
	fs := http.FileServer(http.Dir(dir))
	mux.Handle("/", fs)

	// Apply our middleware
	handler := applyHeaders(mux)

	if err := http.ListenAndServe(":"+*portFlag, handler); err != nil {
		log.Fatal(err)
	}
}
