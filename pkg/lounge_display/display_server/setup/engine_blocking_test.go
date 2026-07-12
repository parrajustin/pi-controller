package setup

import (
	"context"
	"testing"
	"time"

)

func TestEngine_NodesAreNonBlocking(t *testing.T) {
	s, _, _, _, fakeClock := setupTestContext("../../logs")

	startNode := InitNodes()
	visited := make(map[*Node]bool)
	var queue []*Node
	queue = append(queue, startNode)

	// Background ticker to advance fake clock
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Millisecond):
				// fast forward time
				fakeClock.Advance(1 * time.Minute)
			}
		}
	}()

	whitelistedNodes := map[string]bool{
		"Init CDP":             true,
		"wait_web_server":      true,
		"Init Display 2 CDP":   true,
		"Calendar Logic Phase": true, // Has an infinite retry loop for API connection
	}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if visited[curr] {
			continue
		}
		visited[curr] = true

		if whitelistedNodes[curr.Name] {
			// Skip checking whitelisted system init nodes that are allowed to block
		} else {
			if curr.Work != nil {
				done := make(chan struct{})
				go func() {
					curr.Work(s)
					close(done)
				}()

				select {
				case <-done:
					// Success, it returned
				case <-time.After(100 * time.Millisecond):
					t.Errorf("Node %q blocked in Work(). Work() must return quickly or rely on the clock and exit when the clock advances.", curr.Name)
				}
			}

			if curr.DoneCheck != nil {
				done := make(chan struct{})
				go func() {
					curr.DoneCheck(s)
					close(done)
				}()

				select {
				case <-done:
					// Success
				case <-time.After(100 * time.Millisecond):
					t.Errorf("Node %q blocked in DoneCheck().", curr.Name)
				}
			}
		}

		for _, next := range curr.Next {
			queue = append(queue, next)
		}
	}
}
