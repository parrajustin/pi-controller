package setup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/cryptoutil"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Orchestration channels
var credChan = make(chan []byte)
var authCodeChan = make(chan string)
var authURLStr string
var hasToken bool

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

var (
	CredentialsNode          *Node
	AuthTokenNode            *Node
	WorkspaceRedirectedNode  *Node
	AccountsGooglePageNode   *Node
	ChooseAccountNode        *Node
	AccountOptionExistsNode  *Node
	AccountOptionMissingNode *Node
	EmailInputNode           *Node
	PasswordInputNode        *Node
	WrongPasswordNode        *Node
	TwoFactorNode            *Node
	CalendarNode             *Node
)

func init() {
	CredentialsNode = &Node{
		Name:       "Credentials Phase",
		IsRestNode: true,
		Setup: func(s *StateContext) error {
			RegisterRoute(s, "/api/has_cred", func(w http.ResponseWriter, r *http.Request) {
				credPath := filepath.Join(s.OauthDir, "credentials.json")
				info, err := os.Stat(credPath)
				hasCreds := err == nil && info.Size() > 0
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]bool{"hasCreds": hasCreds})
			})
			RegisterRoute(s, "/api/cred", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "Failed to read body", http.StatusInternalServerError)
					return
				}
				credChan <- body
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status": "ok"}`))
			})
			return nil
		},
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			credPath := filepath.Join(s.OauthDir, "credentials.json")

			needsCred := true
			if info, err := os.Stat(credPath); err == nil && info.Size() > 0 {
				// Try parsing it
				credBytes, _ := os.ReadFile(credPath)
				if _, parseErr := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope); parseErr == nil {
					needsCred = false
				}
			}

			if needsCred {
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
		Name:       "Auth Token Phase",
		IsRestNode: true,
		Setup: func(s *StateContext) error {
			RegisterRoute(s, "/api/has_auth", func(w http.ResponseWriter, r *http.Request) {
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
			RegisterRoute(s, "/auth/has_token", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]bool{"hasToken": hasToken})
			})
			RegisterRoute(s, "/api/auth_url", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"url": authURLStr})
			})
			RegisterRoute(s, "/api/token", func(w http.ResponseWriter, r *http.Request) {
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
				if config == nil {
					return fmt.Errorf("oauth config is nil, credentials may be missing or invalid")
				}
				if s.ReceiverFlag != "" {
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
				} else {
					config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"
				}

				authURLStr = config.AuthCodeURL("state-token",
					oauth2.AccessTypeOffline,
					oauth2.SetAuthURLParam("device_id", "lounge-display"),
					oauth2.SetAuthURLParam("device_name", "Lounge Display"),
				)
				fmt.Println("Waiting for auth code on authCodeChan...")
				authCode := <-authCodeChan
				var err error
				tok, err = config.Exchange(context.Background(), authCode)
				if err != nil {
					return err
				}

				tokenBytes, _ := json.Marshal(tok)
				ciphertext, _ := cryptoutil.Encrypt(tokenBytes, s.EncKey)
				os.WriteFile(tokPath, ciphertext, 0600)
			}
			hasToken = true
			
			// Also initialize the CalendarSrv here so we have it later
			ctx := context.Background()
			client := config.Client(ctx, tok)
			srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
			if err == nil {
				s.CalendarSrv = srv
			}

			return nil
		},
	}

	CalendarNode = &Node{
		Name:     "Calendar Logic Phase",
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			for {
				t := time.Now().Format(time.RFC3339)
				if s.CalendarSrv == nil {
					return fmt.Errorf("CalendarSrv is nil")
				}
				_, err := s.CalendarSrv.Events.List("primary").ShowDeleted(false).
					SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
				if err != nil {
					fmt.Printf("Unable to retrieve calendar events: %v, retrying in 5 seconds...\n", err)
					time.Sleep(5 * time.Second)
					continue
				}
				fmt.Println("Calendar events fetched successfully!")
				return nil
			}
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
		Name:       "Password Input Page",
		IsRestNode: true,
		Setup: func(s *StateContext) error {
			RegisterRoute(s, "/api/password", func(w http.ResponseWriter, r *http.Request) {
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
}
