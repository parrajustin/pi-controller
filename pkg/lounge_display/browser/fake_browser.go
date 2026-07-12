package browser

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// FakeBrowser is a mock implementation of Browser for testing.
type FakeBrowser struct {
	LogsDir        string
	CurrentState   string // e.g., "0014_display_meet_landing_page_post"
	CurrentURL     string
	HTMLContent    string
	EvaluateMocks    map[string]func(html string) interface{}
	ActionLog        []string
	CustomEvalResult interface{}
}

func NewFakeBrowser(logsDir string) *FakeBrowser {
	return &FakeBrowser{
		LogsDir:       logsDir,
		EvaluateMocks: make(map[string]func(html string) interface{}),
		ActionLog:     make([]string, 0),
	}
}

func (b *FakeBrowser) LoadState(statePrefix string) error {
	b.CurrentState = statePrefix
	
	files, err := os.ReadDir(b.LogsDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), statePrefix) && strings.HasSuffix(file.Name(), ".html") {
			content, err := os.ReadFile(b.LogsDir + "/" + file.Name())
			if err != nil {
				return err
			}
			b.HTMLContent = string(content)
			
			// Guess URL based on state
			if strings.Contains(statePrefix, "meet_landing_page") {
				b.CurrentURL = "https://meet.google.com/landing"
			} else if strings.Contains(statePrefix, "in_meeting") {
				b.CurrentURL = "https://meet.google.com/abc-defg-hij"
			} else if strings.Contains(statePrefix, "accounts_google_page") || strings.Contains(statePrefix, "password_input_page") {
				b.CurrentURL = "https://accounts.google.com/signin/v2/challenge/password"
			}
			return nil
		}
	}
	
	// If no file matches, still update state (useful for error/edge case testing)
	b.HTMLContent = "<html><body>Fake empty body</body></html>"
	return nil
}

func (b *FakeBrowser) Location() (string, error) {
	b.ActionLog = append(b.ActionLog, "Location")
	return b.CurrentURL, nil
}

func (b *FakeBrowser) Evaluate(script string, res interface{}) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("Evaluate: %s", script))
	
	// Simplify matching by looking for keywords in the script
	lowerScript := strings.ToLower(script)
	htmlLower := strings.ToLower(b.HTMLContent)
	
	var result interface{}

	// Handle specific Evaluate calls from our nodes
	if strings.Contains(lowerScript, "accounts.google.com") {
		result = true
	} else if strings.Contains(lowerScript, "choose an account") {
		result = strings.Contains(htmlLower, "choose an account")
	} else if strings.Contains(lowerScript, "use another account") {
		result = strings.Contains(htmlLower, "use another account")
	} else if strings.Contains(lowerScript, "wrong password") || strings.Contains(lowerScript, "incorrect") {
		if strings.Contains(htmlLower, "wrong password") || strings.Contains(htmlLower, "incorrect") {
			result = "wrong password"
		} else {
			result = ""
		}
	} else if strings.Contains(lowerScript, "2-step verification") || strings.Contains(lowerScript, "verifying it") {
		result = strings.Contains(htmlLower, "2-step verification") || strings.Contains(htmlLower, "verifying it")
	} else if strings.Contains(lowerScript, "join now") || strings.Contains(lowerScript, "ask to join") {
		hasBtn := strings.Contains(htmlLower, "join now") || strings.Contains(htmlLower, "ask to join")
		
		if strings.Contains(lowerScript, "bot-join-button") {
			// This is the JoinMeetingNode work script
			if hasBtn {
				result = "found"
			} else {
				result = "not found"
			}
		} else {
			// This is the InMeetingNode/JoinMeetingNode pre-check
			result = hasBtn
		}
	} else if strings.Contains(lowerScript, "button_state") || strings.Contains(lowerScript, "microphone") && strings.Contains(lowerScript, "camera") {
		// button_state WS handler
		result = `{"in_meeting":true,"microphone":false,"camera":false,"hand":false}`
	} else if strings.Contains(lowerScript, "data-identifier") {
		// Email click
		result = "clicked"
	} else if strings.Contains(lowerScript, "leave") {
		result = true // Assume leave button always clicked in tests
	} else if strings.Contains(lowerScript, "sign in") {
		result = "clicked"
	} else {
		// Default to true for unknown boolean checks, or try custom mocks
		for key, mockFunc := range b.EvaluateMocks {
			if strings.Contains(script, key) {
				result = mockFunc(b.HTMLContent)
				break
			}
		}
		if result == nil {
			if b.CustomEvalResult != nil {
				result = b.CustomEvalResult
			} else {
				result = true
			}
		}
	}

	// Assign the result back to res (using reflection or type assertion based on what res is)
	switch v := res.(type) {
	case *bool:
		if bVal, ok := result.(bool); ok {
			*v = bVal
		}
	case *string:
		if sVal, ok := result.(string); ok {
			*v = sVal
		}
	}

	return nil
}

func (b *FakeBrowser) WaitVisible(selector string, byID bool) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("WaitVisible: %s", selector))
	return nil
}

func (b *FakeBrowser) Click(selector string, byID bool) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("Click: %s", selector))
	return nil
}

func (b *FakeBrowser) SendKeys(selector, keys string, byID bool) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("SendKeys: %s %s", selector, keys))
	return nil
}

func (b *FakeBrowser) Sleep(d time.Duration) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("Sleep: %v", d))
	return nil
}

func (b *FakeBrowser) Navigate(url string) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("Navigate: %s", url))
	b.CurrentURL = url
	return nil
}

func (b *FakeBrowser) OuterHTML(selector string, res *string) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("OuterHTML: %s", selector))
	*res = b.HTMLContent
	return nil
}

func (b *FakeBrowser) CaptureScreenshot(res *[]byte) error {
	b.ActionLog = append(b.ActionLog, "CaptureScreenshot")
	*res = []byte("fake_image_data")
	return nil
}

func (b *FakeBrowser) Reload() error {
	b.ActionLog = append(b.ActionLog, "Reload")
	return nil
}

func (b *FakeBrowser) InjectCursor() error {
	b.ActionLog = append(b.ActionLog, "InjectCursor")
	return nil
}

func (b *FakeBrowser) MoveCursor(deltaX, deltaY float64) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("MoveCursor: %f, %f", deltaX, deltaY))
	return nil
}

func (b *FakeBrowser) ClickAtCursor(button string) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("ClickAtCursor: %s", button))
	return nil
}

func (b *FakeBrowser) ScrollAtCursor(deltaY float64) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("ScrollAtCursor: %f", deltaY))
	return nil
}

func (b *FakeBrowser) SendKeyboardInput(keys string) error {
	b.ActionLog = append(b.ActionLog, fmt.Sprintf("SendKeyboardInput: %s", keys))
	return nil
}

func (b *FakeBrowser) HistoryBack() error {
	b.ActionLog = append(b.ActionLog, "HistoryBack")
	return nil
}

func (b *FakeBrowser) HistoryForward() error {
	b.ActionLog = append(b.ActionLog, "HistoryForward")
	return nil
}
