package setup

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"google.golang.org/api/calendar/v3"
)

type StateContext struct {
	mu          sync.Mutex
	CurrentNode string

	Ctx       context.Context
	TargetCtx context.Context

	Mux *http.ServeMux

	DirFlag     string
	PortFlag    string
	EncKey      string
	OauthDir    string
	MeetingCode string

	CalendarSrv *calendar.Service

	DefaultNode *Node
	NavTarget   string
	NavOpts     map[string]interface{}
	LogsDir     string
	StepCounter int
	NodeTimeout time.Duration
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

func (s *StateContext) SetNavTarget(target string, opts map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NavTarget = target
	s.NavOpts = opts
}

type Node struct {
	Name       string
	IsRestNode bool
	Setup      func(s *StateContext) error
	PreCheck   func(s *StateContext) bool
	Work      func(s *StateContext) error
	DoneCheck func(s *StateContext) error
	Teardown  func(s *StateContext) error
	Next      []*Node
}

func captureDebugArtifacts(s *StateContext, stepName, phase, prefix string) {
	if s.TargetCtx == nil {
		return
	}
	fmt.Printf("Capturing artifacts for %s (%s)...\n", stepName, phase)
	var html string
	var screenshotBuf []byte

	captureCtx, cancel := context.WithTimeout(s.TargetCtx, 5*time.Second)
	defer cancel()

	err := chromedp.Run(captureCtx,
		// use ByQuery to wait for something or just OuterHTML
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
		chromedp.CaptureScreenshot(&screenshotBuf),
	)
	if err != nil {
		log.Printf("Warning: Failed to capture artifacts for %s: %v\n", stepName, err)
		return
	}

	logsDir := s.LogsDir
	if logsDir == "" {
		logsDir = "logs"
	}
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		log.Printf("Warning: Failed to create logs dir: %v\n", err)
	}

	htmlFile := fmt.Sprintf("%s/%04d_%s%s_%s_dump.html", logsDir, s.StepCounter, prefix, stepName, phase)
	imgFile := fmt.Sprintf("%s/%04d_%s%s_%s_screenshot.png", logsDir, s.StepCounter, prefix, stepName, phase)

	os.WriteFile(htmlFile, []byte(html), 0644)
	os.WriteFile(imgFile, screenshotBuf, 0644)
	fmt.Printf("Saved %s and %s\n", htmlFile, imgFile)
}

func RunEngine(startNode *Node, s *StateContext) {
	currentNode := startNode
	if s.DefaultNode == nil {
		s.DefaultNode = startNode
	}
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

		if s.TargetCtx != nil && currentNode.Name != "Init Server" && currentNode.Name != "Auth Phase" && currentNode.Name != "Calendar Logic Phase" {
			captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_"), "pre", "display_")
		}

		if currentNode.Work != nil {
			err := currentNode.Work(s)
			if err != nil {
				log.Printf("Work failed for node %s: %v\n", currentNode.Name, err)
				if currentNode == startNode {
					time.Sleep(2 * time.Second)
					continue
				}
			}
		}

		if s.TargetCtx != nil && currentNode.Name != "Init Server" && currentNode.Name != "Auth Phase" && currentNode.Name != "Calendar Logic Phase" {
			captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_"), "post", "display_")
		}

		if currentNode.DoneCheck != nil {
			err := currentNode.DoneCheck(s)
			if err != nil {
				log.Printf("DoneCheck failed for node %s: %v\n", currentNode.Name, err)
				currentNode = s.DefaultNode
				continue
			}
		}

		if len(currentNode.Next) == 0 {
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

			// If no next node matched, check if the current rest node is still valid
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
			currentNode = s.DefaultNode
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
