package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gorilla/websocket"
	"google.golang.org/api/calendar/v3"
)

type StateContext struct {
	mu          sync.Mutex
	CurrentNode string
	Phase       string
	SetupPhase  int

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

	RegisteredRoutes map[string]bool
	Email            string
	PasswordChan     chan string
	ReceiverFlag     string
	SetupReady       bool

	WSConns    map[*websocket.Conn]bool
	WSHandlers map[string]func(payload json.RawMessage) (interface{}, error)
}

func (s *StateContext) AddWSHandler(msgType string, handler func(payload json.RawMessage) (interface{}, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.WSHandlers == nil {
		s.WSHandlers = make(map[string]func(payload json.RawMessage) (interface{}, error))
	}
	s.WSHandlers[msgType] = handler
}

func (s *StateContext) RemoveWSHandler(msgType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.WSHandlers != nil {
		delete(s.WSHandlers, msgType)
	}
}

func (s *StateContext) GetWSHandler(msgType string) (func(payload json.RawMessage) (interface{}, error), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.WSHandlers == nil {
		return nil, false
	}
	handler, ok := s.WSHandlers[msgType]
	return handler, ok
}

func (s *StateContext) AddWSConn(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.WSConns == nil {
		s.WSConns = make(map[*websocket.Conn]bool)
	}
	s.WSConns[conn] = true
}

func (s *StateContext) RemoveWSConn(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.WSConns != nil {
		delete(s.WSConns, conn)
	}
}

func (s *StateContext) BroadcastState() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.WSConns == nil || len(s.WSConns) == 0 {
		return
	}
	
	state := map[string]interface{}{
		"current_node": s.CurrentNode,
		"meeting_code": s.MeetingCode,
		"setup_ready":  s.SetupReady,
		"phase":        s.Phase,
		"setup_phase":  s.SetupPhase,
	}
	msg := map[string]interface{}{
		"type":    "state_update",
		"payload": state,
	}
	
	for conn := range s.WSConns {
		err := conn.WriteJSON(msg)
		if err != nil {
			log.Printf("Error broadcasting to WS: %v", err)
			conn.Close()
			delete(s.WSConns, conn)
		}
	}
}

func (s *StateContext) SetNodeName(name string) {
	s.mu.Lock()
	s.CurrentNode = name
	s.mu.Unlock()
	s.BroadcastState()
}
func (s *StateContext) GetNodeName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.CurrentNode
}

func (s *StateContext) SetPhase(phase string) {
	s.mu.Lock()
	s.Phase = phase
	s.mu.Unlock()
	s.BroadcastState()
}

func (s *StateContext) SetSetupPhase(phase int) {
	s.mu.Lock()
	s.SetupPhase = phase
	s.mu.Unlock()
	s.BroadcastState()
}

func (s *StateContext) GetPhase() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Phase
}

func (s *StateContext) GetSetupPhase() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.SetupPhase
}


func (s *StateContext) SetNavTarget(target string, opts map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.NavTarget = target
	s.NavOpts = opts
}

func RegisterRoute(s *StateContext, path string, handler http.HandlerFunc) {
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

func (s *StateContext) GetSetupReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.SetupReady
}

func (s *StateContext) SetSetupReady(ready bool) {
	s.mu.Lock()
	s.SetupReady = ready
	s.mu.Unlock()
	s.BroadcastState()
}

type Node struct {
	Name               string
	IsRestNode         bool
	RestNodeValidation func(s *StateContext) bool
	Setup              func(s *StateContext) error
	PreCheck           func(s *StateContext) bool
	Work               func(s *StateContext) error
	DoneCheck          func(s *StateContext) error
	Teardown           func(s *StateContext) error
	Next               []*Node
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

		// If we reach a state where the display is ready to show meeting controls, set SetupReady to true
		if currentNode.Name == "Meet Landing Page" ||
			currentNode.Name == "In Meeting" ||
			currentNode.Name == "NavigateToMeeting" ||
			currentNode.Name == "Join Meeting Page" ||
			currentNode.Name == "Leave Meeting" {
			s.SetSetupReady(true)
		}

		s.SetPhase("pre-setup")
		if currentNode.Setup != nil {
			s.SetPhase("setup")
			err := currentNode.Setup(s)
			if err != nil {
				log.Printf("Setup failed for node %s: %v\n", currentNode.Name, err)
			}
		}

		if s.TargetCtx != nil && currentNode.Name != "Init Server" && currentNode.Name != "Auth Phase" && currentNode.Name != "Calendar Logic Phase" {
			captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_"), "pre", "display_")
		}

		s.SetPhase("pre-work")
		if currentNode.Work != nil {
			s.SetPhase("work")
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

		s.SetPhase("done-check")
		if currentNode.DoneCheck != nil {
			err := currentNode.DoneCheck(s)
			if err != nil {
				log.Printf("DoneCheck failed for node %s: %v\n", currentNode.Name, err)
				currentNode = s.DefaultNode
				continue
			}
		}

		s.SetPhase("transitioning")
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
				log.Printf("Selected next node: %s\n", nextNode.Name)
				break
			}

			// If no next node matched, check if the current rest node is still valid
			if currentNode.IsRestNode {
				if currentNode.RestNodeValidation != nil {
					if !currentNode.RestNodeValidation(s) {
						fmt.Printf("Rest node %s condition is no longer valid. Transitioning to default node.\n", currentNode.Name)
						nextNode = s.DefaultNode
						break
					}
				} else if currentNode.PreCheck != nil {
					if !currentNode.PreCheck(s) {
						fmt.Printf("Rest node %s condition is no longer valid. Transitioning to default node.\n", currentNode.Name)
						nextNode = s.DefaultNode
						break
					}
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
