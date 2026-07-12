package setup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/browser"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/calendarclient"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/cryptoutil"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Orchestration channels
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
		slog.Info("[receiver]", "line", line)
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
			slog.Info("[receiver]", "line", line)
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
	WaitWebServerNode         *Node
	InitDisplay2CDPNode       *Node
	InitCDPNode               *Node
	WaitForClientCallbackNode *Node
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
	InitCDPNode = &Node{
		Name: "Init CDP",
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
			slog.Info("Connecting to Chrome", "url", wsURL)

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
						
						// Initialize real browser here
						s.Browser = browser.NewRealBrowser(targetCtx)
						s.Ctx = ctx
						
						go func(monitorCtx context.Context, sCtx *StateContext) {
							<-monitorCtx.Done()
							slog.Warn("CDP Connection lost on main screen (port 9222)")
							cdpConnectionLossCount.Add(context.Background(), 1, metric.WithAttributes(attribute.String("screen", "main")))
							sCtx.mu.Lock()
							sCtx.Browser = nil
							if sCtx.ForceResetChan != nil {
								select {
								case sCtx.ForceResetChan <- sCtx.DefaultNode:
								default:
								}
							}
							sCtx.mu.Unlock()
						}(ctx, s)

						// Keep target ctx around in case we need it elsewhere (or it gets GC'd)
						
						// Check if connection works
						var tmp string
						if _, err := s.Browser.Location(); err == nil || tmp == "" { // Ignore error for init
							slog.Info("CDP Connected.")
							return nil
						}
					}
				}
				slog.Warn("Failed to connect to CDP or find target. Retrying in 5 seconds...")
				s.Clock.Sleep(5 * time.Second)
			}
		},
	}

	WaitWebServerNode = &Node{
		Name: "wait_web_server",
		Setup: func(s *StateContext) error {
			s.DefaultNode = WaitWebServerNode
			return nil
		},
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			port := s.PortFlag
			if port == "" {
				port = "8080"
			}
			url := fmt.Sprintf("http://127.0.0.1:%s/", port)
			slog.Info("Waiting for web server to start...", "url", url)
			for {
				resp, err := http.Get(url)
				if err == nil {
					resp.Body.Close()
					slog.Info("Web server is up!")
					return nil
				}
				slog.Debug("Web server not ready, retrying...", "error", err)
				s.Clock.Sleep(1 * time.Second)
			}
		},
	}

	InitDisplay2CDPNode = &Node{
		Name:     "Init Display 2 CDP",
		Setup: func(s *StateContext) error {
			s.mu.Lock()
			s.DisplayActive = false
			s.mu.Unlock()
			s.AddWSHandler("validate_display_active", func(payload json.RawMessage) (interface{}, error) {
				s.mu.Lock()
				s.DisplayActive = true
				s.mu.Unlock()
				return map[string]string{"status": "ok"}, nil
			})
			return nil
		},
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			kioskIP := os.Getenv("KIOSK_IP")
			if kioskIP == "" {
				kioskIP = "127.0.0.1"
			}
			if ips, err := net.LookupIP(kioskIP); err == nil && len(ips) > 0 {
				kioskIP = ips[0].String()
			}
			cdpPort := os.Getenv("DISPLAY2_CDP_PORT")
			if cdpPort == "" {
				cdpPort = "9225"
			}
			wsURL := fmt.Sprintf("ws://%s:%s", kioskIP, cdpPort)
			slog.Info("Connecting to Display 2 Chrome", "url", wsURL)

			for {
				allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), wsURL)
				ctx, ctxCancel := chromedp.NewContext(allocCtx)

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
						targetCtx, targetCancel := chromedp.NewContext(allocCtx, chromedp.WithTargetID(activeTarget.TargetID))

						b := browser.NewRealBrowser(targetCtx)
						targetUrl := os.Getenv("DISPLAY2_URL")
						if targetUrl == "" {
							targetUrl = "http://localhost:8080"
						}
						slog.Info("Navigating Display 2", "url", targetUrl)
						err := b.Navigate(targetUrl)
						if err == nil {
							s.mu.Lock()
							s.Browser2 = b
							s.Ctx2 = ctx
							s.mu.Unlock()
							
							go func(monitorCtx context.Context, sCtx *StateContext) {
								<-monitorCtx.Done()
								slog.Warn("CDP Connection lost on display page (port 9223)")
								cdpConnectionLossCount.Add(context.Background(), 1, metric.WithAttributes(attribute.String("screen", "display_page")))
								sCtx.mu.Lock()
								sCtx.Browser2 = nil
								if sCtx.ForceResetChan != nil {
									select {
									case sCtx.ForceResetChan <- sCtx.DefaultNode:
									default:
									}
								}
								sCtx.mu.Unlock()
							}(ctx, s)

							// Successfully told the browser to navigate.
							// Do not cancel the contexts here, as cancelling the allocator
							// or target context can cause Chrome to exit or close the target.
							// We intentionally leave the CDP connection open so Chrome stays alive.
							return nil
						}
						slog.Warn("Navigation failed, will retry", "error", err)
						targetCancel()
					}
				}
				ctxCancel()
				allocCancel()

				slog.Warn("Failed to connect to Display 2 CDP or find target. Retrying in 5 seconds...")
				s.Clock.Sleep(5 * time.Second)
			}
		},
	}

	WaitForClientCallbackNode = &Node{
		Name: "Wait For Client Callback",
		IsRestNode: true,
		PreCheck: func(s *StateContext) bool { return true },
		Teardown: func(s *StateContext) error {
			s.RemoveWSHandler("validate_display_active")
			return nil
		},
	}

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
				
				credPath := filepath.Join(s.OauthDir, "credentials.json")
				err = os.WriteFile(credPath, body, 0600)
				if err != nil {
					http.Error(w, "Failed to write file", http.StatusInternalServerError)
					return
				}
				slog.Info("Saved credentials.json")
				
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status": "ok"}`))
			})
			return nil
		},
		PreCheck: func(s *StateContext) bool {
			s.mu.Lock()
			defer s.mu.Unlock()
			return s.DisplayActive
		},
		Work: func(s *StateContext) error {
			slog.Info("Waiting for credentials on POST /api/cred (non-blocking)...")
			return nil
		},
	}

	AuthTokenNode = &Node{
		Name:       "Auth Token Phase",
		IsRestNode: true,
		Setup: func(s *StateContext) error {
			s.AddWSHandler("get_auth_url", func(payload json.RawMessage) (interface{}, error) {
				return map[string]string{"url": authURLStr}, nil
			})
			s.AddWSHandler("submit_token", func(payload json.RawMessage) (interface{}, error) {
				var p tokenPayload
				if err := json.Unmarshal(payload, &p); err == nil && p.Code != "" {
					credPath := filepath.Join(s.OauthDir, "credentials.json")
					tokPath := filepath.Join(s.OauthDir, "token.json.enc")
					credBytes, _ := os.ReadFile(credPath)
					config, _ := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope)
					if config != nil {
						baseURL := os.Getenv("OAUTH_REDIRECT_URL")
						if baseURL == "" {
							baseURL = "http://localhost:7070"
						}
						
						if s.ReceiverFlag != "" {
							// For simplicity, we assume the code works with oob if ReceiverFlag isn't handling it perfectly in async.
							config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"
						} else {
							config.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"
						}
		
						tok, err := config.Exchange(context.Background(), p.Code)
						if err == nil {
							tokenBytes, _ := json.Marshal(tok)
							ciphertext, _ := cryptoutil.Encrypt(tokenBytes, s.EncKey)
							os.WriteFile(tokPath, ciphertext, 0600)
							
							hasToken = true
							ctx := context.Background()
							client := config.Client(ctx, tok)
							srv, _ := calendar.NewService(ctx, option.WithHTTPClient(client))
							s.CalendarClient = calendarclient.NewRealCalendarClient(srv)
						} else {
							slog.Error("Failed to exchange token", "error", err)
						}
					}
				}
				return map[string]string{"status": "ok"}, nil
			})
			return nil
		},
		Teardown: func(s *StateContext) error {
			s.RemoveWSHandler("get_auth_url")
			s.RemoveWSHandler("submit_token")
			return nil
		},
		PreCheck: func(s *StateContext) bool {
			credPath := filepath.Join(s.OauthDir, "credentials.json")
			info, err := os.Stat(credPath)
			if err == nil && info.Size() > 0 {
				credBytes, _ := os.ReadFile(credPath)
				if _, parseErr := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope); parseErr == nil {
					return true
				}
			}
			return false
		},
		Work: func(s *StateContext) error {
			credPath := filepath.Join(s.OauthDir, "credentials.json")
			tokPath := filepath.Join(s.OauthDir, "token.json.enc")
			credBytes, _ := os.ReadFile(credPath)
			config, _ := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope)

			var tok *oauth2.Token
			if ciphertext, err := os.ReadFile(tokPath); err == nil {
				if plaintext, err := cryptoutil.Decrypt(ciphertext, s.EncKey); err == nil {
					tok = &oauth2.Token{}
					json.Unmarshal(plaintext, tok)
				}
			}

			if tok != nil && tok.AccessToken != "" {
				hasToken = true
				ctx := context.Background()
				client := config.Client(ctx, tok)
				srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
				if err == nil {
					s.CalendarClient = calendarclient.NewRealCalendarClient(srv)
				}
				return nil
			}

			if config == nil {
				return fmt.Errorf("oauth config is nil, credentials may be missing or invalid")
			}
			
			baseURL := os.Getenv("OAUTH_REDIRECT_URL")
			if baseURL == "" {
				baseURL = "http://localhost:7070"
			}

			if s.ReceiverFlag != "" {
				ticket, cmd, err := runReceiver(s.ReceiverFlag)
				if err != nil {
					return err
				}
				go func() {
					defer func() {
						if cmd != nil && cmd.Process != nil {
							cmd.Process.Kill()
						}
					}()
					
					// Wait for receiver code async
					authCode := <-authCodeChan
					tok, err = config.Exchange(context.Background(), authCode)
					if err == nil {
						tokenBytes, _ := json.Marshal(tok)
						ciphertext, _ := cryptoutil.Encrypt(tokenBytes, s.EncKey)
						os.WriteFile(tokPath, ciphertext, 0600)
						hasToken = true
						ctx := context.Background()
						client := config.Client(ctx, tok)
						srv, _ := calendar.NewService(ctx, option.WithHTTPClient(client))
						s.CalendarClient = calendarclient.NewRealCalendarClient(srv)
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
			slog.Info("Waiting for auth code on WS / submit_token (non-blocking)...")
			
			return nil
		},
	}

	CalendarNode = &Node{
		Name:     "Calendar Logic Phase",
		PreCheck: func(s *StateContext) bool { 
			return s.CalendarClient != nil && hasToken 
		},
		Work: func(s *StateContext) error {
			for {
				err := s.CalendarClient.TestConnection()
				if err != nil {
					slog.Warn("Unable to retrieve calendar events, retrying in 5 seconds...", "error", err)
					s.Clock.Sleep(5 * time.Second)
					continue
				}
				slog.Info("Calendar events fetched successfully!")
				return nil
			}
		},
	}

	WorkspaceRedirectedNode = &Node{
		Name: "Workspace Redirected",
		PreCheck: func(s *StateContext) bool {
			urlStr, err := s.Browser.Location()
			if err != nil {
				return false
			}
			u, err := url.Parse(urlStr)
			return err == nil && u.Host == "workspace.google.com"
		},
		Work: func(s *StateContext) error {
			var res string
			if err := s.Browser.Evaluate(`
					(function() {
						let btns = Array.from(document.querySelectorAll('a[data-g-action="sign in"]'));
						let visibleBtn = btns.find(b => b.offsetWidth > 0 && b.offsetHeight > 0);
						if (visibleBtn) {
							visibleBtn.click();
							return "clicked";
						}
						return "not found";
					})();
				`, &res); err != nil {
				return err
			}
			return s.Browser.Sleep(3 * time.Second)
		},
	}

	AccountsGooglePageNode = &Node{
		Name: "Accounts Google Page",
		PreCheck: func(s *StateContext) bool {
			urlStr, err := s.Browser.Location()
			if err != nil {
				return false
			}
			u, err := url.Parse(urlStr)
			return err == nil && u.Host == "accounts.google.com"
		},
	}

	ChooseAccountNode = &Node{
		Name: "Choose Account Page",
		PreCheck: func(s *StateContext) bool {
			var found bool
			s.Browser.Evaluate(`
				(function() {
					let el = document.querySelector('div[data-identifier], div[data-email], #profileIdentifier, .w1I7fb');
					let txt = document.body.innerText.toLowerCase();
					return (el !== null) || txt.includes("choose an account");
				})();
			`, &found)
			return found
		},
	}

	AccountOptionExistsNode = &Node{
		Name: "Account Option Exists",
		PreCheck: func(s *StateContext) bool {
			var found bool
			s.Browser.Evaluate(`
				(function() {
					let txt = document.body.innerText.toLowerCase();
					return txt.includes("`+s.Email+`");
				})();
			`, &found)
			return found
		},
		Work: func(s *StateContext) error {
			var res string
			if err := s.Browser.Evaluate(`
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
				`, &res); err != nil {
				return err
			}
			return s.Browser.Sleep(2 * time.Second)
		},
	}

	AccountOptionMissingNode = &Node{
		Name: "Account Option Missing",
		PreCheck: func(s *StateContext) bool {
			var found bool
			s.Browser.Evaluate(`
				(function() {
					let txt = document.body.innerText.toLowerCase();
					return txt.includes("use another account");
				})();
			`, &found)
			return found
		},
		Work: func(s *StateContext) error {
			var res string
			if err := s.Browser.Evaluate(`
					(function() {
						let els = Array.from(document.querySelectorAll('div'));
						let el = els.find(e => e.innerText && e.innerText.toLowerCase() === "use another account");
						if (el) {
							el.click();
							return "clicked";
						}
						return "not found";
					})();
				`, &res); err != nil {
				return err
			}
			return s.Browser.Sleep(2 * time.Second)
		},
	}

	EmailInputNode = &Node{
		Name: "Email Input Page",
		PreCheck: func(s *StateContext) bool {
			var exists bool
			s.Browser.Evaluate(`document.querySelector('#identifierId') !== null`, &exists)
			return exists
		},
		Work: func(s *StateContext) error {
			if err := s.Browser.WaitVisible(`#identifierId`, true); err != nil {
				return err
			}
			if err := s.Browser.Click(`#identifierId`, true); err != nil {
				return err
			}
			if err := s.Browser.Sleep(500 * time.Millisecond); err != nil {
				return err
			}
			if err := s.Browser.SendKeys(`#identifierId`, s.Email, true); err != nil {
				return err
			}
			if err := s.Browser.Sleep(500 * time.Millisecond); err != nil {
				return err
			}
			if err := s.Browser.Click(`#identifierNext button`, false); err != nil {
				return err
			}
			return s.Browser.Sleep(3 * time.Second)
		},
	}

	PasswordInputNode = &Node{
		Name:       "Password Input Page",
		IsRestNode: true,
		Setup: func(s *StateContext) error {
			s.AddWSHandler("submit_password", func(payload json.RawMessage) (interface{}, error) {
				var p struct {
					Password string `json:"password"`
				}
				if err := json.Unmarshal(payload, &p); err == nil && p.Password != "" {
					go func(password string) {
						if err := s.Browser.SendKeys(`input[type="password"]`, password, false); err != nil {
							slog.Error("Failed to type password", "error", err)
							return
						}
						s.Browser.Sleep(500 * time.Millisecond)
						if err := s.Browser.Click(`#passwordNext button`, false); err != nil {
							slog.Error("Failed to click next", "error", err)
							return
						}
					}(p.Password)
				}
				return map[string]string{"status": "ok"}, nil
			})
			return nil
		},
		Teardown: func(s *StateContext) error {
			s.RemoveWSHandler("submit_password")
			return nil
		},
		PreCheck: func(s *StateContext) bool {
			var exists bool
			s.Browser.Evaluate(`document.querySelector('input[type="password"]') !== null && document.querySelector('input[type="password"]').offsetWidth > 0`, &exists)
			return exists
		},
		RestNodeValidation: func(s *StateContext) bool {
			urlStr, err := s.Browser.Location()
			if err == nil {
				u, err := url.Parse(urlStr)
				if err == nil && u.Host == "accounts.google.com" {
					return true
				}
			}
			// Fallback to original precheck if URL check fails for some reason
			var exists bool
			s.Browser.Evaluate(`document.querySelector('input[type="password"]') !== null && document.querySelector('input[type="password"]').offsetWidth > 0`, &exists)
			return exists
		},
		Work: func(s *StateContext) error {
			slog.Info("Waiting for password on POST /api/password (non-blocking)...")
			return nil
		},
	}

	WrongPasswordNode = &Node{
		Name: "Wrong Password",
		PreCheck: func(s *StateContext) bool {
			var errorText string
			s.Browser.Evaluate(`
				(function() {
					let els = Array.from(document.querySelectorAll('span, div'));
					let errEl = els.find(e => e.innerText && (e.innerText.toLowerCase().includes('wrong password') || e.innerText.toLowerCase().includes('incorrect')) && e.offsetWidth > 0);
					if (errEl) return errEl.innerText.trim();
					return "";
				})();
			`, &errorText)
			return errorText != ""
		},
		Work: func(s *StateContext) error {
			slog.Info("Wrong password entered, retrying...")
			return nil
		},
	}

	TwoFactorNode = &Node{
		Name: "2FA or Authenticated",
		IsRestNode: true,
		PreCheck: func(s *StateContext) bool {
			urlStr, err := s.Browser.Location()
			if err == nil {
				u, err := url.Parse(urlStr)
				if err == nil && u.Host != "accounts.google.com" {
					return true
				}
			}
			var found bool
			s.Browser.Evaluate(`
				(function() {
					let txt = document.body.innerText.toLowerCase();
					return txt.includes('2-step verification') || txt.includes('verifying it');
				})();
			`, &found)
			return found
		},
		Work: func(s *StateContext) error {
			slog.Info("Waiting for 2FA completion (non-blocking)...")
			return nil
		},
	}
}
