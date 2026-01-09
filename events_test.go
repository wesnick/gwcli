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
