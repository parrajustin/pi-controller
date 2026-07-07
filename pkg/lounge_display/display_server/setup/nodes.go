package setup

import (
	"fmt"
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
			fmt.Printf("Server initializing on port %s\n", s.PortFlag)
			return nil
		},
	}

	// Link nodes together

	// 1. Setup Nodes (from setup_nodes.go)
	InitServerNode.Next = []*Node{CredentialsNode}
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
	TwoFactorNode.Next = []*Node{FinishedMeetNode, CheckInvalidMeetingNode, JoinMeetingNode, InMeetingNode}

	// 2. Display Nodes (from display_nodes.go)
	InitCDPNode.Next = []*Node{StartMeetNode}
	StartMeetNode.Next = []*Node{WorkspaceRedirectedNode, FinishedMeetNode, CheckInvalidMeetingNode, JoinMeetingNode, InMeetingNode, NavigateToMeeting}
	FinishedMeetNode.Next = []*Node{CheckInvalidMeetingNode, NavigateToMeeting, JoinMeetingNode, InMeetingNode}
	JoinMeetingNode.Next = []*Node{CheckInvalidMeetingNode, InMeetingNode, FinishedMeetNode}
	InMeetingNode.Next = []*Node{CheckInvalidMeetingNode, FinishedMeetNode, NavigateToMeeting, LeaveMeetingNode}
	LeaveMeetingNode.Next = []*Node{StartMeetNode}
	NavigateToMeeting.Next = []*Node{CheckInvalidMeetingNode, JoinMeetingNode, InMeetingNode, FinishedMeetNode}

	return InitServerNode
}
