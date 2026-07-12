package setup

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

var (
	InitServerNode *Node
)

func InitNodes() *Node {
	InitServerNode = &Node{
		Name: "Init Server",
		PreCheck: func(s *StateContext) bool { return true },
		Setup: func(s *StateContext) error {
			s.DefaultNode = InitServerNode
			slog.Info("Server initializing", "port", s.PortFlag)
			return nil
		},
	}

	// Link nodes together

	// 1. Setup Nodes (from setup_nodes.go)
	InitServerNode.Next = []*Node{WaitWebServerNode}
	WaitWebServerNode.Next = []*Node{InitDisplay2CDPNode}
	InitDisplay2CDPNode.Next = []*Node{WaitForClientCallbackNode}
	WaitForClientCallbackNode.Next = []*Node{CredentialsNode}
	CredentialsNode.Next = []*Node{AuthTokenNode}
	AuthTokenNode.Next = []*Node{CalendarNode}
	CalendarNode.Next = []*Node{InitCDPNode}
	
	WorkspaceRedirectedNode.Next = []*Node{AccountsGooglePageNode}
	AccountsGooglePageNode.Next = []*Node{ChooseAccountNode, EmailInputNode, PasswordInputNode}
	ChooseAccountNode.Next = []*Node{AccountOptionExistsNode, AccountOptionMissingNode}
	AccountOptionExistsNode.Next = []*Node{TwoFactorNode, PasswordInputNode, WrongPasswordNode}
	AccountOptionMissingNode.Next = []*Node{EmailInputNode}
	EmailInputNode.Next = []*Node{PasswordInputNode}
	PasswordInputNode.Next = []*Node{TwoFactorNode, WrongPasswordNode, StartMeetNode}
	WrongPasswordNode.Next = []*Node{PasswordInputNode}
	TwoFactorNode.Next = []*Node{MeetLandingPageNode, CheckInvalidMeetingNode, JoinMeetingNode, InMeetingNode}

	// 2. Display Nodes (from display_nodes.go)
	InitCDPNode.Next = []*Node{StartMeetNode}
	StartMeetNode.Next = []*Node{WorkspaceRedirectedNode, AccountsGooglePageNode, MeetLandingPageNode, CheckInvalidMeetingNode, JoinMeetingNode, InMeetingNode, NavigateToMeeting}
	MeetLandingPageNode.Next = []*Node{CheckInvalidMeetingNode, NavigateToMeeting, JoinMeetingNode, InMeetingNode}
	JoinMeetingNode.Next = []*Node{CheckInvalidMeetingNode, InMeetingNode, MeetLandingPageNode}
	InMeetingNode.Next = []*Node{CheckInvalidMeetingNode, MeetLandingPageNode, NavigateToMeeting, LeaveMeetingNode}
	LeaveMeetingNode.Next = []*Node{StartMeetNode}
	NavigateToMeeting.Next = []*Node{CheckInvalidMeetingNode, JoinMeetingNode, InMeetingNode, MeetLandingPageNode, NavigateToMeeting}

	// 3. Add has_wifi WS handler to all initial setup nodes
	setupNodes := []*Node{
		InitServerNode, WaitWebServerNode, InitDisplay2CDPNode, WaitForClientCallbackNode, CredentialsNode, AuthTokenNode, CalendarNode,
		WorkspaceRedirectedNode, AccountsGooglePageNode, ChooseAccountNode,
		AccountOptionExistsNode, AccountOptionMissingNode, EmailInputNode,
		PasswordInputNode, WrongPasswordNode, TwoFactorNode,
	}

	for i, n := range setupNodes {
		origSetup := n.Setup
		phaseIdx := i + 1
		n.Setup = func(s *StateContext) error {
			s.SetSetupPhase(phaseIdx)
			s.AddWSHandler("has_wifi", func(payload json.RawMessage) (interface{}, error) {
				client := http.Client{Timeout: 3 * time.Second}
				_, err := client.Get("https://google.com")
				return map[string]bool{"internetAccess": err == nil}, nil
			})
			if origSetup != nil {
				return origSetup(s)
			}
			return nil
		}
		origTeardown := n.Teardown
		n.Teardown = func(s *StateContext) error {
			s.RemoveWSHandler("has_wifi")
			if origTeardown != nil {
				return origTeardown(s)
			}
			return nil
		}
	}

	origInitCDPSetup := InitCDPNode.Setup
	InitCDPNode.Setup = func(s *StateContext) error {
		s.SetSetupPhase(1000)
		if origInitCDPSetup != nil {
			return origInitCDPSetup(s)
		}
		return nil
	}

	return InitServerNode
}
