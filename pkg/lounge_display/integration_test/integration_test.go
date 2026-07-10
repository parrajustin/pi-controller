package integration_test

import (
	"context"
	"encoding/base64"
	"os"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/parrajustin/pi-controller/pkg/lounge_display/browser"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/display_server/setup"
)

func TestNodesWithMockHTML(t *testing.T) {
	tests := []struct {
		name          string
		mockFile      string
		urlToNavigate string
		testFunc      func(t *testing.T, s *setup.StateContext)
	}{
		{
			name:          "JoinMeetingNode.PreCheck detects join button",
			mockFile:      "dumps/join_meeting_page.html",
			urlToNavigate: "https://meet.google.com/abc-defg-hij",
			testFunc: func(t *testing.T, s *setup.StateContext) {
				// We expect PreCheck to return true when a join button is found
				isValid := setup.JoinMeetingNode.PreCheck(s)
				assert.True(t, isValid, "PreCheck should return true because the join button is present")
			},
		},
		{
			name:          "InMeetingNode.PreCheck detects we are in a meeting",
			mockFile:      "dumps/in_meeting_page.html",
			urlToNavigate: "https://meet.google.com/abc-defg-hij",
			testFunc: func(t *testing.T, s *setup.StateContext) {
				// InMeetingNode PreCheck returns true when there is NO join button 
				// (meaning we have entered the meeting)
				isValid := setup.InMeetingNode.PreCheck(s)
				assert.True(t, isValid, "PreCheck should return true because we are in the meeting (no join button)")
			},
		},
		{
			name:          "MeetLandingPageNode.RestNodeValidation detects landing page",
			mockFile:      "dumps/landing_page.html",
			urlToNavigate: "https://meet.google.com/landing",
			testFunc: func(t *testing.T, s *setup.StateContext) {
				// This node checks if the URL matches /landing
				isValid := setup.MeetLandingPageNode.RestNodeValidation(s)
				assert.True(t, isValid, "RestNodeValidation should return true on the landing page")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			htmlContent, err := os.ReadFile(tc.mockFile)
			require.NoError(t, err, "Failed to read mock file %s", tc.mockFile)

			opts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", true),
			)
			allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
			defer cancelAlloc()

			ctx, cancelCtx := chromedp.NewContext(allocCtx)
			defer cancelCtx()

			// Intercept fetch requests
			err = chromedp.Run(ctx, fetch.Enable().WithPatterns([]*fetch.RequestPattern{
				{URLPattern: "*meet.google.com*", RequestStage: fetch.RequestStageResponse},
			}))
			require.NoError(t, err)

			chromedp.ListenTarget(ctx, func(ev interface{}) {
				if ev, ok := ev.(*fetch.EventRequestPaused); ok {
					go func() {
						c := chromedp.FromContext(ctx)
						
						// Intercept and return our mock HTML
						if err := fetch.FulfillRequest(ev.RequestID, 200).
							WithBody(base64.StdEncoding.EncodeToString(htmlContent)).
							WithResponseHeaders([]*fetch.HeaderEntry{
								{Name: "Content-Type", Value: "text/html"},
							}).
							Do(cdp.WithExecutor(ctx, c.Target)); err != nil {
							t.Logf("Failed to fulfill request: %v", err)
						}
					}()
				}
			})

			// Navigate to the target URL to trigger the interceptor
			err = chromedp.Run(ctx, chromedp.Navigate(tc.urlToNavigate))
			require.NoError(t, err)

			// Wait a bit for the page to evaluate and load our mock DOM
			time.Sleep(1 * time.Second)

			// Create state context
			s := &setup.StateContext{
				Ctx:     ctx,
				Browser: browser.NewRealBrowser(ctx),
			}

			// Run the test validation
			tc.testFunc(t, s)
		})
	}
}
