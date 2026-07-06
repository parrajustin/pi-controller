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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/parrajustin/pi-controller/pkg/lounge_display/cryptoutil"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

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

// Orchestration channels
var credChan = make(chan []byte)
var authCodeChan = make(chan string)
var authURLStr string
var hasToken bool
var hasCalendar bool

type tokenPayload struct {
	Code string `json:"code"`
}

func runReceiver(binPath string) (string, *exec.Cmd, error) {
	cmd := exec.Command(binPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, err
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", nil, err
	}

	scanner := bufio.NewScanner(stdout)
	var ticket string

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println("[receiver]", line)
		if strings.HasPrefix(line, "Ticket: ") {
			ticket = strings.TrimSpace(strings.TrimPrefix(line, "Ticket: "))
			break
		}
	}

	if ticket == "" {
		return "", nil, fmt.Errorf("failed to extract ticket from receiver")
	}

	go func() {
		defer cmd.Wait()
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("[receiver]", line)
			if strings.HasPrefix(line, "RECEIVED CODE: ") {
				code := strings.TrimSpace(strings.TrimPrefix(line, "RECEIVED CODE: "))
				authCodeChan <- code
				return
			}
		}
	}()

	return ticket, cmd, nil
}

// --- StateContext & Nodes ---

type StateContext struct {
	mu          sync.Mutex
	CurrentNode string

	Ctx         context.Context
	TargetCtx   context.Context
	Email       string
	StepCounter int
	LogsDir     string

	Mux              *http.ServeMux
	RegisteredRoutes map[string]bool
	PasswordChan     chan string

	DirFlag      string
	PortFlag     string
	ReceiverFlag string
	EncKey       string
	OauthDir     string
	NodeTimeout  time.Duration
}

func (s *StateContext) SetNodeName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentNode = name
}
func (s *StateContext) GetNodeName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.CurrentNode
}

func registerRoute(s *StateContext, path string, handler http.HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.RegisteredRoutes == nil {
		s.RegisteredRoutes = make(map[string]bool)
	}
	if !s.RegisteredRoutes[path] {
		s.Mux.HandleFunc(path, handler)
		s.RegisteredRoutes[path] = true
	}
}

type Node struct {
	Name       string
	IsRestNode bool
	Setup      func(s *StateContext) error
	PreCheck   func(s *StateContext) bool
	Work       func(s *StateContext) error
	DoneCheck  func(s *StateContext) error
	Teardown   func(s *StateContext) error
	Next       []*Node
}

var (
	InitServerNode  *Node
	CredentialsNode *Node
	AuthTokenNode   *Node
	CalendarNode    *Node
	InitCDPNode     *Node
	FinalizeNode    *Node

	StartMeetNode            *Node
	FinishedMeetNode         *Node
	WorkspaceRedirectedNode  *Node
	AccountsGooglePageNode   *Node
	ChooseAccountNode        *Node
	AccountOptionExistsNode  *Node
	AccountOptionMissingNode *Node
	EmailInputNode           *Node
	PasswordInputNode        *Node
	WrongPasswordNode        *Node
	TwoFactorNode            *Node
)

func captureDebugArtifacts(s *StateContext, stepName string, phase string, prefix string) {
	if s.TargetCtx == nil {
		return
	}
	fmt.Printf("Capturing artifacts for step %04d: %s (%s)...\n", s.StepCounter, stepName, phase)
	var html string
	var screenshotBuf []byte

	captureCtx, cancel := context.WithTimeout(s.TargetCtx, 5*time.Second)
	defer cancel()

	err := chromedp.Run(captureCtx,
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
		chromedp.CaptureScreenshot(&screenshotBuf),
	)
	if err != nil {
		log.Printf("Warning: Failed to capture artifacts for %s: %v\n", stepName, err)
		return
	}

	if err := os.MkdirAll(s.LogsDir, 0755); err != nil {
		log.Printf("Warning: Failed to create logs dir: %v\n", err)
	}

	htmlFile := fmt.Sprintf("%s/%04d_%s%s_%s_dump.html", s.LogsDir, s.StepCounter, prefix, stepName, phase)
	imgFile := fmt.Sprintf("%s/%04d_%s%s_%s_screenshot.png", s.LogsDir, s.StepCounter, prefix, stepName, phase)

	os.WriteFile(htmlFile, []byte(html), 0644)
	os.WriteFile(imgFile, screenshotBuf, 0644)
	fmt.Printf("Saved %s and %s\n", htmlFile, imgFile)
}

func initNodes() {
	// --- Server Setup Nodes ---
	InitServerNode = &Node{
		Name:     "Init Server",
		PreCheck: func(s *StateContext) bool { return true },
		Setup: func(s *StateContext) error {
			if s.Mux != nil {
				return nil
			}
			s.Mux = http.NewServeMux()

			fs := http.FileServer(http.Dir(s.DirFlag))
			s.Mux.Handle("/", fs)

			registerRoute(s, "/api/ip", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"ip": getLocalIP()})
			})
			registerRoute(s, "/api/status", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				status := "pending"
				if s.GetNodeName() == "Finalize Setup" {
					status = "ready"
				}
				json.NewEncoder(w).Encode(map[string]string{"status": status})
			})
			registerRoute(s, "/api/state", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"current_node": s.GetNodeName()})
			})
			registerRoute(s, "/api/has_wifi", func(w http.ResponseWriter, r *http.Request) {
				client := http.Client{Timeout: 3 * time.Second}
				_, err := client.Get("https://google.com")
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]bool{"internetAccess": err == nil})
			})

			go func() {
				handler := applyHeaders(s.Mux)
				fmt.Printf("Serving directory %s on http://localhost:%s\n", s.DirFlag, s.PortFlag)
				if err := http.ListenAndServe(":"+s.PortFlag, handler); err != nil {
					log.Printf("HTTP Server error: %v\n", err)
				}
			}()
			return nil
		},
	}

	CredentialsNode = &Node{
		Name: "Credentials Phase",
		IsRestNode: true,
		Setup: func(s *StateContext) error {
			registerRoute(s, "/api/has_cred", func(w http.ResponseWriter, r *http.Request) {
				credPath := filepath.Join(s.OauthDir, "credentials.json")
				_, err := os.Stat(credPath)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]bool{"hasCreds": err == nil})
			})
			registerRoute(s, "/api/cred", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					return
				}
				body, _ := io.ReadAll(r.Body)
				credChan <- body
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status": "ok"}`))
			})
			return nil
		},
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			credPath := filepath.Join(s.OauthDir, "credentials.json")
			if _, err := os.Stat(credPath); os.IsNotExist(err) {
				fmt.Println("Waiting for credentials on POST /api/cred...")
				credBytes := <-credChan
				err := os.WriteFile(credPath, credBytes, 0600)
				if err != nil {
					return err
				}
				fmt.Println("Saved credentials.json")
			}
			return nil
		},
	}

	AuthTokenNode = &Node{
		Name: "Auth Token Phase",
		IsRestNode: true,
		Setup: func(s *StateContext) error {
			registerRoute(s, "/api/has_auth", func(w http.ResponseWriter, r *http.Request) {
				tokPath := filepath.Join(s.OauthDir, "token.json.enc")
				hasAuth := false
				if s.EncKey != "" {
					if ciphertext, err := os.ReadFile(tokPath); err == nil {
						if plaintext, err := cryptoutil.Decrypt(ciphertext, s.EncKey); err == nil {
							tok := &oauth2.Token{}
							if err := json.Unmarshal(plaintext, tok); err == nil && tok.AccessToken != "" {
								hasAuth = true
							}
						}
					}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]bool{"hasAuth": hasAuth})
			})
			registerRoute(s, "/auth/has_token", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]bool{"hasToken": hasToken})
			})
			registerRoute(s, "/api/auth_url", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"url": authURLStr})
			})
			registerRoute(s, "/api/token", func(w http.ResponseWriter, r *http.Request) {
				code := ""
				if r.Method == http.MethodGet {
					code = r.URL.Query().Get("code")
				} else if r.Method == http.MethodPost {
					var payload tokenPayload
					json.NewDecoder(r.Body).Decode(&payload)
					code = payload.Code
				}
				if code != "" {
					authCodeChan <- code
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status": "ok"}`))
			})
			return nil
		},
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			credPath := filepath.Join(s.OauthDir, "credentials.json")
			tokPath := filepath.Join(s.OauthDir, "token.json.enc")
			credBytes, _ := os.ReadFile(credPath)
			config, _ := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope)

			baseURL := os.Getenv("OAUTH_REDIRECT_URL")
			if baseURL == "" {
				baseURL = "http://localhost:7070"
			}

			var tok *oauth2.Token
			if ciphertext, err := os.ReadFile(tokPath); err == nil {
				if plaintext, err := cryptoutil.Decrypt(ciphertext, s.EncKey); err == nil {
					tok = &oauth2.Token{}
					json.Unmarshal(plaintext, tok)
				}
			}

			if tok == nil || tok.AccessToken == "" {
				ticket, cmd, err := runReceiver(s.ReceiverFlag)
				if err != nil {
					return err
				}
				defer func() {
					if cmd != nil && cmd.Process != nil {
						cmd.Process.Kill()
					}
				}()

				config.RedirectURL = fmt.Sprintf("%s/auth/%s", strings.TrimRight(baseURL, "/"), ticket)
				authURLStr = config.AuthCodeURL("state-token",
					oauth2.AccessTypeOffline,
					oauth2.SetAuthURLParam("device_id", "lounge-display"),
					oauth2.SetAuthURLParam("device_name", "Lounge Display"),
				)
				fmt.Println("Waiting for auth code...")
				authCode := <-authCodeChan
				tok, err = config.Exchange(context.Background(), authCode)
				if err != nil {
					return err
				}

				tokenBytes, _ := json.Marshal(tok)
				ciphertext, _ := cryptoutil.Encrypt(tokenBytes, s.EncKey)
				os.WriteFile(tokPath, ciphertext, 0600)
			}
			hasToken = true
			return nil
		},
	}

	CalendarNode = &Node{
		Name: "Calendar Logic Phase",
		Setup: func(s *StateContext) error {
			registerRoute(s, "/auth/has_calendar", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]bool{"hasCalendar": hasCalendar})
			})
			return nil
		},
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			credPath := filepath.Join(s.OauthDir, "credentials.json")
			tokPath := filepath.Join(s.OauthDir, "token.json.enc")
			credBytes, _ := os.ReadFile(credPath)
			config, _ := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope)

			ciphertext, _ := os.ReadFile(tokPath)
			plaintext, _ := cryptoutil.Decrypt(ciphertext, s.EncKey)
			tok := &oauth2.Token{}
			json.Unmarshal(plaintext, tok)

			ctx := context.Background()
			client := config.Client(ctx, tok)
			srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
			if err != nil {
				return err
			}

			t := time.Now().Format(time.RFC3339)
			for {
				_, err := srv.Events.List("primary").ShowDeleted(false).
					SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
				if err == nil {
					fmt.Println("Calendar events fetched successfully!")
					hasCalendar = true
					break
				}
				fmt.Printf("Unable to retrieve calendar events: %v. Retrying in 5 seconds...\n", err)
				time.Sleep(5 * time.Second)
			}
			return nil
		},
	}

	// --- Google Login CDP Nodes ---

	InitCDPNode = &Node{
		Name:     "Init CDP",
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			kioskIP := os.Getenv("KIOSK_IP")
			if kioskIP == "" {
				kioskIP = "127.0.0.1"
			}
			if ips, err := net.LookupIP(kioskIP); err == nil && len(ips) > 0 {
				kioskIP = ips[0].String()
			}
			cdpPort := os.Getenv("CDP_PORT")
			if cdpPort == "" {
				cdpPort = "9222"
			}
			wsURL := fmt.Sprintf("ws://%s:%s", kioskIP, cdpPort)
			fmt.Printf("Connecting to Chrome on %s...\n", wsURL)
			for {
				allocCtx, _ := chromedp.NewRemoteAllocator(context.Background(), wsURL)
				ctx, _ := chromedp.NewContext(allocCtx)

				targets, err := chromedp.Targets(ctx)
				if err == nil {
					var activeTarget *target.Info
					for _, t := range targets {
						if t.Type == "page" && !strings.HasPrefix(t.URL, "chrome://") && !strings.HasPrefix(t.URL, "devtools://") {
							activeTarget = t
							break
						}
					}
					if activeTarget != nil {
						targetCtx, _ := chromedp.NewContext(allocCtx, chromedp.WithTargetID(activeTarget.TargetID))
						s.Ctx = ctx
						s.TargetCtx = targetCtx

						if err := chromedp.Run(targetCtx); err != nil {
							fmt.Printf("Init target run failed: %v\n", err)
							continue
						}

						fmt.Println("CDP Connected.")
						return nil
					}
				}
				fmt.Println("Failed to connect to CDP or find target. Retrying in 5 seconds...")
				time.Sleep(5 * time.Second)
			}
		},
	}

	StartMeetNode = &Node{
		Name:     "Start Meet Navigation",
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			fmt.Println("Navigating to https://meet.google.com")
			return chromedp.Run(s.TargetCtx,
				chromedp.Navigate("https://meet.google.com"),
				chromedp.Sleep(4*time.Second),
			)
		},
	}

	FinishedMeetNode = &Node{
		Name: "Finished Meet",
		PreCheck: func(s *StateContext) bool {
			var urlStr string
			chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			u, err := url.Parse(urlStr)
			return err == nil && u.Host == "meet.google.com"
		},
		Work: func(s *StateContext) error {
			fmt.Println("Logged into Meet successfully.")
			return nil
		},
	}

	FinalizeNode = &Node{
		Name:       "Finalize Setup",
		IsRestNode: true,
		PreCheck:   func(s *StateContext) bool { return true },
		Setup: func(s *StateContext) error {
			registerRoute(s, "/auth/finalize", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"success":true}`))
				go func() {
					time.Sleep(1 * time.Second)
					os.Exit(0)
				}()
			})
			return nil
		},
		Work: func(s *StateContext) error {
			fmt.Println("Setup complete! Waiting for /auth/finalize to be called...")
			return nil
		},
	}

	WorkspaceRedirectedNode = &Node{
		Name: "Workspace Redirected",
		PreCheck: func(s *StateContext) bool {
			var urlStr string
			chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			u, err := url.Parse(urlStr)
			return err == nil && u.Host == "workspace.google.com"
		},
		Work: func(s *StateContext) error {
			var res string
			return chromedp.Run(s.TargetCtx,
				chromedp.Evaluate(`
					(function() {
						let btns = Array.from(document.querySelectorAll('a[data-g-action="sign in"]'));
						let visibleBtn = btns.find(b => b.offsetWidth > 0 && b.offsetHeight > 0);
						if (visibleBtn) {
							visibleBtn.click();
							return "clicked";
						}
						return "not found";
					})();
				`, &res),
				chromedp.Sleep(3*time.Second),
			)
		},
	}

	AccountsGooglePageNode = &Node{
		Name: "Accounts Google Page",
		PreCheck: func(s *StateContext) bool {
			var urlStr string
			chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			u, err := url.Parse(urlStr)
			return err == nil && u.Host == "accounts.google.com"
		},
	}

	ChooseAccountNode = &Node{
		Name: "Choose Account Page",
		PreCheck: func(s *StateContext) bool {
			var found bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
				(function() {
					let el = document.querySelector('div[data-identifier], div[data-email], #profileIdentifier, .w1I7fb');
					let txt = document.body.innerText.toLowerCase();
					return (el !== null) || txt.includes("choose an account");
				})();
			`, &found))
			return found
		},
	}

	AccountOptionExistsNode = &Node{
		Name: "Account Option Exists",
		PreCheck: func(s *StateContext) bool {
			var found bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
				(function() {
					let txt = document.body.innerText.toLowerCase();
					return txt.includes("`+s.Email+`");
				})();
			`, &found))
			return found
		},
		Work: func(s *StateContext) error {
			var res string
			return chromedp.Run(s.TargetCtx,
				chromedp.Evaluate(`
					(function() {
						let els = Array.from(document.querySelectorAll('div'));
						let el = els.find(e => e.innerText && e.innerText.includes("`+s.Email+`") && e.getAttribute("data-identifier"));
						if (!el) {
							el = els.find(e => e.innerText && e.innerText.includes("`+s.Email+`") && e.getAttribute("role") === "link");
						}
						if (el) {
							el.click();
							return "clicked";
						}
						return "not found";
					})();
				`, &res),
				chromedp.Sleep(2*time.Second),
			)
		},
	}

	AccountOptionMissingNode = &Node{
		Name: "Account Option Missing",
		PreCheck: func(s *StateContext) bool {
			var found bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
				(function() {
					let txt = document.body.innerText.toLowerCase();
					return txt.includes("use another account");
				})();
			`, &found))
			return found
		},
		Work: func(s *StateContext) error {
			var res string
			return chromedp.Run(s.TargetCtx,
				chromedp.Evaluate(`
					(function() {
						let els = Array.from(document.querySelectorAll('div'));
						let el = els.find(e => e.innerText && e.innerText.toLowerCase() === "use another account");
						if (el) {
							el.click();
							return "clicked";
						}
						return "not found";
					})();
				`, &res),
				chromedp.Sleep(2*time.Second),
			)
		},
	}

	EmailInputNode = &Node{
		Name: "Email Input Page",
		PreCheck: func(s *StateContext) bool {
			var exists bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`document.querySelector('#identifierId') !== null`, &exists))
			return exists
		},
		Work: func(s *StateContext) error {
			return chromedp.Run(s.TargetCtx,
				chromedp.WaitVisible(`#identifierId`, chromedp.ByID),
				chromedp.Click(`#identifierId`, chromedp.ByID),
				chromedp.Sleep(500*time.Millisecond),
				chromedp.SendKeys(`#identifierId`, s.Email, chromedp.ByID),
				chromedp.Sleep(500*time.Millisecond),
				chromedp.Click(`#identifierNext button`, chromedp.ByQuery),
				chromedp.Sleep(3*time.Second),
			)
		},
	}

	PasswordInputNode = &Node{
		Name: "Password Input Page",
		IsRestNode: true,
		Setup: func(s *StateContext) error {
			registerRoute(s, "/api/password", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					return
				}
				body, _ := io.ReadAll(r.Body)
				var payload struct {
					Password string `json:"password"`
				}
				json.Unmarshal(body, &payload)
				if payload.Password != "" {
					s.PasswordChan <- payload.Password
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status": "ok"}`))
			})
			return nil
		},
		PreCheck: func(s *StateContext) bool {
			var exists bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`document.querySelector('input[type="password"]') !== null && document.querySelector('input[type="password"]').offsetWidth > 0`, &exists))
			return exists
		},
		Work: func(s *StateContext) error {
			fmt.Println("Waiting for password on POST /api/password ...")
			password := <-s.PasswordChan
			return chromedp.Run(s.TargetCtx,
				chromedp.SendKeys(`input[type="password"]`, password, chromedp.ByQuery),
				chromedp.Sleep(500*time.Millisecond),
				chromedp.Click(`#passwordNext button`, chromedp.ByQuery),
				chromedp.Sleep(3*time.Second),
			)
		},
	}

	WrongPasswordNode = &Node{
		Name: "Wrong Password",
		PreCheck: func(s *StateContext) bool {
			var errorText string
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
				(function() {
					let els = Array.from(document.querySelectorAll('span, div'));
					let errEl = els.find(e => e.innerText && (e.innerText.toLowerCase().includes('wrong password') || e.innerText.toLowerCase().includes('incorrect')) && e.offsetWidth > 0);
					if (errEl) return errEl.innerText.trim();
					return "";
				})();
			`, &errorText))
			return errorText != ""
		},
		Work: func(s *StateContext) error {
			fmt.Println("Wrong password entered, retrying...")
			return nil
		},
	}

	TwoFactorNode = &Node{
		Name: "2FA or Authenticated",
		PreCheck: func(s *StateContext) bool {
			var urlStr string
			chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			u, err := url.Parse(urlStr)
			if err == nil && u.Host != "accounts.google.com" {
				return true
			}
			var found bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
				(function() {
					let txt = document.body.innerText.toLowerCase();
					return txt.includes('2-step verification') || txt.includes('verifying it');
				})();
			`, &found))
			return found
		},
		DoneCheck: func(s *StateContext) error {
			fmt.Println("Polling for 2FA completion (up to 10 minutes)...")
			deadline := time.Now().Add(10 * time.Minute)
			for time.Now().Before(deadline) {
				var urlStr string
				chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
				u, err := url.Parse(urlStr)
				if err == nil && (u.Host == "meet.google.com" || u.Host == "workspace.google.com") {
					return nil
				}
				time.Sleep(15 * time.Second)
			}
			return fmt.Errorf("timed out waiting for 2FA completion")
		},
	}

	// Link nodes
	InitServerNode.Next = []*Node{CredentialsNode}
	CredentialsNode.Next = []*Node{AuthTokenNode}
	AuthTokenNode.Next = []*Node{CalendarNode}
	CalendarNode.Next = []*Node{InitCDPNode}
	InitCDPNode.Next = []*Node{StartMeetNode}

	StartMeetNode.Next = []*Node{WorkspaceRedirectedNode, FinishedMeetNode}
	WorkspaceRedirectedNode.Next = []*Node{AccountsGooglePageNode}
	AccountsGooglePageNode.Next = []*Node{ChooseAccountNode, EmailInputNode, PasswordInputNode}

	ChooseAccountNode.Next = []*Node{AccountOptionExistsNode, AccountOptionMissingNode}
	AccountOptionExistsNode.Next = []*Node{PasswordInputNode}
	AccountOptionMissingNode.Next = []*Node{EmailInputNode}

	EmailInputNode.Next = []*Node{PasswordInputNode}
	PasswordInputNode.Next = []*Node{WrongPasswordNode, TwoFactorNode}
	WrongPasswordNode.Next = []*Node{PasswordInputNode}

	TwoFactorNode.Next = []*Node{FinishedMeetNode, WorkspaceRedirectedNode}
	FinishedMeetNode.Next = []*Node{FinalizeNode}
}

func RunEngine(startNode *Node, s *StateContext) {
	currentNode := startNode
	for {
		s.StepCounter++
		fmt.Printf("\n=== Executing Node %04d: %s ===\n", s.StepCounter, currentNode.Name)
		s.SetNodeName(currentNode.Name)

		if currentNode.Setup != nil {
			err := currentNode.Setup(s)
			if err != nil {
				log.Printf("Setup failed for node %s: %v\n", currentNode.Name, err)
			}
		}

		if s.TargetCtx != nil && currentNode != InitServerNode && currentNode != CredentialsNode && currentNode != AuthTokenNode && currentNode != CalendarNode {
			captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_"), "pre", "setup_")
		}

		if currentNode.Work != nil {
			err := currentNode.Work(s)
			if err != nil {
				log.Printf("Work failed for node %s: %v\n", currentNode.Name, err)
			}
		}

		if s.TargetCtx != nil && currentNode != InitServerNode && currentNode != CredentialsNode && currentNode != AuthTokenNode && currentNode != CalendarNode {
			captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_"), "post", "setup_")
		}

		if currentNode.DoneCheck != nil {
			err := currentNode.DoneCheck(s)
			if err != nil {
				log.Printf("DoneCheck failed for node %s: %v\n", currentNode.Name, err)
				currentNode = startNode
				continue
			}
		}

		if len(currentNode.Next) == 0 && !currentNode.IsRestNode {
			fmt.Println("\nFlow finished successfully at terminal node:", currentNode.Name)
			return
		}

		fmt.Printf("Finding next node from %d possibilities...\n", len(currentNode.Next))
		var nextNode *Node

		var timeout time.Duration
		if currentNode.IsRestNode {
			timeout = 0
		} else if s.NodeTimeout > 0 {
			timeout = s.NodeTimeout
		} else {
			timeout = 5 * time.Minute
		}
		deadline := time.Now().Add(timeout)

		for nextNode == nil {
			if !currentNode.IsRestNode && time.Now().After(deadline) {
				fmt.Printf("Non-rest node timeout reached (%v).\n", timeout)
				break
			}

			for _, n := range currentNode.Next {
				if n == currentNode {
					continue // Ignore self-links
				}
				if n.PreCheck != nil && n.PreCheck(s) {
					nextNode = n
					break
				}
			}

			if nextNode != nil {
				break
			}

			if currentNode.IsRestNode && currentNode.PreCheck != nil {
				if !currentNode.PreCheck(s) {
					fmt.Printf("Rest node %s condition is no longer valid.\n", currentNode.Name)
					break
				}
			}

			time.Sleep(500 * time.Millisecond)
		}

		if nextNode == nil {
			fmt.Println("\nERROR: No valid next path found! Restarting flow.")
			currentNode = startNode
			continue
		}

		if currentNode.Teardown != nil {
			err := currentNode.Teardown(s)
			if err != nil {
				log.Printf("Teardown failed for node %s: %v\n", currentNode.Name, err)
			}
		}

		currentNode = nextNode
	}
}

func main() {
	encKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if encKey == "" {
		log.Fatalf("TOKEN_ENCRYPTION_KEY environment variable is required")
	}
	oauthDir := os.Getenv("OAUTH_DIR")
	if oauthDir == "" {
		oauthDir = "."
	}

	dirFlag := flag.String("dir", "startup", "the directory to serve")
	portFlag := flag.String("port", "8080", "the port to listen on")
	receiverFlag := flag.String("receiver", "/app/receiver", "the path to the receiver binary")
	logsDirFlag := flag.String("logs-dir", "logs", "Directory to store HTML dumps and screenshots")
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
	if err := os.RemoveAll(*logsDirFlag); err != nil {
		log.Printf("Warning: Failed to clear logs directory: %v\n", err)
	}
	if err := os.MkdirAll(*logsDirFlag, 0755); err != nil {
		log.Printf("Warning: Failed to create logs directory: %v\n", err)
	}

	stateCtx := &StateContext{
		Email:        "lounge.room@mountainviewmasoniclodge.com",
		LogsDir:      *logsDirFlag,
		PasswordChan: make(chan string),
		DirFlag:      dir,
		PortFlag:     *portFlag,
		ReceiverFlag: *receiverFlag,
		EncKey:       encKey,
		OauthDir:     oauthDir,
		NodeTimeout:  20 * time.Minute,
	}

	initNodes()
	fmt.Println("Starting State Machine Engine...")
	RunEngine(InitServerNode, stateCtx)
}
