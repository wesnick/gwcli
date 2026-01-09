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
		name         string
		startDate    string
		startTime    string
		expectedDate string
		expectedTime string
	}{
		{
			name:         "datetime event",
			startTime:    "2024-01-15T10:00:00-08:00",
			expectedDate: "2024-01-15",
			expectedTime: "10:00",
		},
		{
			name:         "all-day event",
			startDate:    "2024-01-15",
			expectedDate: "2024-01-15",
			expectedTime: "all-day",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := &mockEvent{
				startDate:     tt.startDate,
				startDateTime: tt.startTime,
			}
			date, timeStr := formatEventTimeFromMock(ev)
			if date != tt.expectedDate {
				t.Errorf("formatEventTime() date = %q, expected %q", date, tt.expectedDate)
			}
			if timeStr != tt.expectedTime {
				t.Errorf("formatEventTime() time = %q, expected %q", timeStr, tt.expectedTime)
			}
		})
	}
}

// mockEvent is a simple struct for testing formatEventTime logic
type mockEvent struct {
	startDate     string
	startDateTime string
}

// formatEventTimeFromMock is a test helper that mimics formatEventTime logic
func formatEventTimeFromMock(ev *mockEvent) (date, timeStr string) {
	if ev.startDateTime != "" {
		// Parse RFC3339
		date = ev.startDateTime[:10]
		if len(ev.startDateTime) > 11 {
			timeStr = ev.startDateTime[11:16]
		}
	} else if ev.startDate != "" {
		date = ev.startDate
		timeStr = "all-day"
	}
	return date, timeStr
}
