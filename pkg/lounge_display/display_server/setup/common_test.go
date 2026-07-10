package setup

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/parrajustin/pi-controller/pkg/lounge_display/browser"
	"github.com/parrajustin/pi-controller/pkg/lounge_display/calendarclient"
)

// FakeClock allows tests to control time instantly.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []fakeWaiter
}

type fakeWaiter struct {
	wakeAt time.Time
	ch     chan struct{}
}

func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *FakeClock) Sleep(d time.Duration) {
	ch := make(chan struct{})
	f.mu.Lock()
	f.waiters = append(f.waiters, fakeWaiter{
		wakeAt: f.now.Add(d),
		ch:     ch,
	})
	f.mu.Unlock()
	<-ch
}

func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	
	var remaining []fakeWaiter
	for _, w := range f.waiters {
		if !f.now.Before(w.wakeAt) {
			close(w.ch)
		} else {
			remaining = append(remaining, w)
		}
	}
	f.waiters = remaining
}

type mockWSConn struct {
	messages []map[string]interface{}
}

func (m *mockWSConn) WriteJSON(v interface{}) error {
	msgBytes, _ := json.Marshal(v)
	var msg map[string]interface{}
	json.Unmarshal(msgBytes, &msg)
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockWSConn) Close() error { return nil }

func setupTestContext(logsDir string) (*StateContext, *browser.FakeBrowser, *calendarclient.FakeCalendarClient, *mockWSConn, *FakeClock) {
	mux := http.NewServeMux()
	fakeBrowser := browser.NewFakeBrowser(logsDir)
	fakeCalendar := calendarclient.NewFakeCalendarClient()
	wsConn := &mockWSConn{messages: make([]map[string]interface{}, 0)}
	fakeClock := NewFakeClock(time.Now())

	s := &StateContext{
		Mux:              mux,
		Browser:          fakeBrowser,
		CalendarClient:   fakeCalendar,
		Clock:            fakeClock,
		Ctx:              context.Background(),
		NodeTimeout:      1 * time.Second,
		PasswordChan:     make(chan string, 1),
		Email:            "test@example.com",
		RegisteredRoutes: make(map[string]bool),
		WSHandlers:       make(map[string]func(payload json.RawMessage) (interface{}, error)),
	}
	
	// Ensure nodes are initialized
	InitNodes()

	return s, fakeBrowser, fakeCalendar, wsConn, fakeClock
}
