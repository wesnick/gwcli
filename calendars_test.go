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

func TestRunCalendarsList(t *testing.T) {
	calendarsJSON := `{
		"kind": "calendar#calendarList",
		"items": [
			{
				"id": "primary",
				"summary": "user@example.com",
				"primary": true,
				"accessRole": "owner",
				"timeZone": "America/Los_Angeles"
			},
			{
				"id": "work@group.calendar.google.com",
				"summary": "Work Calendar",
				"accessRole": "writer",
				"timeZone": "America/New_York"
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/calendar/v3/users/me/calendarList") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(calendarsJSON)),
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

	err = runCalendarsList(context.Background(), conn, "", out)
	if err != nil {
		t.Fatalf("runCalendarsList() error = %v", err)
	}

	var result []calendarOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 calendars, got %d", len(result))
	}

	if result[0].Summary != "user@example.com" {
		t.Errorf("expected first calendar summary 'user@example.com', got %q", result[0].Summary)
	}

	if !result[0].Primary {
		t.Error("expected first calendar to be primary")
	}
}
