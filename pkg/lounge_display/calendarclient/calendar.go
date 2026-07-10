package calendarclient

import (
	"time"

	"google.golang.org/api/calendar/v3"
)

type EventInfo struct {
	Name        string `json:"name"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Accepted    string `json:"acceptedStatus"`
	Description string `json:"description"`
	MeetLink    string `json:"meetLink"`
}

type CalendarClient interface {
	FetchEvents() ([]EventInfo, error)
	TestConnection() error
}

type RealCalendarClient struct {
	srv *calendar.Service
}

func NewRealCalendarClient(srv *calendar.Service) *RealCalendarClient {
	return &RealCalendarClient{srv: srv}
}

func (c *RealCalendarClient) FetchEvents() ([]EventInfo, error) {
	tMin := time.Now().Add(-15 * time.Minute).Format(time.RFC3339)
	tMax := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	events, err := c.srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(tMin).TimeMax(tMax).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		return nil, err
	}

	var results []EventInfo
	for _, item := range events.Items {
		startDate := item.Start.DateTime
		if startDate == "" {
			startDate = item.Start.Date
		}
		endDate := ""
		if item.End != nil {
			endDate = item.End.DateTime
			if endDate == "" {
				endDate = item.End.Date
			}
		}

		acceptedStatus := "unknown"
		for _, attendee := range item.Attendees {
			if attendee.Self {
				acceptedStatus = attendee.ResponseStatus
				break
			}
		}
		if len(item.Attendees) == 0 {
			acceptedStatus = "accepted"
		}

		if acceptedStatus == "declined" {
			continue
		}

		results = append(results, EventInfo{
			Name:        item.Summary,
			StartTime:   startDate,
			EndTime:     endDate,
			Accepted:    acceptedStatus,
			Description: item.Description,
			MeetLink:    item.HangoutLink,
		})
	}
	return results, nil
}

func (c *RealCalendarClient) TestConnection() error {
	t := time.Now().Format(time.RFC3339)
	_, err := c.srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	return err
}

type FakeCalendarClient struct {
	Events []EventInfo
	Err    error
}

func NewFakeCalendarClient() *FakeCalendarClient {
	return &FakeCalendarClient{
		Events: []EventInfo{},
		Err:    nil,
	}
}

func (c *FakeCalendarClient) FetchEvents() ([]EventInfo, error) {
	return c.Events, c.Err
}

func (c *FakeCalendarClient) TestConnection() error {
	return c.Err
}
