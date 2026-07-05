package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// StateContext holds shared variables for the state machine
type StateContext struct {
	Ctx       context.Context
	TargetCtx context.Context
	Email     string
}

// StateFunc defines a function signature for a state in our machine
type StateFunc func(s *StateContext) StateFunc

func captureDebugArtifacts(ctx context.Context, stepName string) {
	fmt.Printf("Capturing artifacts for step: %s...\n", stepName)
	var html string
	var screenshotBuf []byte

	// 5-second timeout for capturing debug info
	captureCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := chromedp.Run(captureCtx,
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
		chromedp.CaptureScreenshot(&screenshotBuf),
	)
	if err != nil {
		log.Printf("Warning: Failed to capture artifacts for %s: %v\n", stepName, err)
		return
	}

	htmlFile := fmt.Sprintf("%s_dump.html", stepName)
	imgFile := fmt.Sprintf("%s_screenshot.png", stepName)

	os.WriteFile(htmlFile, []byte(html), 0644)
	os.WriteFile(imgFile, screenshotBuf, 0644)
	fmt.Printf("Saved %s and %s\n", htmlFile, imgFile)
}

func MeetAccessStep(s *StateContext) StateFunc {
	fmt.Println("\n=== STEP: Meet Access ===")
	fmt.Println("Navigating to https://meet.google.com")
	
	err := chromedp.Run(s.TargetCtx,
		chromedp.Navigate("https://meet.google.com"),
		chromedp.Sleep(4*time.Second), // Wait for Google's redirects
	)
	if err != nil {
		log.Printf("Error navigating to meet.google.com: %v\n", err)
		return nil
	}

	captureDebugArtifacts(s.TargetCtx, "meet_access")

	var currentURL string
	chromedp.Run(s.TargetCtx, chromedp.Location(&currentURL))

	if strings.Contains(currentURL, "workspace.google.com") {
		fmt.Println("Redirected to workspace.google.com successfully.")
		return ManageGoogleLoginStep
	}

	if strings.Contains(currentURL, "meet.google.com") {
		fmt.Println("Stayed on meet.google.com. We might already be logged in.")
		return nil
	}

	fmt.Printf("Unexpected URL: %s. Retrying MeetAccessStep...\n", currentURL)
	return MeetAccessStep
}

func ManageGoogleLoginStep(s *StateContext) StateFunc {
	fmt.Println("\n=== STEP: Manage Google Login ===")
	captureDebugArtifacts(s.TargetCtx, "manage_google_login_before")

	var currentURL string
	chromedp.Run(s.TargetCtx, chromedp.Location(&currentURL))

	if !strings.Contains(currentURL, "workspace.google.com") {
		fmt.Println("Not at workspace.google.com. Returning to Meet Access step.")
		return MeetAccessStep
	}

	fmt.Println("Attempting to click the 'Sign in' button...")
	var res string
	err := chromedp.Run(s.TargetCtx,
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
		chromedp.Sleep(4*time.Second),
	)

	if err != nil || res != "clicked" {
		log.Printf("Failed to click 'Sign in' button: err=%v, res=%s\n", err, res)
		return MeetAccessStep
	}

	captureDebugArtifacts(s.TargetCtx, "manage_google_login_after")
	chromedp.Run(s.TargetCtx, chromedp.Location(&currentURL))

	if strings.Contains(currentURL, "accounts.google.com") {
		fmt.Println("Successfully navigated to accounts.google.com.")
		return AccountLoginStep1
	}

	fmt.Printf("Did not navigate to accounts.google.com. Current URL: %s. Restarting.\n", currentURL)
	return MeetAccessStep
}

func AccountLoginStep1(s *StateContext) StateFunc {
	fmt.Println("\n=== STEP: Account Login Step 1 ===")
	captureDebugArtifacts(s.TargetCtx, "account_login_step1_before")

	fmt.Println("Waiting for email input field and clicking it...")
	
	err := chromedp.Run(s.TargetCtx,
		chromedp.WaitVisible(`#identifierId`, chromedp.ByID),
		chromedp.Click(`#identifierId`, chromedp.ByID),
		chromedp.Sleep(1*time.Second), // Give dropdown time to appear
	)
	if err != nil {
		log.Printf("Failed to find or click email input field: %v\n", err)
		return nil
	}

	captureDebugArtifacts(s.TargetCtx, "account_login_step1_dropdown")

	fmt.Println("Typing email and clicking Next to proceed to the passkey/password page...")
	
	err = chromedp.Run(s.TargetCtx,
		chromedp.SendKeys(`#identifierId`, s.Email, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#identifierNext button`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		log.Printf("Failed to enter email and click Next: %v\n", err)
		return nil
	}

	captureDebugArtifacts(s.TargetCtx, "account_login_step1_after_passkey")

	return AccountLoginValidation
}

func AccountLoginValidation(s *StateContext) StateFunc {
	fmt.Println("\n=== STEP: Account Login Validation ===")
	captureDebugArtifacts(s.TargetCtx, "account_login_validation_before")
	
	fmt.Println("Looking for the 'Try another way' button...")
	var res string
	err := chromedp.Run(s.TargetCtx,
		chromedp.Evaluate(`
			(function() {
				let btns = Array.from(document.querySelectorAll('button, div[role="button"], a, span'));
				let btn = btns.find(b => b.innerText && b.innerText.toLowerCase().includes('try another way') && b.offsetWidth > 0);
				if (btn) {
					btn.click();
					return "clicked";
				}
				return "not found";
			})();
		`, &res),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil || res != "clicked" {
		log.Printf("Failed to click 'Try another way': err=%v, res=%s\n", err, res)
		return nil
	}

	captureDebugArtifacts(s.TargetCtx, "account_login_validation_try_another_way")

	fmt.Println("Looking for the 'passkey' option...")
	err = chromedp.Run(s.TargetCtx,
		chromedp.WaitVisible(`//div[@role="link" or @role="button"]//div[contains(text(), "passkey")] | //div[@role="link" and contains(., "passkey")]`, chromedp.BySearch),
		chromedp.Click(`//div[@role="link" or @role="button"]//div[contains(text(), "passkey")] | //div[@role="link" and contains(., "passkey")]`, chromedp.BySearch),
		chromedp.Sleep(3*time.Second),
	)

	if err != nil {
		log.Printf("Failed to click 'passkey' option: err=%v\n", err)
		return nil
	}

	captureDebugArtifacts(s.TargetCtx, "account_login_validation_passkey_prompt")

	fmt.Println("Waiting for the 'Verifying it's you' page...")
	err = chromedp.Run(s.TargetCtx,
		chromedp.WaitVisible(`//div[contains(text(), "Verifying it") or contains(., "Verify")] | //span[contains(text(), "Verifying it")]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),
	)
	
	if err != nil {
		log.Printf("Warning: Could not detect 'Verifying it's you' page explicitly, continuing to monitor URL. err=%v\n", err)
	} else {
		fmt.Println("Reached 'Verifying it's you' page!")
	}
	
	captureDebugArtifacts(s.TargetCtx, "account_login_validation_verifying")

	fmt.Println("Waiting for user to authenticate with passkey...")
	fmt.Println("This may take a while. Monitoring URL changes...")

	// We wait until the URL changes from accounts.google.com to something else (like workspace or meet)
	for i := 0; i < 60; i++ { // wait up to 2 minutes
		var currentURL string
		chromedp.Run(s.TargetCtx, chromedp.Location(&currentURL))
		
		if !strings.Contains(currentURL, "accounts.google.com") {
			fmt.Printf("\nAuthentication successful! Redirected to: %s\n", currentURL)
			break
		}
		
		fmt.Print(".")
		time.Sleep(2 * time.Second)
	}
	fmt.Println()

	captureDebugArtifacts(s.TargetCtx, "account_login_validation_complete")
	
	return nil
}

func main() {
	// Connect to the existing Chrome instance using the debugging port
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9222")
	defer cancelAlloc()

	// Create a temporary context to list targets
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	// Get the list of all targets (tabs)
	targets, err := chromedp.Targets(ctx)
	if err != nil {
		log.Fatalf("Failed to get browser targets: %v", err)
	}

	// Find the active page target
	var activeTarget *target.Info
	for _, t := range targets {
		if t.Type == "page" && !strings.HasPrefix(t.URL, "chrome://") && !strings.HasPrefix(t.URL, "devtools://") {
			activeTarget = t
			break
		}
	}

	if activeTarget == nil {
		log.Fatalf("No active web page found.")
	}

	// Create a new context that attaches directly to the active tab
	// We don't set a global timeout here because the state machine drives the lifecycle
	targetCtx, cancelTarget := chromedp.NewContext(allocCtx, chromedp.WithTargetID(activeTarget.TargetID))
	defer cancelTarget()

	// Initialize the state machine
	stateCtx := &StateContext{
		Ctx:       ctx,
		TargetCtx: targetCtx,
		Email:     "lounge.room@mountainviewmasoniclodge.com",
	}

	fmt.Println("Starting State Machine...")
	var currentState StateFunc = MeetAccessStep

	// Execute states until a state returns nil
	for currentState != nil {
		currentState = currentState(stateCtx)
	}
	
	fmt.Println("\nState machine execution completed.")
}
