package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"


	"github.com/gorilla/websocket"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/browser"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/calendarclient"
)

var (
	nodeWorkDuration  metric.Float64Histogram
	nodePhaseDuration metric.Float64Histogram
	nodeVisits        metric.Int64Counter
	nodeRestIdleTime  metric.Float64Histogram
)

func init() {
	meter := otel.Meter("display_server/setup")
	var err error
	nodeWorkDuration, err = meter.Float64Histogram("node.work.duration", metric.WithDescription("Execution time of work phase in ms"), metric.WithUnit("ms"))
	if err != nil { panic(err) }
	nodePhaseDuration, err = meter.Float64Histogram("node.phase.duration", metric.WithDescription("Execution time of different phases"), metric.WithUnit("ms"))
	if err != nil { panic(err) }
	nodeVisits, err = meter.Int64Counter("node.visits", metric.WithDescription("Frequency of specific states"))
	if err != nil { panic(err) }
	nodeRestIdleTime, err = meter.Float64Histogram("node.rest.idle_time", metric.WithDescription("Idle time in rest nodes"), metric.WithUnit("s"))
	if err != nil { panic(err) }
}

type StateContext struct {
	mu          sync.Mutex
	CurrentNode string
	Phase       string
	SetupPhase  int

	Browser        browser.Browser
	CalendarClient calendarclient.CalendarClient
	Ctx            context.Context
	Clock          Clock

	Mux *http.ServeMux

	DirFlag     string
	PortFlag    string
	EncKey      string
	OauthDir    string
	MeetingCode string


	DefaultNode *Node
	NavTarget   string
	NavOpts     map[string]interface{}
	LogsDir     string
	StepCounter int
	NodeTimeout time.Duration

	RegisteredRoutes map[string]bool
	Email            string
	PasswordChan     chan string
	DisplayActive    bool
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
			slog.Error("Error broadcasting to WS", "error", err)
			conn.Close()
			delete(s.WSConns, conn)
		}
	}
}

// WriteWSJSON safely writes a JSON message to a websocket connection using the context's mutex
func (s *StateContext) WriteWSJSON(conn *websocket.Conn, v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return conn.WriteJSON(v)
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
	if s.Browser == nil {
		return
	}
	slog.Info("Capturing artifacts", "step", stepName, "phase", phase)
	var html string
	var screenshotBuf []byte

	errHTML := s.Browser.OuterHTML("html", &html)
	errImg := s.Browser.CaptureScreenshot(&screenshotBuf)
	if errHTML != nil || errImg != nil {
		slog.Warn("Failed to capture artifacts", "step", stepName, "errHTML", errHTML, "errImg", errImg)
		return
	}

	logsDir := s.LogsDir
	if logsDir == "" {
		logsDir = "logs"
	}
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		slog.Warn("Failed to create logs dir", "error", err)
	}

	htmlFile := fmt.Sprintf("%s/%04d_%s%s_%s_dump.html", logsDir, s.StepCounter, prefix, stepName, phase)
	imgFile := fmt.Sprintf("%s/%04d_%s%s_%s_screenshot.png", logsDir, s.StepCounter, prefix, stepName, phase)

	os.WriteFile(htmlFile, []byte(html), 0644)
	os.WriteFile(imgFile, screenshotBuf, 0644)
	slog.Info("Saved artifacts", "html", htmlFile, "img", imgFile)
}

func RunEngine(startNode *Node, s *StateContext) {
	if s.Clock == nil {
		s.Clock = &RealClock{}
	}
	currentNode := startNode
	if s.DefaultNode == nil {
		s.DefaultNode = startNode
	}
	for {
		s.StepCounter++
		slog.Info("Executing Node", "step", s.StepCounter, "node", currentNode.Name)
		s.SetNodeName(currentNode.Name)
		
		nodeVisits.Add(context.Background(), 1, metric.WithAttributes(attribute.String("node_name", currentNode.Name)))
		nodeStartTime := time.Now()

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
			start := time.Now()
			err := currentNode.Setup(s)
			nodePhaseDuration.Record(context.Background(), float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("node_name", currentNode.Name), attribute.String("phase", "setup")))
			if err != nil {
				slog.Error("Setup failed", "node", currentNode.Name, "error", err)
			}
		}

		if s.Browser != nil && currentNode.Name != "Init Server" && currentNode.Name != "Auth Phase" && currentNode.Name != "Calendar Logic Phase" {
			captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_"), "pre", "display_")
		}

		s.SetPhase("pre-work")
		if currentNode.Work != nil {
			s.SetPhase("work")
			start := time.Now()
			err := currentNode.Work(s)
			durationMs := float64(time.Since(start).Milliseconds())
			nodePhaseDuration.Record(context.Background(), durationMs, metric.WithAttributes(attribute.String("node_name", currentNode.Name), attribute.String("phase", "work")))
			nodeWorkDuration.Record(context.Background(), durationMs, metric.WithAttributes(attribute.String("node_name", currentNode.Name)))
			if err != nil {
				slog.Error("Work failed", "node", currentNode.Name, "error", err)
				if currentNode == startNode {
					s.Clock.Sleep(2 * time.Second)
					continue
				}
			}
		}

		if s.Browser != nil && currentNode.Name != "Init Server" && currentNode.Name != "Auth Phase" && currentNode.Name != "Calendar Logic Phase" {
			captureDebugArtifacts(s, strings.ReplaceAll(strings.ToLower(currentNode.Name), " ", "_"), "post", "display_")
		}

		s.SetPhase("done-check")
		if currentNode.DoneCheck != nil {
			start := time.Now()
			err := currentNode.DoneCheck(s)
			nodePhaseDuration.Record(context.Background(), float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("node_name", currentNode.Name), attribute.String("phase", "done_check")))
			if err != nil {
				slog.Error("DoneCheck failed", "node", currentNode.Name, "error", err)
				currentNode = s.DefaultNode
				continue
			}
		}

		s.SetPhase("transitioning")
		if len(currentNode.Next) == 0 {
			slog.Info("Flow finished successfully at terminal node", "node", currentNode.Name)
			return
		}

		slog.Info("Finding next node", "possibilities", len(currentNode.Next))
		var nextNode *Node

		var timeout time.Duration
		if currentNode.IsRestNode {
			timeout = 0
		} else if s.NodeTimeout > 0 {
			timeout = s.NodeTimeout
		} else {
			timeout = 5 * time.Minute
		}
		deadline := s.Clock.Now().Add(timeout)

		for nextNode == nil {
			if !currentNode.IsRestNode && s.Clock.Now().After(deadline) {
				slog.Warn("Non-rest node timeout reached", "timeout", timeout)
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
				slog.Info("Selected next node", "node", nextNode.Name)
				break
			}

			// If no next node matched, check if the current rest node is still valid
			if currentNode.IsRestNode {
				if currentNode.RestNodeValidation != nil {
					if !currentNode.RestNodeValidation(s) {
						slog.Info("Rest node condition no longer valid, transitioning to default node", "node", currentNode.Name)
						nextNode = s.DefaultNode
						break
					}
				} else if currentNode.PreCheck != nil {
					if !currentNode.PreCheck(s) {
						slog.Info("Rest node condition no longer valid, transitioning to default node", "node", currentNode.Name)
						nextNode = s.DefaultNode
						break
					}
				}
			}

			s.Clock.Sleep(500 * time.Millisecond)
		}

		if nextNode == nil {
			slog.Error("No valid next path found! Restarting flow.")
			nextNode = s.DefaultNode
		}

		if currentNode.Teardown != nil {
			start := time.Now()
			err := currentNode.Teardown(s)
			nodePhaseDuration.Record(context.Background(), float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("node_name", currentNode.Name), attribute.String("phase", "teardown")))
			if err != nil {
				slog.Error("Teardown failed", "node", currentNode.Name, "error", err)
			}
		}

		if currentNode.IsRestNode {
			nodeRestIdleTime.Record(context.Background(), time.Since(nodeStartTime).Seconds(), metric.WithAttributes(attribute.String("node_name", currentNode.Name)))
		}

		currentNode = nextNode
	}
}
