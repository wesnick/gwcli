package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/calendar/v3"
)

func TestRunEventsList(t *testing.T) {
	eventsJSON := `{
		"kind": "calendar#events",
		"items": [
			{
				"id": "event1",
				"summary": "Team Meeting",
				"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
				"end": {"dateTime": "2024-01-15T11:00:00-08:00"},
				"status": "confirmed"
			},
			{
				"id": "event2",
				"summary": "Lunch",
				"start": {"date": "2024-01-16"},
				"end": {"date": "2024-01-17"},
				"status": "confirmed"
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(eventsJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsList(context.Background(), conn, "primary", "", "", "", 25, false, out)
	if err != nil {
		t.Fatalf("runEventsList() error = %v", err)
	}

	var result []eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 events, got %d", len(result))
	}

	if result[0].Summary != "Team Meeting" {
		t.Errorf("expected first event summary 'Team Meeting', got %q", result[0].Summary)
	}
}

func TestRunEventsListAllDay(t *testing.T) {
	eventsJSON := `{
		"kind": "calendar#events",
		"items": [
			{
				"id": "event1",
				"summary": "Holiday",
				"start": {"date": "2024-12-25"},
				"end": {"date": "2024-12-26"},
				"status": "confirmed"
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(eventsJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsList(context.Background(), conn, "primary", "", "", "", 25, false, out)
	if err != nil {
		t.Fatalf("runEventsList() error = %v", err)
	}

	var result []eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 event, got %d", len(result))
	}

	if !result[0].AllDay {
		t.Error("expected event to be all-day")
	}

	if result[0].StartDate != "2024-12-25" {
		t.Errorf("expected start date '2024-12-25', got %q", result[0].StartDate)
	}
}

func TestRunEventsListEmpty(t *testing.T) {
	emptyJSON := `{
		"kind": "calendar#events",
		"items": []
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(emptyJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsList(context.Background(), conn, "primary", "", "", "", 25, false, out)
	if err != nil {
		t.Fatalf("runEventsList() error = %v", err)
	}

	var result []eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 events, got %d", len(result))
	}
}

func TestRunEventsListTextOutput(t *testing.T) {
	eventsJSON := `{
		"kind": "calendar#events",
		"items": [
			{
				"id": "event1",
				"summary": "Team Meeting",
				"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
				"end": {"dateTime": "2024-01-15T11:00:00-08:00"},
				"status": "confirmed"
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(eventsJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}

	err = runEventsList(context.Background(), conn, "primary", "", "", "", 25, false, out)
	if err != nil {
		t.Fatalf("runEventsList() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DATE") {
		t.Errorf("expected output to contain 'DATE' header, got %q", output)
	}
	if !strings.Contains(output, "Team Meeting") {
		t.Errorf("expected output to contain 'Team Meeting', got %q", output)
	}
	if !strings.Contains(output, "event1") {
		t.Errorf("expected output to contain event ID, got %q", output)
	}
}

func TestRunEventsListWithAttendees(t *testing.T) {
	eventsJSON := `{
		"kind": "calendar#events",
		"items": [
			{
				"id": "event1",
				"summary": "Team Meeting",
				"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
				"end": {"dateTime": "2024-01-15T11:00:00-08:00"},
				"status": "confirmed",
				"attendees": [
					{
						"email": "user1@example.com",
						"displayName": "User One",
						"responseStatus": "accepted",
						"self": true
					},
					{
						"email": "user2@example.com",
						"displayName": "User Two",
						"responseStatus": "tentative"
					}
				],
				"organizer": {
					"email": "organizer@example.com",
					"displayName": "Organizer",
					"self": false
				}
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(eventsJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsList(context.Background(), conn, "primary", "", "", "", 25, false, out)
	if err != nil {
		t.Fatalf("runEventsList() error = %v", err)
	}

	var result []eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}

	if len(result[0].Attendees) != 2 {
		t.Errorf("expected 2 attendees, got %d", len(result[0].Attendees))
	}

	if result[0].Attendees[0].Email != "user1@example.com" {
		t.Errorf("expected first attendee email 'user1@example.com', got %q", result[0].Attendees[0].Email)
	}

	if result[0].Organizer == nil {
		t.Fatal("expected organizer to be set")
	}
	if result[0].Organizer.Email != "organizer@example.com" {
		t.Errorf("expected organizer email 'organizer@example.com', got %q", result[0].Organizer.Email)
	}
}

func TestFormatEventTime(t *testing.T) {
	tests := []struct {
		name     string
		event    *calendar.Event
		wantDate string
		wantTime string
	}{
		{
			name: "timed event",
			event: &calendar.Event{
				Start: &calendar.EventDateTime{DateTime: "2024-01-15T10:00:00-08:00"},
			},
			wantDate: "2024-01-15",
			wantTime: "10:00",
		},
		{
			name: "all-day event",
			event: &calendar.Event{
				Start: &calendar.EventDateTime{Date: "2024-01-15"},
			},
			wantDate: "2024-01-15",
			wantTime: "all-day",
		},
		{
			name:     "nil start",
			event:    &calendar.Event{},
			wantDate: "",
			wantTime: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, timeStr := formatEventTime(tt.event)
			if date != tt.wantDate {
				t.Errorf("formatEventTime() date = %q, want %q", date, tt.wantDate)
			}
			if timeStr != tt.wantTime {
				t.Errorf("formatEventTime() time = %q, want %q", timeStr, tt.wantTime)
			}
		})
	}
}

func TestRunEventsRead(t *testing.T) {
	eventJSON := `{
		"id": "event1",
		"summary": "Team Meeting",
		"description": "Weekly sync meeting",
		"location": "Conference Room A",
		"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
		"end": {"dateTime": "2024-01-15T11:00:00-08:00"},
		"status": "confirmed",
		"htmlLink": "https://calendar.google.com/event?eid=xxx",
		"attendees": [
			{"email": "alice@example.com", "responseStatus": "accepted"},
			{"email": "bob@example.com", "responseStatus": "tentative"}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/event1") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(eventJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsRead(context.Background(), conn, "primary", "event1", out)
	if err != nil {
		t.Fatalf("runEventsRead() error = %v", err)
	}

	var result eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Summary != "Team Meeting" {
		t.Errorf("expected summary 'Team Meeting', got %q", result.Summary)
	}

	if result.Location != "Conference Room A" {
		t.Errorf("expected location 'Conference Room A', got %q", result.Location)
	}

	if len(result.Attendees) != 2 {
		t.Errorf("expected 2 attendees, got %d", len(result.Attendees))
	}
}

func TestRunEventsReadTextOutput(t *testing.T) {
	eventJSON := `{
		"id": "event1",
		"summary": "Team Meeting",
		"description": "Weekly sync meeting",
		"location": "Conference Room A",
		"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
		"end": {"dateTime": "2024-01-15T11:00:00-08:00"},
		"status": "confirmed",
		"htmlLink": "https://calendar.google.com/event?eid=xxx",
		"attendees": [
			{"email": "alice@example.com", "displayName": "Alice", "responseStatus": "accepted"},
			{"email": "bob@example.com", "responseStatus": "tentative"}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/event1") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(eventJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}

	err = runEventsRead(context.Background(), conn, "primary", "event1", out)
	if err != nil {
		t.Fatalf("runEventsRead() error = %v", err)
	}

	output := buf.String()

	// Verify text output contains expected fields
	if !strings.Contains(output, "Summary: Team Meeting") {
		t.Errorf("expected output to contain 'Summary: Team Meeting', got %q", output)
	}
	if !strings.Contains(output, "Location: Conference Room A") {
		t.Errorf("expected output to contain 'Location: Conference Room A', got %q", output)
	}
	if !strings.Contains(output, "Description: Weekly sync meeting") {
		t.Errorf("expected output to contain 'Description: Weekly sync meeting', got %q", output)
	}
	if !strings.Contains(output, "Attendees:") {
		t.Errorf("expected output to contain 'Attendees:', got %q", output)
	}
	if !strings.Contains(output, "Alice (accepted)") {
		t.Errorf("expected output to contain 'Alice (accepted)', got %q", output)
	}
	if !strings.Contains(output, "bob@example.com (tentative)") {
		t.Errorf("expected output to contain 'bob@example.com (tentative)', got %q", output)
	}
	if !strings.Contains(output, "ID: event1") {
		t.Errorf("expected output to contain 'ID: event1', got %q", output)
	}
}

func TestRunEventsReadAllDay(t *testing.T) {
	eventJSON := `{
		"id": "event1",
		"summary": "Holiday",
		"start": {"date": "2024-12-25"},
		"end": {"date": "2024-12-26"},
		"status": "confirmed"
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(eventJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}

	err = runEventsRead(context.Background(), conn, "primary", "event1", out)
	if err != nil {
		t.Fatalf("runEventsRead() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Date: 2024-12-25 (all day)") {
		t.Errorf("expected output to contain 'Date: 2024-12-25 (all day)', got %q", output)
	}
}

func TestRunEventsReadMissingEventID(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsRead(context.Background(), conn, "primary", "", out)
	if err == nil {
		t.Fatal("expected error for missing event ID, got nil")
	}

	if !strings.Contains(err.Error(), "event ID is required") {
		t.Errorf("expected error to contain 'event ID is required', got %q", err.Error())
	}
}

func TestRunEventsReadDefaultCalendar(t *testing.T) {
	eventJSON := `{
		"id": "event1",
		"summary": "Team Meeting",
		"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
		"end": {"dateTime": "2024-01-15T11:00:00-08:00"}
	}`

	var requestedPath string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestedPath = req.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(eventJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	// Pass empty calendarID to test default behavior
	err = runEventsRead(context.Background(), conn, "", "event1", out)
	if err != nil {
		t.Fatalf("runEventsRead() error = %v", err)
	}

	// Verify that "primary" was used as the default calendar ID
	if !strings.Contains(requestedPath, "/calendars/primary/events/") {
		t.Errorf("expected request to use 'primary' calendar, got path %q", requestedPath)
	}
}

func TestRunEventsCreate(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "new-event",
						"summary": "New Meeting",
						"start": {"dateTime": "2024-01-20T14:00:00Z"},
						"end": {"dateTime": "2024-01-20T15:00:00Z"},
						"status": "confirmed"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := createEventOptions{
		summary: "New Meeting",
		start:   "2024-01-20T14:00:00Z",
		end:     "2024-01-20T15:00:00Z",
	}
	err = runEventsCreate(context.Background(), conn, "primary", opts, out)
	if err != nil {
		t.Fatalf("runEventsCreate() error = %v", err)
	}

	var result eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Summary != "New Meeting" {
		t.Errorf("expected summary 'New Meeting', got %q", result.Summary)
	}
}

func TestRunEventsCreateAllDay(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "allday-event",
						"summary": "Holiday",
						"start": {"date": "2024-12-25"},
						"end": {"date": "2024-12-26"},
						"status": "confirmed"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := createEventOptions{
		summary: "Holiday",
		start:   "2024-12-25",
		allDay:  true,
	}
	err = runEventsCreate(context.Background(), conn, "primary", opts, out)
	if err != nil {
		t.Fatalf("runEventsCreate() error = %v", err)
	}

	var result eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Summary != "Holiday" {
		t.Errorf("expected summary 'Holiday', got %q", result.Summary)
	}
	if !result.AllDay {
		t.Error("expected event to be all-day")
	}
}

func TestRunEventsCreateMissingSummary(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := createEventOptions{
		summary: "",
		start:   "2024-01-20T14:00:00Z",
	}
	err = runEventsCreate(context.Background(), conn, "primary", opts, out)
	if err == nil {
		t.Fatal("expected error for missing summary, got nil")
	}

	if !strings.Contains(err.Error(), "summary is required") {
		t.Errorf("expected error to contain 'summary is required', got %q", err.Error())
	}
}

func TestRunEventsCreateMissingStart(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := createEventOptions{
		summary: "Meeting",
		start:   "",
	}
	err = runEventsCreate(context.Background(), conn, "primary", opts, out)
	if err == nil {
		t.Fatal("expected error for missing start, got nil")
	}

	if !strings.Contains(err.Error(), "start time is required") {
		t.Errorf("expected error to contain 'start time is required', got %q", err.Error())
	}
}

func TestRunEventsCreateTextOutput(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "new-event",
						"summary": "New Meeting",
						"start": {"dateTime": "2024-01-20T14:00:00Z"},
						"end": {"dateTime": "2024-01-20T15:00:00Z"},
						"status": "confirmed",
						"htmlLink": "https://calendar.google.com/event?eid=xxx"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}

	opts := createEventOptions{
		summary: "New Meeting",
		start:   "2024-01-20T14:00:00Z",
		end:     "2024-01-20T15:00:00Z",
	}
	err = runEventsCreate(context.Background(), conn, "primary", opts, out)
	if err != nil {
		t.Fatalf("runEventsCreate() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Created event:") {
		t.Errorf("expected output to contain 'Created event:', got %q", output)
	}
	if !strings.Contains(output, "New Meeting") {
		t.Errorf("expected output to contain 'New Meeting', got %q", output)
	}
}

func TestRunEventsQuickAdd(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/quickAdd") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "quick-event",
						"summary": "Lunch with Bob",
						"start": {"dateTime": "2024-01-21T12:00:00-08:00"},
						"end": {"dateTime": "2024-01-21T13:00:00-08:00"},
						"status": "confirmed"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsQuickAdd(context.Background(), conn, "primary", "Lunch with Bob tomorrow at noon", out)
	if err != nil {
		t.Fatalf("runEventsQuickAdd() error = %v", err)
	}

	var result eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Summary != "Lunch with Bob" {
		t.Errorf("expected summary 'Lunch with Bob', got %q", result.Summary)
	}
}

func TestRunEventsQuickAddEmptyText(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsQuickAdd(context.Background(), conn, "primary", "", out)
	if err == nil {
		t.Fatal("expected error for empty text, got nil")
	}

	if !strings.Contains(err.Error(), "text is required") {
		t.Errorf("expected error to contain 'text is required', got %q", err.Error())
	}
}

func TestRunEventsQuickAddTextOutput(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/quickAdd") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "quick-event",
						"summary": "Lunch with Bob",
						"start": {"dateTime": "2024-01-21T12:00:00-08:00"},
						"end": {"dateTime": "2024-01-21T13:00:00-08:00"},
						"status": "confirmed",
						"htmlLink": "https://calendar.google.com/event?eid=xxx"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}

	err = runEventsQuickAdd(context.Background(), conn, "primary", "Lunch with Bob tomorrow at noon", out)
	if err != nil {
		t.Fatalf("runEventsQuickAdd() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Created event:") {
		t.Errorf("expected output to contain 'Created event:', got %q", output)
	}
	if !strings.Contains(output, "Lunch with Bob") {
		t.Errorf("expected output to contain 'Lunch with Bob', got %q", output)
	}
}

func TestParseDurationToMinutes(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"15", 15, false},
		{"15m", 15, false},
		{"1h", 60, false},
		{"2h", 120, false},
		{"1d", 1440, false},
		{"2d", 2880, false},
		{"1w", 10080, false},
		{"30m", 30, false},
		{"", 0, true},
		{"abc", 0, true},
		{"1x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDurationToMinutes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDurationToMinutes(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseDurationToMinutes(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseReminderSpec(t *testing.T) {
	tests := []struct {
		input       string
		wantMinutes int64
		wantMethod  string
		wantErr     bool
	}{
		{"15m popup", 15, "popup", false},
		{"1h email", 60, "email", false},
		{"30 popup", 30, "popup", false},
		{"2d email", 2880, "email", false},
		{"1w popup", 10080, "popup", false},
		{"15m", 15, "popup", false}, // Default to popup
		{"", 0, "", true},
		{"invalid", 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			minutes, method, err := parseReminderSpec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseReminderSpec(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if minutes != tt.wantMinutes {
				t.Errorf("parseReminderSpec(%q) minutes = %d, want %d", tt.input, minutes, tt.wantMinutes)
			}
			if method != tt.wantMethod {
				t.Errorf("parseReminderSpec(%q) method = %q, want %q", tt.input, method, tt.wantMethod)
			}
		})
	}
}

func TestParseReminders(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantLen int
		wantErr bool
	}{
		{
			name:    "empty",
			input:   []string{},
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "single reminder",
			input:   []string{"15m popup"},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "multiple reminders",
			input:   []string{"15m popup", "1h email"},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "invalid reminder",
			input:   []string{"invalid"},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reminders, err := parseReminders(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseReminders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(reminders) != tt.wantLen {
				t.Errorf("parseReminders() returned %d reminders, want %d", len(reminders), tt.wantLen)
			}
		})
	}
}

func TestRunEventsCreateWithAttendees(t *testing.T) {
	var capturedBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				body, _ := io.ReadAll(req.Body)
				capturedBody = string(body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "new-event",
						"summary": "Team Meeting",
						"start": {"dateTime": "2024-01-20T14:00:00Z"},
						"end": {"dateTime": "2024-01-20T15:00:00Z"},
						"status": "confirmed",
						"attendees": [
							{"email": "alice@example.com"},
							{"email": "bob@example.com"}
						]
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := createEventOptions{
		summary:   "Team Meeting",
		start:     "2024-01-20T14:00:00Z",
		end:       "2024-01-20T15:00:00Z",
		attendees: []string{"alice@example.com", "bob@example.com"},
	}
	err = runEventsCreate(context.Background(), conn, "primary", opts, out)
	if err != nil {
		t.Fatalf("runEventsCreate() error = %v", err)
	}

	// Verify that attendees were included in the request
	if !strings.Contains(capturedBody, "alice@example.com") {
		t.Errorf("expected request body to contain 'alice@example.com', got %q", capturedBody)
	}
	if !strings.Contains(capturedBody, "bob@example.com") {
		t.Errorf("expected request body to contain 'bob@example.com', got %q", capturedBody)
	}

	var result eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result.Attendees) != 2 {
		t.Errorf("expected 2 attendees in output, got %d", len(result.Attendees))
	}
}

func TestRunEventsCreateWithReminders(t *testing.T) {
	var capturedBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				body, _ := io.ReadAll(req.Body)
				capturedBody = string(body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "new-event",
						"summary": "Team Meeting",
						"start": {"dateTime": "2024-01-20T14:00:00Z"},
						"end": {"dateTime": "2024-01-20T15:00:00Z"},
						"status": "confirmed",
						"reminders": {
							"useDefault": false,
							"overrides": [
								{"method": "popup", "minutes": 15},
								{"method": "email", "minutes": 60}
							]
						}
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := createEventOptions{
		summary:   "Team Meeting",
		start:     "2024-01-20T14:00:00Z",
		end:       "2024-01-20T15:00:00Z",
		reminders: []string{"15m popup", "1h email"},
	}
	err = runEventsCreate(context.Background(), conn, "primary", opts, out)
	if err != nil {
		t.Fatalf("runEventsCreate() error = %v", err)
	}

	// Verify that reminders were included in the request
	// Note: Go's json marshaling omits false boolean values, so we check for
	// the presence of overrides which indicates custom reminders
	if !strings.Contains(capturedBody, `"reminders"`) {
		t.Errorf("expected request body to contain reminders field, got %q", capturedBody)
	}
	if !strings.Contains(capturedBody, `"overrides"`) {
		t.Errorf("expected request body to contain overrides field, got %q", capturedBody)
	}
	if !strings.Contains(capturedBody, `"method":"popup"`) {
		t.Errorf("expected request body to contain popup method, got %q", capturedBody)
	}
	if !strings.Contains(capturedBody, `"method":"email"`) {
		t.Errorf("expected request body to contain email method, got %q", capturedBody)
	}

	var result eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Reminders == nil {
		t.Fatal("expected reminders in output")
	}
	if len(result.Reminders.Overrides) != 2 {
		t.Errorf("expected 2 reminder overrides, got %d", len(result.Reminders.Overrides))
	}
}

func TestRunEventsCreateDefaultEnd(t *testing.T) {
	// Test that when end is not specified, it defaults to 1 hour after start
	var capturedBody string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				body, _ := io.ReadAll(req.Body)
				capturedBody = string(body)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "new-event",
						"summary": "Quick Meeting",
						"start": {"dateTime": "2024-01-20T14:00:00Z"},
						"end": {"dateTime": "2024-01-20T15:00:00Z"},
						"status": "confirmed"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := createEventOptions{
		summary: "Quick Meeting",
		start:   "2024-01-20T14:00:00Z",
		// No end specified - should default to 1 hour later
	}
	err = runEventsCreate(context.Background(), conn, "primary", opts, out)
	if err != nil {
		t.Fatalf("runEventsCreate() error = %v", err)
	}

	// Verify that the end time is 1 hour after start
	if !strings.Contains(capturedBody, "2024-01-20T15:00:00Z") {
		t.Errorf("expected end time to be 1 hour after start, got body: %q", capturedBody)
	}
}

func TestRunEventsCreateDefaultCalendar(t *testing.T) {
	var requestedPath string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestedPath = req.URL.Path
			if req.Method == "POST" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "new-event",
						"summary": "Meeting",
						"start": {"dateTime": "2024-01-20T14:00:00Z"},
						"end": {"dateTime": "2024-01-20T15:00:00Z"},
						"status": "confirmed"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := createEventOptions{
		summary: "Meeting",
		start:   "2024-01-20T14:00:00Z",
	}
	// Pass empty calendarID to test default behavior
	err = runEventsCreate(context.Background(), conn, "", opts, out)
	if err != nil {
		t.Fatalf("runEventsCreate() error = %v", err)
	}

	// Verify that "primary" was used as the default calendar ID
	if !strings.Contains(requestedPath, "/calendars/primary/events") {
		t.Errorf("expected request to use 'primary' calendar, got path %q", requestedPath)
	}
}

func TestRunEventsQuickAddDefaultCalendar(t *testing.T) {
	var requestedPath string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestedPath = req.URL.Path
			if req.Method == "POST" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "quick-event",
						"summary": "Lunch",
						"start": {"dateTime": "2024-01-21T12:00:00-08:00"},
						"end": {"dateTime": "2024-01-21T13:00:00-08:00"},
						"status": "confirmed"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	// Pass empty calendarID to test default behavior
	err = runEventsQuickAdd(context.Background(), conn, "", "Lunch tomorrow at noon", out)
	if err != nil {
		t.Fatalf("runEventsQuickAdd() error = %v", err)
	}

	// Verify that "primary" was used as the default calendar ID
	if !strings.Contains(requestedPath, "/calendars/primary/events/quickAdd") {
		t.Errorf("expected request to use 'primary' calendar, got path %q", requestedPath)
	}
}

func TestRunEventsImport(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/import") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "imported-event",
						"iCalUID": "test-event@example.com",
						"summary": "Imported Meeting",
						"start": {"dateTime": "2024-01-15T10:00:00Z"},
						"end": {"dateTime": "2024-01-15T11:00:00Z"}
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Imported Meeting
END:VEVENT
END:VCALENDAR`

	err = runEventsImport(context.Background(), conn, "primary", strings.NewReader(icsData), false, out)
	if err != nil {
		t.Fatalf("runEventsImport() error = %v", err)
	}

	var result importResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Imported != 1 {
		t.Errorf("expected 1 imported event, got %d", result.Imported)
	}
}

func TestRunEventsImportDryRun(t *testing.T) {
	// In dry-run mode, no API calls should be made
	var apiCallCount int
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			apiCallCount++
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:test-event@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
END:VEVENT
END:VCALENDAR`

	err = runEventsImport(context.Background(), conn, "primary", strings.NewReader(icsData), true, out)
	if err != nil {
		t.Fatalf("runEventsImport() error = %v", err)
	}

	// No API calls should have been made in dry-run mode (beyond connection setup)
	// The connection setup makes some calls, but import-specific calls should be 0
	var result importResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Imported != 1 {
		t.Errorf("expected 1 event (dry run), got %d", result.Imported)
	}
}

func TestRunEventsImportMultipleEvents(t *testing.T) {
	importCount := 0
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/import") {
				importCount++
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "imported-event",
						"summary": "Imported",
						"start": {"dateTime": "2024-01-15T10:00:00Z"},
						"end": {"dateTime": "2024-01-15T11:00:00Z"}
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:event1@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Event 1
END:VEVENT
BEGIN:VEVENT
UID:event2@example.com
DTSTART:20240116T100000Z
DTEND:20240116T110000Z
SUMMARY:Event 2
END:VEVENT
END:VCALENDAR`

	err = runEventsImport(context.Background(), conn, "primary", strings.NewReader(icsData), false, out)
	if err != nil {
		t.Fatalf("runEventsImport() error = %v", err)
	}

	if importCount != 2 {
		t.Errorf("expected 2 import API calls, got %d", importCount)
	}

	var result importResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Imported != 2 {
		t.Errorf("expected 2 imported events, got %d", result.Imported)
	}
}

func TestRunEventsImportEmptyICS(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
END:VCALENDAR`

	err = runEventsImport(context.Background(), conn, "primary", strings.NewReader(icsData), false, out)
	if err != nil {
		t.Fatalf("runEventsImport() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No events found") {
		t.Errorf("expected output to contain 'No events found', got %q", output)
	}
}

func TestRunEventsImportDuplicate(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/import") {
				// Return 409 Conflict for duplicate
				return &http.Response{
					StatusCode: http.StatusConflict,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"error":{"code":409,"message":"Already exists"}}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	icsData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:duplicate@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Duplicate Event
END:VEVENT
END:VCALENDAR`

	err = runEventsImport(context.Background(), conn, "primary", strings.NewReader(icsData), false, out)
	if err != nil {
		t.Fatalf("runEventsImport() error = %v", err)
	}

	var result importResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Duplicate should be counted as skipped
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped event, got %d", result.Skipped)
	}
	if result.Imported != 0 {
		t.Errorf("expected 0 imported events, got %d", result.Imported)
	}
}

func TestRunEventsUpdate(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "PATCH" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/event1") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"id": "event1",
						"summary": "Updated Meeting",
						"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
						"end": {"dateTime": "2024-01-15T11:00:00-08:00"},
						"status": "confirmed"
					}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	opts := updateEventOptions{
		summary: "Updated Meeting",
	}
	err = runEventsUpdate(context.Background(), conn, "primary", "event1", opts, out)
	if err != nil {
		t.Fatalf("runEventsUpdate() error = %v", err)
	}

	var result eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Summary != "Updated Meeting" {
		t.Errorf("expected summary 'Updated Meeting', got %q", result.Summary)
	}
}

func TestRunEventsDelete(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "DELETE" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/event1") {
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(``)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsDelete(context.Background(), conn, "primary", "event1", false, out)
	if err != nil {
		t.Fatalf("runEventsDelete() error = %v", err)
	}
}

func TestRunEventsSearch(t *testing.T) {
	eventsJSON := `{
		"kind": "calendar#events",
		"items": [
			{
				"id": "event1",
				"summary": "Project Review Meeting",
				"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
				"end": {"dateTime": "2024-01-15T11:00:00-08:00"}
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") &&
				strings.Contains(req.URL.RawQuery, "q=review") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(eventsJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsSearch(context.Background(), conn, []string{"primary"}, "review", "", "", 25, out)
	if err != nil {
		t.Fatalf("runEventsSearch() error = %v", err)
	}

	var result []eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 event, got %d", len(result))
	}
}

func TestRunEventsUpdated(t *testing.T) {
	eventsJSON := `{
		"kind": "calendar#events",
		"items": [
			{
				"id": "event1",
				"summary": "Recently Updated Meeting",
				"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
				"end": {"dateTime": "2024-01-15T11:00:00-08:00"},
				"updated": "2024-01-14T08:00:00Z"
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") &&
				strings.Contains(req.URL.RawQuery, "updatedMin=") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(eventsJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsUpdated(context.Background(), conn, "primary", "2024-01-13T00:00:00Z", "", "", 25, out)
	if err != nil {
		t.Fatalf("runEventsUpdated() error = %v", err)
	}

	var result []eventOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 event, got %d", len(result))
	}
}

func TestRunEventsConflicts(t *testing.T) {
	eventsJSON := `{
		"kind": "calendar#events",
		"items": [
			{
				"id": "event1",
				"summary": "Meeting A",
				"start": {"dateTime": "2024-01-15T10:00:00-08:00"},
				"end": {"dateTime": "2024-01-15T11:00:00-08:00"}
			},
			{
				"id": "event2",
				"summary": "Meeting B",
				"start": {"dateTime": "2024-01-15T10:30:00-08:00"},
				"end": {"dateTime": "2024-01-15T11:30:00-08:00"}
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(eventsJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runEventsConflicts(context.Background(), conn, "primary", "", "", out)
	if err != nil {
		t.Fatalf("runEventsConflicts() error = %v", err)
	}

	var result []conflictOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(result))
	}
}
