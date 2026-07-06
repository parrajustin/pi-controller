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

	"github.com/chromedp/chromedp"
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

	mux.HandleFunc("/api/meeting/button_state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		nodeName := stateCtx.GetNodeName()
		if nodeName != "In Meeting" || stateCtx.TargetCtx == nil {
			log.Printf("[/api/meeting/button_state] Not in meeting (Node: %s, TargetCtx exists: %v)\n", nodeName, stateCtx.TargetCtx != nil)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"in_meeting": false,
				"microphone": false,
				"camera":     false,
				"hand":       false,
			})
			return
		}

		var stateJSON string
		err := chromedp.Run(stateCtx.TargetCtx, chromedp.Evaluate(`
			(function() {
				let micBtn = document.querySelector('button[aria-label*="microphone" i]');
				let camBtn = document.querySelector('button[aria-label*="camera" i]');
				let handBtn = document.querySelector('button[aria-label*="hand" i]');
				
				let micOn = micBtn ? micBtn.getAttribute('aria-label').toLowerCase().includes('turn off') : false;
				let camOn = camBtn ? camBtn.getAttribute('aria-label').toLowerCase().includes('turn off') : false;
				let handRaised = handBtn ? handBtn.getAttribute('aria-label').toLowerCase().includes('lower') : false;

				return JSON.stringify({
					in_meeting: true,
					microphone: micOn,
					camera: camOn,
					hand: handRaised
				});
			})();
		`, &stateJSON))

		if err != nil {
			log.Printf("[/api/meeting/button_state] Error evaluating state: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.Write([]byte(stateJSON))
	})

	mux.HandleFunc("/api/meeting/click_button", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		nodeName := stateCtx.GetNodeName()
		if nodeName != "In Meeting" || stateCtx.TargetCtx == nil {
			log.Printf("[/api/meeting/click_button] Not in meeting (Node: %s, TargetCtx exists: %v)\n", nodeName, stateCtx.TargetCtx != nil)
			http.Error(w, "Not in meeting", http.StatusBadRequest)
			return
		}

		var payload struct {
			Button string `json:"button"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			log.Printf("[/api/meeting/click_button] Invalid JSON payload: %v\n", err)
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		var query string
		switch payload.Button {
		case "microphone":
			query = `button[aria-label*="microphone" i]`
		case "camera":
			query = `button[aria-label*="camera" i]`
		case "hand":
			query = `button[aria-label*="hand" i]`
		case "hangup":
			log.Printf("[/api/meeting/click_button] Triggering LeaveMeeting node\n")
			stateCtx.SetNavTarget("LeaveMeeting", nil)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"clicked": true})
			return
		default:
			log.Printf("[/api/meeting/click_button] Unknown button requested: %s\n", payload.Button)
			http.Error(w, "Unknown button", http.StatusBadRequest)
			return
		}

		log.Printf("[/api/meeting/click_button] Attempting to click %s using query: %s\n", payload.Button, query)

		var clicked bool
		err := chromedp.Run(stateCtx.TargetCtx, chromedp.Evaluate(fmt.Sprintf(`
			(function() {
				let btn = document.querySelector('%s');
				if (btn) {
					btn.click();
					return true;
				}
				return false;
			})();
		`, query), &clicked))

		if err != nil {
			log.Printf("[/api/meeting/click_button] Error executing click script: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("[/api/meeting/click_button] Click result for %s: %v\n", payload.Button, clicked)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"clicked": clicked})
	})

	// Run the setup flow
	startNode := setup.InitNodes()
	setup.RunEngine(startNode, stateCtx)
}
