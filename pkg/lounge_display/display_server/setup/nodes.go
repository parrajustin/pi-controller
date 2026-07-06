package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/cryptoutil"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var (
	InitServerNode   *Node
	AuthNode         *Node
	CalendarNode     *Node
	InitCDPNode      *Node
	StartMeetNode    *Node
	FinishedMeetNode *Node
)

func InitNodes() *Node {
	InitServerNode = &Node{
		Name: "Init Server",
		PreCheck: func(s *StateContext) bool { return true },
		Setup: func(s *StateContext) error {
			fmt.Printf("Server initializing on port %s\n", s.PortFlag)
			return nil
		},
	}

	AuthNode = &Node{
		Name: "Auth Phase",
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			if s.EncKey == "" {
				return fmt.Errorf("TOKEN_ENCRYPTION_KEY is required")
			}

			tokPath := filepath.Join(s.OauthDir, "token.json.enc")
			ciphertext, err := os.ReadFile(tokPath)
			if err != nil {
				return fmt.Errorf("failed to read token: %w", err)
			}

			plaintext, err := cryptoutil.Decrypt(ciphertext, s.EncKey)
			if err != nil {
				return fmt.Errorf("failed to decrypt token: %w", err)
			}

			tok := &oauth2.Token{}
			if err := json.Unmarshal(plaintext, tok); err != nil {
				return fmt.Errorf("failed to unmarshal token: %w", err)
			}

			credPath := filepath.Join(s.OauthDir, "credentials.json")
			credBytes, err := os.ReadFile(credPath)
			if err != nil {
				return fmt.Errorf("unable to read credentials file: %w", err)
			}

			config, err := google.ConfigFromJSON(credBytes, calendar.CalendarReadonlyScope)
			if err != nil {
				return fmt.Errorf("unable to parse client secret file to config: %w", err)
			}

			ctx := context.Background()
			client := config.Client(ctx, tok)
			srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
			if err != nil {
				return fmt.Errorf("unable to retrieve Calendar client: %w", err)
			}

			s.CalendarSrv = srv
			return nil
		},
	}

	CalendarNode = &Node{
		Name: "Calendar Logic Phase",
		PreCheck: func(s *StateContext) bool { return true },
		Work: func(s *StateContext) error {
			t := time.Now().Format(time.RFC3339)
			_, err := s.CalendarSrv.Events.List("primary").ShowDeleted(false).
				SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
			if err != nil {
				return fmt.Errorf("unable to retrieve calendar events: %w", err)
			}
			fmt.Println("Calendar events fetched successfully!")
			return nil
		},
	}

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
			err := chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
			if err != nil {
				return false
			}
			u, err := url.Parse(urlStr)
			return err == nil && u.Host == "meet.google.com"
		},
		Work: func(s *StateContext) error {
			fmt.Println("Successfully on Meet. Assuming logged in.")
			return nil
		},
	}

	// Link nodes
	InitServerNode.Next = []*Node{AuthNode}
	AuthNode.Next = []*Node{CalendarNode}
	CalendarNode.Next = []*Node{InitCDPNode}
	InitCDPNode.Next = []*Node{StartMeetNode}
	StartMeetNode.Next = []*Node{FinishedMeetNode}

	return InitServerNode
}
