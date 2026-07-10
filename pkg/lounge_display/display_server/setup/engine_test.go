package setup

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEngine_GettersAndSetters(t *testing.T) {
	s := &StateContext{}

	s.SetPhase("setup")
	assert.Equal(t, "setup", s.GetPhase())

	s.SetSetupPhase(5)
	assert.Equal(t, 5, s.GetSetupPhase())

	s.SetSetupReady(true)
	assert.True(t, s.GetSetupReady())
}

func TestEngine_ExecutionOrder(t *testing.T) {
	s, _, _, _, _ := setupTestContext("../../logs")

	var executionOrder []string

	node1 := &Node{
		Name: "Node1",
		Setup: func(ctx *StateContext) error {
			executionOrder = append(executionOrder, "setup")
			return nil
		},
		Work: func(ctx *StateContext) error {
			executionOrder = append(executionOrder, "work")
			return nil
		},
		DoneCheck: func(ctx *StateContext) error {
			executionOrder = append(executionOrder, "done-check")
			return nil
		},
		Teardown: func(ctx *StateContext) error {
			executionOrder = append(executionOrder, "teardown")
			return nil
		},
		Next: []*Node{}, // Empty next causes terminal return
	}

	RunEngine(node1, s)

	assert.Equal(t, []string{"setup", "work", "done-check"}, executionOrder, "Terminal node does not execute Teardown")
}

func TestEngine_WorkFailureRetryOnStartNode(t *testing.T) {
	s, _, _, _, fakeClock := setupTestContext("../../logs")

	workAttempts := 0

	node1 := &Node{
		Name: "Node1",
		Work: func(ctx *StateContext) error {
			workAttempts++
			if workAttempts == 1 {
				return errors.New("temporary failure")
			}
			return nil
		},
		Next: []*Node{},
	}

	done := make(chan struct{})
	go func() {
		RunEngine(node1, s)
		close(done)
	}()
	
	time.Sleep(10 * time.Millisecond) // wait for goroutine to hit sleep
	fakeClock.Advance(2 * time.Second) // wake it up
	<-done
	
	// Should retry because it is the start node and it failed the first time
	assert.Equal(t, 2, workAttempts)
}

func TestEngine_DoneCheckFailureRevertsToDefault(t *testing.T) {
	s, _, _, _, _ := setupTestContext("../../logs")

	defaultNode := &Node{
		Name: "Default",
		Next: []*Node{}, // Terminal
	}
	s.DefaultNode = defaultNode

	node1 := &Node{
		Name: "Node1",
		DoneCheck: func(ctx *StateContext) error {
			return errors.New("done check failed")
		},
		Next: []*Node{
			{Name: "Unreachable", PreCheck: func(ctx *StateContext) bool { return true }},
		},
	}

	RunEngine(node1, s)
	
	// Because DoneCheck failed, it should revert to Default Node directly (skipping Next array)
	assert.Equal(t, "Default", s.GetNodeName())
}

func TestEngine_RestNodeValidationFailure(t *testing.T) {
	s, _, _, _, fakeClock := setupTestContext("../../logs")

	defaultNode := &Node{
		Name: "Default",
		Next: []*Node{}, // Terminal
	}
	s.DefaultNode = defaultNode

	iterations := 0

	restNode := &Node{
		Name:       "RestNode",
		IsRestNode: true,
		RestNodeValidation: func(ctx *StateContext) bool {
			iterations++
			// Fail on the second iteration
			return iterations == 1
		},
		Next: []*Node{
			{Name: "Unreachable", PreCheck: func(ctx *StateContext) bool { return false }},
		},
	}

	go RunEngine(restNode, s)
	time.Sleep(10 * time.Millisecond) // Let goroutine start
	fakeClock.Advance(1 * time.Second) // Let loop tick twice
	time.Sleep(10 * time.Millisecond) // Let goroutine transition

	assert.Equal(t, "Default", s.GetNodeName(), "RestNodeValidation failure should revert to Default Node")
}

func TestEngine_RestNodePreCheckFailure(t *testing.T) {
	s, _, _, _, fakeClock := setupTestContext("../../logs")

	defaultNode := &Node{
		Name: "Default",
		Next: []*Node{}, // Terminal
	}
	s.DefaultNode = defaultNode

	iterations := 0

	restNode := &Node{
		Name:       "RestNode2",
		IsRestNode: true,
		PreCheck: func(ctx *StateContext) bool {
			iterations++
			// Serve as PreCheck fallback for RestNodeValidation
			// Return false on second iteration
			return iterations == 1
		},
		Next: []*Node{
			{Name: "Unreachable", PreCheck: func(ctx *StateContext) bool { return false }},
		},
	}

	go RunEngine(restNode, s)
	time.Sleep(10 * time.Millisecond) // Let goroutine start
	fakeClock.Advance(1 * time.Second) // Let loop tick twice
	time.Sleep(10 * time.Millisecond) // Let goroutine transition

	assert.Equal(t, "Default", s.GetNodeName(), "RestNode PreCheck failure should revert to Default Node")
}
