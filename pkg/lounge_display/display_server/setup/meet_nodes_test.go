package setup

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInitCDPNode(t *testing.T) {
	s, _, _, _, _ := setupTestContext("../../logs")
	assert.True(t, InitCDPNode.PreCheck(s))

	// Note: We can't easily unit test InitCDPNode's Work because it requires a live CDP server.
	// But we can test that it exists.
}

func TestStartMeetNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	assert.True(t, StartMeetNode.PreCheck(s))

	err := StartMeetNode.Setup(s)
	assert.NoError(t, err)
	assert.Equal(t, StartMeetNode, s.DefaultNode)

	err = StartMeetNode.Work(s)
	assert.NoError(t, err)
	
	actions := strings.Join(browser.ActionLog, "\n")
	assert.Contains(t, actions, "Navigate: https://meet.google.com/landing")
}

func TestMeetLandingPageNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	err := MeetLandingPageNode.Setup(s)
	assert.NoError(t, err)

	browser.CurrentURL = "https://meet.google.com/landing"
	assert.True(t, MeetLandingPageNode.PreCheck(s))
	assert.True(t, MeetLandingPageNode.RestNodeValidation(s))

	// With NavTarget set, PreCheck should be false
	s.NavTarget = "NavigateToMeeting"
	assert.False(t, MeetLandingPageNode.PreCheck(s))
	s.NavTarget = ""

	handler, ok := s.GetWSHandler("join_meeting")
	assert.True(t, ok)

	res, err := handler([]byte(`{"code": "xyz-abcd-efg"}`))
	assert.NoError(t, err)
	resMap, _ := res.(map[string]string)
	assert.Equal(t, "ok", resMap["status"])
	assert.Equal(t, "NavigateToMeeting", s.NavTarget)
	assert.Equal(t, "xyz-abcd-efg", s.NavOpts["code"])

	err = MeetLandingPageNode.Work(s)
	assert.NoError(t, err)
	assert.Equal(t, "", s.MeetingCode)

	MeetLandingPageNode.Teardown(s)
	_, ok = s.GetWSHandler("join_meeting")
	assert.False(t, ok)
}

func TestNavigateToMeetingNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	s.NavTarget = "NavigateToMeeting"
	assert.True(t, NavigateToMeeting.PreCheck(s))

	s.NavOpts = map[string]interface{}{"code": "test-code"}
	err := NavigateToMeeting.Work(s)
	assert.NoError(t, err)
	
	actions := strings.Join(browser.ActionLog, "\n")
	assert.Contains(t, actions, "Navigate: https://meet.google.com/test-code")
	assert.Equal(t, "test-code", s.MeetingCode)
}

func TestJoinMeetingNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")
	
	browser.CurrentURL = "https://meet.google.com/test-code"
	browser.HTMLContent = "join now"
	assert.True(t, JoinMeetingNode.PreCheck(s))

	// Override evaluation to return "found" for the button
	browser.HTMLContent = "join now bot-join-button"
	err := JoinMeetingNode.Work(s)
	assert.NoError(t, err)
	
	actions := strings.Join(browser.ActionLog, "\n")
	assert.Contains(t, actions, "Click: #bot-join-button")
}

func TestInMeetingNode(t *testing.T) {
	s, browser, _, _, _ := setupTestContext("../../logs")

	err := InMeetingNode.Setup(s)
	assert.NoError(t, err)
	
	browser.CurrentURL = "https://meet.google.com/test-code"
	browser.CustomEvalResult = false // No join button implies we are in-meeting
	assert.True(t, InMeetingNode.PreCheck(s))

	err = InMeetingNode.Work(s)
	assert.NoError(t, err)
	assert.Equal(t, "test-code", s.MeetingCode)

	// Test button_state handler
	browser.HTMLContent = `{"in_meeting":true,"microphone":false,"camera":false,"hand":false}`
	stateHandler, ok := s.GetWSHandler("button_state")
	assert.True(t, ok)
	res, err := stateHandler(nil)
	assert.NoError(t, err)
	resMap, _ := res.(map[string]interface{})
	assert.False(t, resMap["microphone"].(bool))

	// Test click_button handler
	clickHandler, ok := s.GetWSHandler("click_button")
	assert.True(t, ok)
	
	// Hangup triggers state change, not browser eval
	_, err = clickHandler([]byte(`{"button": "hangup"}`))
	assert.NoError(t, err)
	assert.Equal(t, "LeaveMeeting", s.NavTarget)

	// Regular click triggers eval
	browser.CustomEvalResult = true
	_, err = clickHandler([]byte(`{"button": "microphone"}`))
	assert.NoError(t, err)

	InMeetingNode.Teardown(s)
	_, ok = s.GetWSHandler("button_state")
	assert.False(t, ok)
}

func TestCheckInvalidMeetingNode(t *testing.T) {
	s, fakeBrowser, _, _, _ := setupTestContext("../../logs")

	fakeBrowser.CurrentURL = "https://meet.google.com/_meet/invalid-url"
	assert.True(t, CheckInvalidMeetingNode.PreCheck(s))

	err := CheckInvalidMeetingNode.Work(s)
	assert.NoError(t, err)
	assert.Equal(t, "", s.MeetingCode)

	err = CheckInvalidMeetingNode.DoneCheck(s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid meeting URL")
}

func TestLeaveMeetingNode(t *testing.T) {
	s, browser, _, _, fakeClock := setupTestContext("../../logs")

	s.NavTarget = "LeaveMeeting"
	assert.True(t, LeaveMeetingNode.PreCheck(s))

	browser.CustomEvalResult = true // pretend click succeeds
	go func() {
		time.Sleep(10 * time.Millisecond)
		fakeClock.Advance(2 * time.Second)
	}()
	err := LeaveMeetingNode.Work(s)
	assert.NoError(t, err)
	assert.Equal(t, "", s.NavTarget)
}
