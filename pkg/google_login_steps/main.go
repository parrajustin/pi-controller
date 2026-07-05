package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type StateContext struct {
	Ctx         context.Context
	TargetCtx   context.Context
	Email       string
	StepCounter int
	LogsDir     string
}

type Node struct {
	Name      string
	PreCheck  func(s *StateContext) bool
	Work      func(s *StateContext) error
	DoneCheck func(s *StateContext) error
	Next      []*Node
}

func captureDebugArtifacts(s *StateContext, stepName string) {
	fmt.Printf("Capturing artifacts for step %04d: %s...\n", s.StepCounter, stepName)
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

	htmlFile := fmt.Sprintf("%s/%04d_%s_dump.html", s.LogsDir, s.StepCounter, stepName)
	imgFile := fmt.Sprintf("%s/%04d_%s_screenshot.png", s.LogsDir, s.StepCounter, stepName)

	os.WriteFile(htmlFile, []byte(html), 0644)
	os.WriteFile(imgFile, screenshotBuf, 0644)
	fmt.Printf("Saved %s and %s\n", htmlFile, imgFile)
}

func RunEngine(startNode *Node, s *StateContext) {
	currentNode := startNode
	for {
		s.StepCounter++
		fmt.Printf("\n=== Executing Node %04d: %s ===\n", s.StepCounter, currentNode.Name)
		
		if currentNode.Work != nil {
			err := currentNode.Work(s)
			if err != nil {
				log.Printf("Work failed for node %s: %v\n", currentNode.Name, err)
			}
		}

		if currentNode.DoneCheck != nil {
			err := currentNode.DoneCheck(s)
			if err != nil {
				log.Printf("DoneCheck failed for node %s: %v\n", currentNode.Name, err)
				captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_")+"_failed")
				fmt.Println("ERROR: Node failed its DoneCheck. The engine is lost.")
				fmt.Println("Please check the dumps and restart.")
				fmt.Println("Press Enter to restart from the beginning...")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				currentNode = startNode
				continue
			}
		}

		captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_"))

		if len(currentNode.Next) == 0 {
			fmt.Println("\nFlow finished successfully at terminal node:", currentNode.Name)
			return
		}

		fmt.Printf("Finding next node from %d possibilities...\n", len(currentNode.Next))
		var nextNode *Node
		
		deadline := time.Now().Add(10 * time.Second) // Wait up to 10 seconds for a path
		for time.Now().Before(deadline) && nextNode == nil {
			for _, n := range currentNode.Next {
				if n.PreCheck(s) {
					nextNode = n
					break
				}
			}
			if nextNode == nil {
				time.Sleep(500 * time.Millisecond)
			}
		}

		if nextNode == nil {
			captureDebugArtifacts(s, "engine_lost_error")
			fmt.Println("\nERROR: No valid next path found! The engine is lost.")
			fmt.Println("Please check the dumps and restart.")
			fmt.Println("Press Enter to restart from the beginning...")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			currentNode = startNode
			continue
		}

		currentNode = nextNode
	}
}

func PrintMermaidGraph(startNode *Node) {
	fmt.Println("graph TD")
	
	visited := make(map[string]bool)
	var queue []*Node
	
	queue = append(queue, startNode)
	visited[startNode.Name] = true
	
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		
		currID := strings.ReplaceAll(curr.Name, " ", "_")
		
		for _, nextNode := range curr.Next {
			nextID := strings.ReplaceAll(nextNode.Name, " ", "_")
			fmt.Printf("    %s[\"%s\"] --> %s[\"%s\"]\n", currID, curr.Name, nextID, nextNode.Name)
			
			if !visited[nextNode.Name] {
				visited[nextNode.Name] = true
				queue = append(queue, nextNode)
			}
		}
		
		if len(curr.Next) == 0 {
			fmt.Printf("    %s[\"%s\"]:::terminal\n", currID, curr.Name)
		}
	}
	fmt.Println("classDef terminal fill:#f9f,stroke:#333,stroke-width:2px;")
}

// Global node pointers so they can reference each other
var (
	StartNode                *Node
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

func initNodes() {
	StartNode = &Node{
		Name: "Start",
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
			fmt.Println("Already logged in, staying on Meet.")
			return nil
		},
		Next: []*Node{}, // terminal
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
			fmt.Println("Attempting to click the 'Sign in' button...")
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
		Work: nil, // Just a branch node
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
		Work: nil,
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
			fmt.Println("Clicking the existing account option...")
			var res string
			return chromedp.Run(s.TargetCtx,
				chromedp.Evaluate(`
					(function() {
						let els = Array.from(document.querySelectorAll('div'));
						let el = els.find(e => e.innerText && e.innerText.includes("`+s.Email+`") && e.getAttribute("data-identifier"));
						if (!el) {
							// fallback just in case data-identifier is missing but text matches
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
			fmt.Println("Clicking 'Use another account'...")
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
			fmt.Println("Typing email and clicking Next...")
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
		PreCheck: func(s *StateContext) bool {
			var exists bool
			chromedp.Run(s.TargetCtx, chromedp.Evaluate(`document.querySelector('input[type="password"]') !== null && document.querySelector('input[type="password"]').offsetWidth > 0`, &exists))
			return exists
		},
		Work: func(s *StateContext) error {
			fmt.Print("Enter password for " + s.Email + ": ")
			scanner := bufio.NewScanner(os.Stdin)
			var password string
			if scanner.Scan() {
				password = scanner.Text()
			}
			fmt.Println("Typing password and clicking Next...")
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
			if errorText != "" {
				fmt.Printf("\nPassword error detected: %s\n", errorText)
				return true
			}
			return false
		},
		Work: func(s *StateContext) error {
			fmt.Println("Password was wrong. Returning to password input...")
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
		Work: func(s *StateContext) error {
			fmt.Println("Login successful or pending 2FA.")
			return nil
		},
		DoneCheck: func(s *StateContext) error {
			fmt.Println("Polling for 2FA completion (up to 10 minutes)...")
			deadline := time.Now().Add(10 * time.Minute)
			for time.Now().Before(deadline) {
				var urlStr string
				chromedp.Run(s.TargetCtx, chromedp.Location(&urlStr))
				u, err := url.Parse(urlStr)
				if err == nil && (u.Host == "meet.google.com" || u.Host == "workspace.google.com") {
					fmt.Println("\nSuccessfully completed 2FA!")
					return nil
				}
				fmt.Print(".")
				time.Sleep(15 * time.Second)
			}
			fmt.Println()
			return fmt.Errorf("timed out waiting for 2FA completion")
		},
		Next: []*Node{FinishedMeetNode, WorkspaceRedirectedNode},
	}

	// Link nodes
	StartNode.Next = []*Node{WorkspaceRedirectedNode, FinishedMeetNode}
	WorkspaceRedirectedNode.Next = []*Node{AccountsGooglePageNode}
	
	// accounts.google.com can lead to choose account OR directly to email input
	AccountsGooglePageNode.Next = []*Node{ChooseAccountNode, EmailInputNode, PasswordInputNode}
	
	ChooseAccountNode.Next = []*Node{AccountOptionExistsNode, AccountOptionMissingNode}
	AccountOptionExistsNode.Next = []*Node{PasswordInputNode}
	AccountOptionMissingNode.Next = []*Node{EmailInputNode}
	
	EmailInputNode.Next = []*Node{PasswordInputNode}
	
	PasswordInputNode.Next = []*Node{WrongPasswordNode, TwoFactorNode}
	WrongPasswordNode.Next = []*Node{PasswordInputNode}
}

func main() {
	graphFlag := flag.Bool("graph", false, "Print the state machine graph in Mermaid format and exit")
	logsDirFlag := flag.String("logs-dir", "logs", "Directory to store HTML dumps and screenshots")
	flag.Parse()

	if *graphFlag {
		initNodes()
		PrintMermaidGraph(StartNode)
		return
	}

	if err := os.RemoveAll(*logsDirFlag); err != nil {
		log.Printf("Warning: Failed to clear logs directory: %v\n", err)
	}
	if err := os.MkdirAll(*logsDirFlag, 0755); err != nil {
		log.Printf("Warning: Failed to create logs directory: %v\n", err)
	}

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

	targetCtx, cancelTarget := chromedp.NewContext(allocCtx, chromedp.WithTargetID(activeTarget.TargetID))
	defer cancelTarget()

	stateCtx := &StateContext{
		Ctx:       ctx,
		TargetCtx: targetCtx,
		Email:     "lounge.room@mountainviewmasoniclodge.com",
		LogsDir:   *logsDirFlag,
	}

	initNodes()

	fmt.Println("Starting State Machine Engine...")
	RunEngine(StartNode, stateCtx)
}
