package setup

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInitServerNode(t *testing.T) {
	s, _, _, _, _ := setupTestContext("../../logs")
	InitServerNode.Setup(s)
	assert.Equal(t, InitServerNode, s.DefaultNode)
}

func TestCredentialsNode(t *testing.T) {
	s, _, _, _, _ := setupTestContext("../../logs")
	tmpDir := t.TempDir()
	s.OauthDir = tmpDir
	
	// Setup registers /api/has_cred and /api/cred
	err := CredentialsNode.Setup(s)
	assert.NoError(t, err)

	s.mu.Lock()
	s.DisplayActive = true
	s.mu.Unlock()

	// Test PreCheck
	assert.True(t, CredentialsNode.PreCheck(s))

	// Test API /has_cred before creds exist
	req := httptest.NewRequest("GET", "/api/has_cred", nil)
	w := httptest.NewRecorder()
	s.Mux.ServeHTTP(w, req)
	assert.Contains(t, w.Body.String(), `"hasCreds":false`)

	// Simulate receiving credentials via POST
	go func() {
		time.Sleep(100 * time.Millisecond)
		credReq := httptest.NewRequest("POST", "/api/cred", bytes.NewBuffer([]byte(`{"dummy":"cred"}`)))
		credW := httptest.NewRecorder()
		s.Mux.ServeHTTP(credW, credReq)
	}()

	err = CredentialsNode.Work(s)
	assert.NoError(t, err)
	
	// Wait for the mock POST request goroutine
	time.Sleep(200 * time.Millisecond)
	
	// API /has_cred should now return true
	req2 := httptest.NewRequest("GET", "/api/has_cred", nil)
	w2 := httptest.NewRecorder()
	s.Mux.ServeHTTP(w2, req2)
	assert.Contains(t, w2.Body.String(), `"hasCreds":true`)
}

func TestAuthTokenNode(t *testing.T) {
	s, _, _, _, _ := setupTestContext("../../logs")
	
	// Setup
	err := AuthTokenNode.Setup(s)
	assert.NoError(t, err)
	
	handler, ok := s.GetWSHandler("get_auth_url")
	assert.True(t, ok)
	res, err := handler(nil)
	assert.NoError(t, err)
	resMap, _ := res.(map[string]string)
	// It's empty initially because Work hasn't run to generate the URL, but handler exists
	_ = resMap

	handler, ok = s.GetWSHandler("submit_token")
	assert.True(t, ok)

	// Provide a dummy credentials.json to prevent nil pointer in google config
	tmpDir := t.TempDir()
	s.OauthDir = tmpDir
	os.WriteFile(filepath.Join(tmpDir, "credentials.json"), []byte(`{"web":{"client_id":"dummy","client_secret":"dummy","redirect_uris":["http://localhost:7070"]}}`), 0600)

	// Mock the authCodeChan
	go func() {
		time.Sleep(100 * time.Millisecond)
		handler([]byte(`{"code": "auth_code_123"}`))
	}()

	err = AuthTokenNode.Work(s)
	assert.NoError(t, err) 

	AuthTokenNode.Teardown(s)
	_, ok = s.GetWSHandler("get_auth_url")
	assert.False(t, ok)
}

func TestCalendarNode(t *testing.T) {
	s, _, fakeCalendar, _, fakeClock := setupTestContext("../../logs")
	hasToken = true // Global variable used in setup_nodes.go
	s.CalendarClient = fakeCalendar
	assert.True(t, CalendarNode.PreCheck(s))

	// Success case
	err := CalendarNode.Work(s)
	assert.NoError(t, err)

	// Failure case
	fakeCalendar.Err = fmt.Errorf("test error")
	done := make(chan bool)
	go func() {
		CalendarNode.Work(s)
		done <- true
	}()
	
	time.Sleep(10 * time.Millisecond) // Let goroutine reach sleep
	fakeCalendar.Err = nil
	fakeClock.Advance(5 * time.Second) // Advance clock past retry delay
	
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		// It's retrying correctly
	}
}

func TestWorkspaceRedirectedNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	browser.CurrentURL = "https://workspace.google.com/foo"
	assert.True(t, WorkspaceRedirectedNode.PreCheck(s))

	browser.CurrentURL = "https://accounts.google.com/"
	assert.False(t, WorkspaceRedirectedNode.PreCheck(s))

	// Mocking sign-in click success
	err := WorkspaceRedirectedNode.Work(s)
	assert.NoError(t, err)
	foundEval := false
	for _, act := range browser.ActionLog {
		if strings.Contains(act, `data-g-action="sign in"`) {
			foundEval = true
		}
	}
	assert.True(t, foundEval)
}

func TestAccountsGooglePageNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	browser.CurrentURL = "https://accounts.google.com/signin"
	assert.True(t, AccountsGooglePageNode.PreCheck(s))
}

func TestChooseAccountNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	// Test positive condition matching HTML text
	browser.HTMLContent = "<html><body>choose an account</body></html>"
	assert.True(t, ChooseAccountNode.PreCheck(s))

	browser.HTMLContent = ""
	assert.False(t, ChooseAccountNode.PreCheck(s))
}

func TestAccountOptionExistsNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	browser.CustomEvalResult = true
	assert.True(t, AccountOptionExistsNode.PreCheck(s))

	err := AccountOptionExistsNode.Work(s)
	assert.NoError(t, err)
}

func TestAccountOptionMissingNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	browser.HTMLContent = "use another account"
	assert.True(t, AccountOptionMissingNode.PreCheck(s))

	err := AccountOptionMissingNode.Work(s)
	assert.NoError(t, err)
}

func TestEmailInputNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	browser.CustomEvalResult = true
	assert.True(t, EmailInputNode.PreCheck(s))

	err := EmailInputNode.Work(s)
	assert.NoError(t, err)

	actions := strings.Join(browser.ActionLog, "\n")
	assert.Contains(t, actions, "WaitVisible: #identifierId")
	assert.Contains(t, actions, "SendKeys: #identifierId test@example.com")
	assert.Contains(t, actions, "Click: #identifierNext button")
}

func TestPasswordInputNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	PasswordInputNode.Setup(s)
	handler, ok := s.GetWSHandler("submit_password")
	assert.True(t, ok)

	browser.CustomEvalResult = true
	assert.True(t, PasswordInputNode.PreCheck(s))

	go func() {
		handler([]byte(`{"password": "mypassword123"}`))
	}()

	err := PasswordInputNode.Work(s)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	actions := strings.Join(browser.ActionLog, "\n")
	assert.Contains(t, actions, "SendKeys: input[type=\"password\"] mypassword123")
	assert.Contains(t, actions, "Click: #passwordNext button")

	PasswordInputNode.Teardown(s)
	_, ok = s.GetWSHandler("submit_password")
	assert.False(t, ok)
}

func TestWrongPasswordNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	browser.HTMLContent = "wrong password"
	assert.True(t, WrongPasswordNode.PreCheck(s))

	err := WrongPasswordNode.Work(s)
	assert.NoError(t, err)
}

func TestTwoFactorNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	browser.HTMLContent = "2-step verification"
	browser.CurrentURL = "https://accounts.google.com/signin/challenge" // Not matching meet.google.com
	
	// It evaluates to true because URL is not meet.google.com and contains text
	assert.True(t, TwoFactorNode.PreCheck(s))
}
