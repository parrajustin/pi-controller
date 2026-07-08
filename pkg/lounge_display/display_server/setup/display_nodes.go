package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

var (
	InitCDPNode             *Node
	StartMeetNode           *Node
	MeetLandingPageNode     *Node
	NavigateToMeeting       *Node
	JoinMeetingNode         *Node
	InMeetingNode           *Node
	LeaveMeetingNode        *Node
	CheckInvalidMeetingNode *Node
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
		Name: "Start Meet Navigation",
		PreCheck: func(s *StateContext) bool { return true },
		Setup: func(s *StateContext) error {
			s.DefaultNode = StartMeetNode
			return nil
		},
		Work: func(s *StateContext) error {
			fmt.Println("Navigating to https://meet.google.com")
			return chromedp.Run(s.TargetCtx,
				chromedp.Navigate("https://meet.google.com"),
				chromedp.Sleep(4*time.Second),
			)
		},
	}

	MeetLandingPageNode = &Node{
		Name: "Meet Landing Page",
		IsRestNode: true,
		PreCheck: func(s *StateContext) bool {
			s.mu.Lock()
			target := s.NavTarget
			s.mu.Unlock()
			if target != "" {
				return false
			}

			var urlStr string
			err := chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			if err != nil {
				return false
			}
			u, err := url.Parse(urlStr)
			return err == nil && u.Host == "meet.google.com" && (u.Path == "/" || u.Path == "/new" || u.Path == "")
		},
		Setup: func(s *StateContext) error {
			s.AddWSHandler("join_meeting", func(payload json.RawMessage) (interface{}, error) {
				var p struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(payload, &p); err != nil {
					return nil, fmt.Errorf("invalid payload")
				} else if p.Code == "" {
					return nil, fmt.Errorf("meeting code is required")
				}
				s.SetNavTarget("NavigateToMeeting", map[string]interface{}{"code": p.Code})
				return map[string]string{"status": "ok"}, nil
			})
			return nil
		},
		Work: func(s *StateContext) error {
			var urlStr string
			_ = chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			log.Printf("Entered Meet Landing Page. Current URL: %s\n", urlStr)
			return nil
		},
		Teardown: func(s *StateContext) error {
			s.RemoveWSHandler("join_meeting")
			return nil
		},
	}

	CheckInvalidMeetingNode = &Node{
		Name: "Check Invalid Meeting",
		PreCheck: func(s *StateContext) bool {
			var urlStr string
			err := chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			if err != nil {
				return false
			}
			u, err := url.Parse(urlStr)
			return err == nil && u.Host == "meet.google.com" && strings.HasPrefix(u.Path, "/_meet/")
		},
		Work: func(s *StateContext) error {
			s.mu.Lock()
			s.MeetingCode = ""
			s.mu.Unlock()
			return nil
		},
		DoneCheck: func(s *StateContext) error {
			return fmt.Errorf("invalid meeting URL detected, redirecting to default node")
		},
	}

	InMeetingNode = &Node{
		Name: "In Meeting",
		IsRestNode: true,
		PreCheck: func(s *StateContext) bool {
			s.mu.Lock()
			target := s.NavTarget
			s.mu.Unlock()
			if target != "" {
				return false
			}

			var urlStr string
			err := chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			if err != nil {
				return false
			}
			u, err := url.Parse(urlStr)
			if err != nil || u.Host != "meet.google.com" || u.Path == "/" || u.Path == "/new" || u.Path == "" || strings.HasPrefix(u.Path, "/_meet/") {
				return false
			}

			var hasJoinBtn bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
				(function() {
					let btns = Array.from(document.querySelectorAll('button, div[role="button"], span'));
					return btns.some(b => b.innerText && (b.innerText.toLowerCase().includes('join now') || b.innerText.toLowerCase().includes('ask to join') || b.innerText.toLowerCase().includes('join anyway')) && b.offsetWidth > 0 && b.offsetHeight > 0);
				})();
			`, &hasJoinBtn))
			
			return !hasJoinBtn
		},
		Setup: func(s *StateContext) error {
			s.AddWSHandler("button_state", func(payload json.RawMessage) (interface{}, error) {
				var stateJSON string
				err := chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
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
					return nil, err
				}
				var parsed map[string]interface{}
				json.Unmarshal([]byte(stateJSON), &parsed)
				return parsed, nil
			})

			s.AddWSHandler("click_button", func(payload json.RawMessage) (interface{}, error) {
				var p struct {
					Button string `json:"button"`
				}
				if err := json.Unmarshal(payload, &p); err != nil {
					return nil, fmt.Errorf("invalid payload")
				}
				var query string
				switch p.Button {
				case "microphone":
					query = `button[aria-label*="microphone" i]`
				case "camera":
					query = `button[aria-label*="camera" i]`
				case "hand":
					query = `button[aria-label*="hand" i]`
				case "hangup":
					s.SetNavTarget("LeaveMeeting", nil)
					return map[string]bool{"clicked": true}, nil
				default:
					return nil, fmt.Errorf("unknown button")
				}

				var clicked bool
				err := chromedp.Run(s.TargetCtx, chromedp.Evaluate(fmt.Sprintf(`
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
					return nil, err
				}
				return map[string]bool{"clicked": clicked}, nil
			})
			return nil
		},
		Work: func(s *StateContext) error {
			var urlStr string
			_ = chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			log.Printf("Entered In Meeting. Current URL: %s\n", urlStr)
			u, _ := url.Parse(urlStr)
			if u != nil {
				s.mu.Lock()
				s.MeetingCode = strings.TrimPrefix(u.Path, "/")
				s.mu.Unlock()
			}
			return nil
		},
		Teardown: func(s *StateContext) error {
			s.RemoveWSHandler("button_state")
			s.RemoveWSHandler("click_button")
			return nil
		},
	}

	NavigateToMeeting = &Node{
		Name: "NavigateToMeeting",
		PreCheck: func(s *StateContext) bool {
			s.mu.Lock()
			defer s.mu.Unlock()
			return s.NavTarget == "NavigateToMeeting"
		},
		Work: func(s *StateContext) error {
			s.mu.Lock()
			code := ""
			if s.NavOpts != nil {
				if c, ok := s.NavOpts["code"].(string); ok {
					code = c
				}
			}
			s.NavTarget = ""
			s.NavOpts = nil
			s.MeetingCode = code
			s.mu.Unlock()

			if code == "" {
				return fmt.Errorf("meeting code is required")
			}

			meetURL := fmt.Sprintf("https://meet.google.com/%s", code)
			fmt.Printf("Joining meeting: %s\n", meetURL)

			err := chromedp.Run(s.TargetCtx,
				chromedp.Navigate(meetURL),
				chromedp.Sleep(4*time.Second),
			)
			if err != nil {
				return fmt.Errorf("failed to navigate: %w", err)
			}

			var buf []byte
			var html string
			err = chromedp.Run(s.TargetCtx,
				chromedp.FullScreenshot(&buf, 100),
				chromedp.OuterHTML("html", &html),
			)
			if err == nil {
				os.WriteFile("meeting_dump.png", buf, 0644)
				os.WriteFile("meeting_dump.html", []byte(html), 0644)
				fmt.Println("Saved meeting_dump.png and meeting_dump.html")
			} else {
				fmt.Printf("Failed to capture meeting dumps: %v\n", err)
			}

			return nil
		},
	}

	JoinMeetingNode = &Node{
		Name: "Join Meeting Page",
		PreCheck: func(s *StateContext) bool {
			s.mu.Lock()
			target := s.NavTarget
			s.mu.Unlock()
			if target != "" {
				return false
			}

			var urlStr string
			err := chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			if err != nil {
				return false
			}
			u, err := url.Parse(urlStr)
			if err != nil || u.Host != "meet.google.com" || u.Path == "/" || u.Path == "/new" || u.Path == "" || strings.HasPrefix(u.Path, "/_meet/") {
				return false
			}

			var hasJoinBtn bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
				(function() {
					let btns = Array.from(document.querySelectorAll('button, div[role="button"], span'));
					return btns.some(b => b.innerText && (b.innerText.toLowerCase().includes('join now') || b.innerText.toLowerCase().includes('ask to join') || b.innerText.toLowerCase().includes('join anyway')) && b.offsetWidth > 0 && b.offsetHeight > 0);
				})();
			`, &hasJoinBtn))
			return hasJoinBtn
		},
		Work: func(s *StateContext) error {
			deadline := time.Now().Add(15 * time.Second)
			var res string
			for time.Now().Before(deadline) {
				err := chromedp.Run(s.TargetCtx,
					chromedp.Evaluate(`
						(function() {
							let btns = Array.from(document.querySelectorAll('button, div[role="button"], span'));
							let joinBtn = btns.find(b => b.innerText && (b.innerText.toLowerCase().includes('join now') || b.innerText.toLowerCase().includes('ask to join') || b.innerText.toLowerCase().includes('join anyway')) && b.offsetWidth > 0 && b.offsetHeight > 0);
							if (joinBtn) {
								let clickable = joinBtn.closest('button') || joinBtn.closest('div[role="button"]') || joinBtn;
								if (clickable.disabled || clickable.getAttribute('aria-disabled') === 'true') {
									return "disabled";
								}
								clickable.id = 'bot-join-button';
								return "found";
							}
							return "not found";
						})();
					`, &res),
				)
				if err == nil && res == "found" {
					break
				}
				time.Sleep(1 * time.Second)
			}

			if res != "found" {
				return fmt.Errorf("join button not found or enabled in time, last state: %s", res)
			}

			err := chromedp.Run(s.TargetCtx,
				chromedp.Click(`#bot-join-button`, chromedp.ByQuery),
				chromedp.Sleep(3*time.Second),
			)
			if err != nil {
				return fmt.Errorf("failed to click join button: %w", err)
			}
			return nil
		},
	}

	LeaveMeetingNode = &Node{
		Name: "Leave Meeting",
		PreCheck: func(s *StateContext) bool {
			s.mu.Lock()
			defer s.mu.Unlock()
			return s.NavTarget == "LeaveMeeting"
		},
		Work: func(s *StateContext) error {
			s.mu.Lock()
			s.NavTarget = ""
			s.mu.Unlock()

			var clicked bool
			err := chromedp.Run(s.TargetCtx, chromedp.Evaluate(`
				(function() {
					let btn = document.querySelector('button[aria-label*="leave" i], button[aria-label*="hang" i]');
					if (btn) {
						btn.click();
						return true;
					}
					return false;
				})();
			`, &clicked))

			if err != nil {
				return fmt.Errorf("failed to click leave button: %w", err)
			}

			if !clicked {
				return fmt.Errorf("leave button not found")
			}

			time.Sleep(2 * time.Second)
			return nil
		},
	}
}
