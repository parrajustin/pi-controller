package setup

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

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

type Node struct {
	Name      string
	Setup     func(s *StateContext) error
	PreCheck  func(s *StateContext) bool
	Work      func(s *StateContext) error
	DoneCheck func(s *StateContext) error
	Teardown  func(s *StateContext) error
	Next      []*Node
}

func RunEngine(startNode *Node, s *StateContext) {
	currentNode := startNode
	stepCounter := 0
	for {
		stepCounter++
		fmt.Printf("\n=== Executing Node %04d: %s ===\n", stepCounter, currentNode.Name)
		s.SetNodeName(currentNode.Name)

		if currentNode.Setup != nil {
			err := currentNode.Setup(s)
			if err != nil {
				log.Printf("Setup failed for node %s: %v\n", currentNode.Name, err)
			}
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

		if currentNode.DoneCheck != nil {
			err := currentNode.DoneCheck(s)
			if err != nil {
				log.Printf("DoneCheck failed for node %s: %v\n", currentNode.Name, err)
				currentNode = startNode
				continue
			}
		}

		if len(currentNode.Next) == 0 {
			fmt.Println("\nFlow finished successfully at terminal node:", currentNode.Name)
			return
		}

		fmt.Printf("Finding next node from %d possibilities...\n", len(currentNode.Next))
		var nextNode *Node

		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) && nextNode == nil {
			for _, n := range currentNode.Next {
				if n.PreCheck != nil && n.PreCheck(s) {
					nextNode = n
					break
				}
			}
			if nextNode == nil {
				time.Sleep(500 * time.Millisecond)
			}
		}

		if nextNode == nil {
			fmt.Println("\nERROR: No valid next path found! Restarting flow.")
			currentNode = startNode
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
