package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/parrajustin/pi-controller/pkg/lounge_display/display_server/setup"
	"google.golang.org/api/calendar/v3"
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
		Mux:         mux,
		DirFlag:     dir,
		PortFlag:    *portFlag,
		EncKey:      encKey,
		OauthDir:    oauthDir,
		NodeTimeout: 10 * time.Minute,
	}

	// Start the server in a goroutine
	go func() {
		fmt.Printf("Serving directory %s on http://localhost:%s\n", dir, *portFlag)
		fs := http.FileServer(http.Dir(dir))
		mux.Handle("/", fs)
		handler := applyHeaders(mux)
		if err := http.ListenAndServe(":"+*portFlag, handler); err != nil {
			log.Fatal(err)
		}
	}()

	fmt.Println("Starting application APIs...")

	// Register the application APIs
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"current_node": stateCtx.GetNodeName(),
			"meeting_code": stateCtx.MeetingCode,
		})
	})

	mux.HandleFunc("/api/calendar_events", func(w http.ResponseWriter, r *http.Request) {
		events, err := fetchCalendarEvents(stateCtx.CalendarSrv)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)
	})

	mux.HandleFunc("/api/join_meeting", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		if payload.Code == "" {
			http.Error(w, "Meeting code is required", http.StatusBadRequest)
			return
		}

		// Tell the engine to move to NavigateToMeeting node
		stateCtx.SetNavTarget("NavigateToMeeting", map[string]interface{}{"code": payload.Code})

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})

	// Run the setup flow
	startNode := setup.InitNodes()
	setup.RunEngine(startNode, stateCtx)
}
