package setup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

var testUpgrader = websocket.Upgrader{}

func setupLifecycleWSTest(t *testing.T, s *StateContext) (*websocket.Conn, *httptest.Server) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal("upgrade:", err)
			return
		}
		s.AddWSConn(c)
	}))

	url := "ws" + strings.TrimPrefix(ts.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal("dial:", err)
	}

	return clientConn, ts
}

func TestEngine_StateUpdates(t *testing.T) {
	s, _, _, _, _ := setupTestContext("../../logs")
	clientConn, ts := setupLifecycleWSTest(t, s)
	defer ts.Close()
	defer clientConn.Close()

	time.Sleep(50 * time.Millisecond)

	testNode := &Node{
		Name: "LifecycleNode",
		Work: func(ctx *StateContext) error { return nil },
	}

	go RunEngine(testNode, s)

	phasesSeen := make(map[string]bool)
	clientConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	for {
		var msg map[string]interface{}
		err := clientConn.ReadJSON(&msg)
		if err != nil {
			break
		}
		if msg["type"] == "state_update" {
			payload := msg["payload"].(map[string]interface{})
			if phase, ok := payload["phase"].(string); ok {
				phasesSeen[phase] = true
			}
		}
	}

	assert.True(t, phasesSeen["pre-setup"])
	assert.True(t, phasesSeen["pre-work"])
	assert.True(t, phasesSeen["work"])
	assert.True(t, phasesSeen["transitioning"])
}

func TestEngine_WSTeardownOnTransition(t *testing.T) {
	s, _, _, _, fakeClock := setupTestContext("../../logs")
	
	node2 := &Node{
		Name: "Node2",
		PreCheck: func(ctx *StateContext) bool { return true },
	}

	node1 := &Node{
		Name: "Node1",
		Setup: func(ctx *StateContext) error {
			ctx.AddWSHandler("node1_api", func(payload json.RawMessage) (interface{}, error) {
				return nil, nil
			})
			return nil
		},
		Teardown: func(ctx *StateContext) error {
			ctx.RemoveWSHandler("node1_api")
			return nil
		},
		Next: []*Node{node2},
	}

	go RunEngine(node1, s)
	time.Sleep(10 * time.Millisecond) // Let goroutine start
	fakeClock.Advance(10 * time.Millisecond) // Wake any sleepers
	time.Sleep(10 * time.Millisecond) // Let goroutine transition

	// After running, the engine should have transitioned to Node2 and torn down Node1
	_, ok := s.GetWSHandler("node1_api")
	assert.False(t, ok, "node1_api should have been removed during transition to Node2")
}

func TestEngine_WSTeardownOnTimeout(t *testing.T) {
	s, _, _, _, fakeClock := setupTestContext("../../logs")
	s.NodeTimeout = 100 * time.Millisecond // very short timeout
	
	defaultNode := &Node{
		Name: "Default",
	}
	s.DefaultNode = defaultNode

	timeoutNode := &Node{
		Name: "TimeoutNode",
		Setup: func(ctx *StateContext) error {
			ctx.AddWSHandler("timeout_api", func(payload json.RawMessage) (interface{}, error) {
				return nil, nil
			})
			return nil
		},
		Teardown: func(ctx *StateContext) error {
			ctx.RemoveWSHandler("timeout_api")
			return nil
		},
		Next: []*Node{{
			Name: "Unreachable",
			PreCheck: func(ctx *StateContext) bool { return false },
		}}, // Prevents it from returning immediately as a terminal node
	}

	go RunEngine(timeoutNode, s)
	time.Sleep(10 * time.Millisecond) // Let goroutine start
	fakeClock.Advance(600 * time.Millisecond) // Let it hit the timeout
	time.Sleep(10 * time.Millisecond) // Let goroutine transition

	// After running, the engine should have timed out and reverted to Default Node
	// Most importantly, Teardown MUST have been called!
	_, ok := s.GetWSHandler("timeout_api")
	assert.False(t, ok, "timeout_api should have been removed during default timeout transition")
	assert.Equal(t, "Default", s.GetNodeName())
}

func TestEngine_VerifyAllNodesHandlers(t *testing.T) {
	s, _, _, _, _ := setupTestContext("../../logs")
	
	// Create map to trace endpoints
	endpointMap := map[*Node][]string{
		AuthTokenNode:       {"get_auth_url", "submit_token"},
		PasswordInputNode:   {"submit_password"},
		MeetLandingPageNode: {"join_meeting"},
		InMeetingNode:       {"button_state", "click_button"},
	}

	// We also know InitNodes adds "has_wifi" to all 13 setup nodes
	InitNodes()
	setupNodes := []*Node{
		InitServerNode, WaitWebServerNode, InitDisplay2CDPNode, WaitForClientCallbackNode, CredentialsNode, AuthTokenNode, CalendarNode,
		WorkspaceRedirectedNode, AccountsGooglePageNode, ChooseAccountNode,
		AccountOptionExistsNode, AccountOptionMissingNode, EmailInputNode,
		PasswordInputNode, WrongPasswordNode, TwoFactorNode,
	}

	for _, n := range setupNodes {
		endpointMap[n] = append(endpointMap[n], "has_wifi")
	}

	for node, expectedEndpoints := range endpointMap {
		if node.Setup != nil {
			err := node.Setup(s)
			assert.NoError(t, err)

			for _, ep := range expectedEndpoints {
				_, ok := s.GetWSHandler(ep)
				assert.True(t, ok, "Expected endpoint %s to be registered by %s", ep, node.Name)
			}
		}

		if node.Teardown != nil {
			err := node.Teardown(s)
			assert.NoError(t, err)

			for _, ep := range expectedEndpoints {
				_, ok := s.GetWSHandler(ep)
				assert.False(t, ok, "Expected endpoint %s to be removed by %s Teardown", ep, node.Name)
			}
		}
	}
}
