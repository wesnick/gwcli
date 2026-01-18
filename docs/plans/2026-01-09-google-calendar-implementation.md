# Google Calendar Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Google Calendar API support to gwcli with full command coverage for calendars, events, and query operations, following the patterns established by Gmail and Tasks integration.

**Architecture:** Extend the existing authentication infrastructure to include Calendar API scope. Add a `*calendar.Service` field to the `CmdG` struct. Create `calendars.go` and `events.go` command handlers following the existing patterns in `tasks.go` and `tasklists.go`. Implement helper packages under `pkg/gwcli/gcal/` for ICS parsing, reminder handling, and date utilities.

**Tech Stack:** Kong CLI, `google.golang.org/api/calendar/v3`, existing gwcli authentication infrastructure, `outputWriter` pattern for JSON/text output, `github.com/emersion/go-ical` for ICS parsing.

---

## Prerequisites

- gwcli codebase at `/home/wes/Devel/gwcli`
- Reference: `/home/wes/Devel/gwcli/docs/plans/2026-01-09-go-port-requirements.md`
- Go 1.24+
- Existing Tasks integration as pattern reference

---

## Phase 1: Foundation (Tasks 1-3)

### Task 1: Add Calendar API Scope to Authentication

**Files:**
- Modify: `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth.go`
- Test: `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth_test.go`

**Step 1: Write the failing test**

Add to `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth_test.go`:

```go
func TestClientFromCredentialsIncludesCalendarScope(t *testing.T) {
	credJSON := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token"
		}
	}`

	cfg, err := clientFromCredentials(strings.NewReader(credJSON))
	if err != nil {
		t.Fatalf("clientFromCredentials() error = %v", err)
	}

	hasCalendarScope := false
	for _, scope := range cfg.Scopes {
		if scope == "https://www.googleapis.com/auth/calendar" {
			hasCalendarScope = true
			break
		}
	}

	if !hasCalendarScope {
		t.Errorf("clientFromCredentials() missing calendar scope, got scopes: %v", cfg.Scopes)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/gwcli/gmailctl/ -run TestClientFromCredentialsIncludesCalendarScope -v`
Expected: FAIL with "missing calendar scope"

**Step 3: Add Calendar import and scope to OAuth2 config**

In `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth.go`, add import:

```go
import (
	// ... existing imports ...
	"google.golang.org/api/calendar/v3"
)
```

Update `clientFromCredentials` function to add calendar scope:

```go
return google.ConfigFromJSON(credBytes,
	gmail.GmailModifyScope,
	gmail.GmailSettingsBasicScope,
	gmail.GmailLabelsScope,
	tasks.TasksScope,
	calendar.CalendarScope,
)
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/gwcli/gmailctl/ -run TestClientFromCredentialsIncludesCalendarScope -v`
Expected: PASS

**Step 5: Update service account scopes**

In `ServiceAccountAuthenticator.Service()` method, add calendar scope to the JWT config:

```go
config, err := google.JWTConfigFromJSON(
	a.credBytes,
	gmail.GmailModifyScope,
	gmail.GmailSettingsBasicScope,
	gmail.GmailLabelsScope,
	tasks.TasksScope,
	calendar.CalendarScope,
)
```

**Step 6: Run all auth tests**

Run: `go test ./pkg/gwcli/gmailctl/ -v`
Expected: All PASS

**Step 7: Commit**

```bash
git add pkg/gwcli/gmailctl/auth.go pkg/gwcli/gmailctl/auth_test.go
git commit -m "$(cat <<'EOF'
feat: add Calendar API scope to authentication

Add calendar.CalendarScope to OAuth2 and service account configurations
to enable Google Calendar API access alongside Gmail and Tasks.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Add Calendar Service to CmdG Struct

**Files:**
- Modify: `/home/wes/Devel/gwcli/pkg/gwcli/connection.go`
- Modify: `/home/wes/Devel/gwcli/pkg/gwcli/connection_test.go`

**Step 1: Write the failing test**

Add to `/home/wes/Devel/gwcli/pkg/gwcli/connection_test.go`:

```go
func TestCmdGHasCalendarService(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	if conn.CalendarService() == nil {
		t.Error("CalendarService() returned nil, expected *calendar.Service")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/gwcli/ -run TestCmdGHasCalendarService -v`
Expected: FAIL (CalendarService method doesn't exist)

**Step 3: Add calendar import and field to CmdG struct**

In `/home/wes/Devel/gwcli/pkg/gwcli/connection.go`, add import:

```go
import (
	// ... existing imports ...
	"google.golang.org/api/calendar/v3"
)
```

Add field to CmdG struct:

```go
type CmdG struct {
	// ... existing fields ...
	calendarService *calendar.Service
}
```

Add getter method:

```go
// CalendarService returns the Google Calendar API service client.
func (c *CmdG) CalendarService() *calendar.Service {
	return c.calendarService
}
```

**Step 4: Update client initialization to create Calendar service**

Find where `tasksService` is initialized and add Calendar service initialization:

```go
// Initialize Calendar service
calSvc, err := calendar.NewService(context.Background(), option.WithHTTPClient(c.authedClient))
if err != nil {
	return fmt.Errorf("creating calendar service: %w", err)
}
c.calendarService = calSvc
```

**Step 5: Update NewFake to initialize Calendar service**

In the `NewFake()` function, add Calendar service initialization:

```go
calSvc, err := calendar.NewService(ctx, option.WithHTTPClient(client))
if err != nil {
	return nil, fmt.Errorf("creating fake calendar service: %w", err)
}
c.calendarService = calSvc
```

**Step 6: Run test to verify it passes**

Run: `go test ./pkg/gwcli/ -run TestCmdGHasCalendarService -v`
Expected: PASS

**Step 7: Run all connection tests**

Run: `go test ./pkg/gwcli/ -v`
Expected: All PASS

**Step 8: Commit**

```bash
git add pkg/gwcli/connection.go pkg/gwcli/connection_test.go
git commit -m "$(cat <<'EOF'
feat: add Calendar service to CmdG struct

Initialize Google Calendar API service alongside Gmail and Tasks
services in the connection setup, enabling calendar operations.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Create Calendar List Commands

**Files:**
- Create: `/home/wes/Devel/gwcli/calendars.go`
- Create: `/home/wes/Devel/gwcli/calendars_test.go`

**Step 1: Write the failing test for list calendars**

Create `/home/wes/Devel/gwcli/calendars_test.go`:

```go
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunCalendarsList -v`
Expected: FAIL (runCalendarsList doesn't exist)

**Step 3: Create calendars.go with list command**

Create `/home/wes/Devel/gwcli/calendars.go`:

```go
package main

import (
	"context"
	"fmt"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
)

// calendarOutput represents a calendar for JSON output.
type calendarOutput struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	TimeZone    string `json:"timezone,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
	AccessRole  string `json:"accessRole,omitempty"`
	ColorID     string `json:"colorId,omitempty"`
}

// runCalendarsList lists all calendars accessible to the authenticated user.
func runCalendarsList(ctx context.Context, conn *gwcli.CmdG, minAccessRole string, out *outputWriter) error {
	out.writeVerbose("Fetching calendars...")

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	call := svc.CalendarList.List().Context(ctx)
	if minAccessRole != "" {
		call = call.MinAccessRole(minAccessRole)
	}

	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to list calendars: %w", err)
	}

	if out.json {
		output := make([]calendarOutput, len(resp.Items))
		for i, cal := range resp.Items {
			output[i] = calendarOutput{
				ID:          cal.Id,
				Summary:     cal.Summary,
				Description: cal.Description,
				TimeZone:    cal.TimeZone,
				Primary:     cal.Primary,
				AccessRole:  cal.AccessRole,
				ColorID:     cal.ColorId,
			}
		}
		return out.writeJSON(output)
	}

	headers := []string{"SUMMARY", "ACCESS", "TIMEZONE", "ID"}
	rows := make([][]string, len(resp.Items))
	for i, cal := range resp.Items {
		summary := cal.Summary
		if cal.Primary {
			summary = summary + " (primary)"
		}
		rows[i] = []string{
			truncateString(summary, 40),
			cal.AccessRole,
			cal.TimeZone,
			cal.Id,
		}
	}
	return out.writeTable(headers, rows)
}
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunCalendarsList -v`
Expected: PASS

**Step 5: Run all calendar tests**

Run: `go test . -run TestRunCalendars -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add calendars.go calendars_test.go
git commit -m "$(cat <<'EOF'
feat: add calendar list command

Implement calendars list subcommand for listing accessible Google Calendars:
- Lists all calendars with summary, access role, timezone, and ID
- Supports --min-access-role filtering (freeBusyReader, reader, writer, owner)
- JSON and table output formats

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2: Core Event Operations (Tasks 4-7)

### Task 4: Create Event List Command (Agenda Query)

**Files:**
- Create: `/home/wes/Devel/gwcli/events.go`
- Create: `/home/wes/Devel/gwcli/events_test.go`

**Step 1: Write the failing test for list events**

Create `/home/wes/Devel/gwcli/events_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunEventsList -v`
Expected: FAIL (runEventsList doesn't exist)

**Step 3: Create events.go with list command**

Create `/home/wes/Devel/gwcli/events.go`:

```go
package main

import (
	"context"
	"fmt"
	"time"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/calendar/v3"
)

// eventOutput represents an event for JSON output.
type eventOutput struct {
	ID            string            `json:"id"`
	CalendarID    string            `json:"calendarId,omitempty"`
	Summary       string            `json:"summary"`
	Description   string            `json:"description,omitempty"`
	Location      string            `json:"location,omitempty"`
	Start         string            `json:"start"`
	End           string            `json:"end"`
	StartDate     string            `json:"startDate,omitempty"`
	EndDate       string            `json:"endDate,omitempty"`
	AllDay        bool              `json:"allDay,omitempty"`
	Status        string            `json:"status,omitempty"`
	HTMLLink      string            `json:"htmlLink,omitempty"`
	Attendees     []attendeeOutput  `json:"attendees,omitempty"`
	Organizer     *organizerOutput  `json:"organizer,omitempty"`
	Reminders     *remindersOutput  `json:"reminders,omitempty"`
	RecurringID   string            `json:"recurringEventId,omitempty"`
	Recurrence    []string          `json:"recurrence,omitempty"`
	ConferenceURL string            `json:"conferenceUrl,omitempty"`
	Created       string            `json:"created,omitempty"`
	Updated       string            `json:"updated,omitempty"`
}

type attendeeOutput struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"`
	Self           bool   `json:"self,omitempty"`
	Organizer      bool   `json:"organizer,omitempty"`
}

type organizerOutput struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"`
	Self        bool   `json:"self,omitempty"`
}

type remindersOutput struct {
	UseDefault bool              `json:"useDefault"`
	Overrides  []reminderOutput  `json:"overrides,omitempty"`
}

type reminderOutput struct {
	Method  string `json:"method"`
	Minutes int64  `json:"minutes"`
}

// runEventsList lists events in a calendar.
func runEventsList(ctx context.Context, conn *gwcli.CmdG, calendarID, timeMin, timeMax, query string, maxResults int, singleEvents bool, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}

	out.writeVerbose("Fetching events from calendar %s...", calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	call := svc.Events.List(calendarID).Context(ctx)

	if maxResults > 0 {
		call = call.MaxResults(int64(maxResults))
	}

	// Default to single events (expanded recurring events)
	call = call.SingleEvents(singleEvents)

	if timeMin != "" {
		call = call.TimeMin(timeMin)
	} else {
		// Default to now
		call = call.TimeMin(time.Now().Format(time.RFC3339))
	}

	if timeMax != "" {
		call = call.TimeMax(timeMax)
	}

	if query != "" {
		call = call.Q(query)
	}

	// Order by start time when using single events
	if singleEvents {
		call = call.OrderBy("startTime")
	}

	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	if out.json {
		output := make([]eventOutput, len(resp.Items))
		for i, ev := range resp.Items {
			output[i] = eventOutputFromEvent(ev, calendarID)
		}
		return out.writeJSON(output)
	}

	if len(resp.Items) == 0 {
		out.writeMessage("No upcoming events found.")
		return nil
	}

	headers := []string{"DATE", "TIME", "SUMMARY", "ID"}
	rows := make([][]string, len(resp.Items))
	for i, ev := range resp.Items {
		date, timeStr := formatEventTime(ev)
		rows[i] = []string{
			date,
			timeStr,
			truncateString(ev.Summary, 40),
			ev.Id,
		}
	}
	return out.writeTable(headers, rows)
}

// eventOutputFromEvent converts a calendar.Event to eventOutput.
func eventOutputFromEvent(ev *calendar.Event, calendarID string) eventOutput {
	out := eventOutput{
		ID:          ev.Id,
		CalendarID:  calendarID,
		Summary:     ev.Summary,
		Description: ev.Description,
		Location:    ev.Location,
		Status:      ev.Status,
		HTMLLink:    ev.HtmlLink,
		RecurringID: ev.RecurringEventId,
		Recurrence:  ev.Recurrence,
		Created:     ev.Created,
		Updated:     ev.Updated,
	}

	// Handle start/end times
	if ev.Start != nil {
		if ev.Start.DateTime != "" {
			out.Start = ev.Start.DateTime
		} else if ev.Start.Date != "" {
			out.StartDate = ev.Start.Date
			out.AllDay = true
		}
	}
	if ev.End != nil {
		if ev.End.DateTime != "" {
			out.End = ev.End.DateTime
		} else if ev.End.Date != "" {
			out.EndDate = ev.End.Date
		}
	}

	// Attendees
	if len(ev.Attendees) > 0 {
		out.Attendees = make([]attendeeOutput, len(ev.Attendees))
		for i, a := range ev.Attendees {
			out.Attendees[i] = attendeeOutput{
				Email:          a.Email,
				DisplayName:    a.DisplayName,
				ResponseStatus: a.ResponseStatus,
				Self:           a.Self,
				Organizer:      a.Organizer,
			}
		}
	}

	// Organizer
	if ev.Organizer != nil {
		out.Organizer = &organizerOutput{
			Email:       ev.Organizer.Email,
			DisplayName: ev.Organizer.DisplayName,
			Self:        ev.Organizer.Self,
		}
	}

	// Reminders
	if ev.Reminders != nil {
		out.Reminders = &remindersOutput{
			UseDefault: ev.Reminders.UseDefault,
		}
		if len(ev.Reminders.Overrides) > 0 {
			out.Reminders.Overrides = make([]reminderOutput, len(ev.Reminders.Overrides))
			for i, r := range ev.Reminders.Overrides {
				out.Reminders.Overrides[i] = reminderOutput{
					Method:  r.Method,
					Minutes: r.Minutes,
				}
			}
		}
	}

	// Conference data
	if ev.ConferenceData != nil && len(ev.ConferenceData.EntryPoints) > 0 {
		for _, ep := range ev.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" {
				out.ConferenceURL = ep.Uri
				break
			}
		}
	}

	return out
}

// formatEventTime extracts date and time strings for display.
func formatEventTime(ev *calendar.Event) (date, timeStr string) {
	if ev.Start == nil {
		return "", ""
	}

	if ev.Start.DateTime != "" {
		t, err := time.Parse(time.RFC3339, ev.Start.DateTime)
		if err == nil {
			date = t.Format("2006-01-02")
			timeStr = t.Format("15:04")
		} else {
			date = ev.Start.DateTime[:10]
			if len(ev.Start.DateTime) > 11 {
				timeStr = ev.Start.DateTime[11:16]
			}
		}
	} else if ev.Start.Date != "" {
		date = ev.Start.Date
		timeStr = "all-day"
	}

	return date, timeStr
}
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunEventsList -v`
Expected: PASS

**Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "$(cat <<'EOF'
feat: add events list command (agenda query)

Implement events list subcommand for listing calendar events:
- Query events by date range (--time-min, --time-max)
- Full-text search with --query
- Expand recurring events with --single-events
- JSON and table output formats
- Extract all event details including attendees, reminders, conference

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Create Event Read Command

**Files:**
- Modify: `/home/wes/Devel/gwcli/events.go`
- Modify: `/home/wes/Devel/gwcli/events_test.go`

**Step 1: Write the failing test**

Add to `/home/wes/Devel/gwcli/events_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunEventsRead -v`
Expected: FAIL (runEventsRead doesn't exist)

**Step 3: Add read event command**

Add to `/home/wes/Devel/gwcli/events.go`:

```go
// runEventsRead gets details of a single event.
func runEventsRead(ctx context.Context, conn *gwcli.CmdG, calendarID, eventID string, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}
	if eventID == "" {
		return fmt.Errorf("event ID is required")
	}

	out.writeVerbose("Fetching event %s from calendar %s...", eventID, calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	ev, err := svc.Events.Get(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get event: %w", err)
	}

	if out.json {
		return out.writeJSON(eventOutputFromEvent(ev, calendarID))
	}

	// Text output with details
	out.writeMessage(fmt.Sprintf("Summary: %s", ev.Summary))

	date, timeStr := formatEventTime(ev)
	if timeStr == "all-day" {
		out.writeMessage(fmt.Sprintf("Date: %s (all day)", date))
	} else {
		out.writeMessage(fmt.Sprintf("When: %s %s", date, timeStr))
	}

	if ev.Location != "" {
		out.writeMessage(fmt.Sprintf("Location: %s", ev.Location))
	}

	if ev.Description != "" {
		out.writeMessage(fmt.Sprintf("Description: %s", ev.Description))
	}

	if len(ev.Attendees) > 0 {
		out.writeMessage("Attendees:")
		for _, a := range ev.Attendees {
			name := a.Email
			if a.DisplayName != "" {
				name = a.DisplayName
			}
			out.writeMessage(fmt.Sprintf("  - %s (%s)", name, a.ResponseStatus))
		}
	}

	if ev.HtmlLink != "" {
		out.writeMessage(fmt.Sprintf("Link: %s", ev.HtmlLink))
	}

	out.writeMessage(fmt.Sprintf("ID: %s", ev.Id))

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunEventsRead -v`
Expected: PASS

**Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "$(cat <<'EOF'
feat: add events read command

Implement events read subcommand for getting event details:
- Fetch single event by ID
- Display all event fields including attendees
- JSON and text output formats

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Create Event Create Commands (Detailed and Quick Add)

**Files:**
- Modify: `/home/wes/Devel/gwcli/events.go`
- Modify: `/home/wes/Devel/gwcli/events_test.go`

**Step 1: Write the failing test for create event**

Add to `/home/wes/Devel/gwcli/events_test.go`:

```go
func TestRunEventsCreate(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{
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
		summary:     "New Meeting",
		start:       "2024-01-20T14:00:00Z",
		end:         "2024-01-20T15:00:00Z",
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

func TestRunEventsQuickAdd(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/quickAdd") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{
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
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunEventsCreate -v`
Expected: FAIL (runEventsCreate doesn't exist)

**Step 3: Add create event commands**

Add to `/home/wes/Devel/gwcli/events.go`:

```go
// createEventOptions holds options for creating an event.
type createEventOptions struct {
	summary     string
	description string
	location    string
	start       string
	end         string
	allDay      bool
	attendees   []string
	reminders   []string
	colorID     string
}

// runEventsCreate creates a new event with full specification.
func runEventsCreate(ctx context.Context, conn *gwcli.CmdG, calendarID string, opts createEventOptions, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}
	if opts.summary == "" {
		return fmt.Errorf("event summary is required")
	}
	if opts.start == "" {
		return fmt.Errorf("event start time is required")
	}

	out.writeVerbose("Creating event %q in calendar %s...", opts.summary, calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	event := &calendar.Event{
		Summary:     opts.summary,
		Description: opts.description,
		Location:    opts.location,
	}

	// Set start/end times
	if opts.allDay {
		event.Start = &calendar.EventDateTime{Date: opts.start}
		if opts.end != "" {
			event.End = &calendar.EventDateTime{Date: opts.end}
		} else {
			event.End = &calendar.EventDateTime{Date: opts.start}
		}
	} else {
		event.Start = &calendar.EventDateTime{DateTime: opts.start}
		if opts.end != "" {
			event.End = &calendar.EventDateTime{DateTime: opts.end}
		} else {
			// Default to 1 hour duration
			startTime, err := time.Parse(time.RFC3339, opts.start)
			if err == nil {
				endTime := startTime.Add(time.Hour)
				event.End = &calendar.EventDateTime{DateTime: endTime.Format(time.RFC3339)}
			} else {
				event.End = &calendar.EventDateTime{DateTime: opts.start}
			}
		}
	}

	// Add attendees
	if len(opts.attendees) > 0 {
		event.Attendees = make([]*calendar.EventAttendee, len(opts.attendees))
		for i, email := range opts.attendees {
			event.Attendees[i] = &calendar.EventAttendee{Email: email}
		}
	}

	// Set color
	if opts.colorID != "" {
		event.ColorId = opts.colorID
	}

	// Parse and set reminders
	if len(opts.reminders) > 0 {
		reminders, err := parseReminders(opts.reminders)
		if err != nil {
			return fmt.Errorf("invalid reminder format: %w", err)
		}
		event.Reminders = &calendar.EventReminders{
			UseDefault: false,
			Overrides:  reminders,
		}
	}

	created, err := svc.Events.Insert(calendarID, event).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	if out.json {
		return out.writeJSON(eventOutputFromEvent(created, calendarID))
	}

	out.writeMessage(fmt.Sprintf("Created event %q (ID: %s)", created.Summary, created.Id))
	if created.HtmlLink != "" {
		out.writeMessage(fmt.Sprintf("Link: %s", created.HtmlLink))
	}
	return nil
}

// runEventsQuickAdd creates an event using natural language parsing.
func runEventsQuickAdd(ctx context.Context, conn *gwcli.CmdG, calendarID, text string, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}
	if text == "" {
		return fmt.Errorf("event text is required")
	}

	out.writeVerbose("Quick adding event %q to calendar %s...", text, calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	created, err := svc.Events.QuickAdd(calendarID, text).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to quick add event: %w", err)
	}

	if out.json {
		return out.writeJSON(eventOutputFromEvent(created, calendarID))
	}

	out.writeMessage(fmt.Sprintf("Created event %q (ID: %s)", created.Summary, created.Id))
	date, timeStr := formatEventTime(created)
	out.writeMessage(fmt.Sprintf("When: %s %s", date, timeStr))
	if created.HtmlLink != "" {
		out.writeMessage(fmt.Sprintf("Link: %s", created.HtmlLink))
	}
	return nil
}

// parseReminders parses reminder strings like "15m popup", "1h email".
func parseReminders(specs []string) ([]*calendar.EventReminder, error) {
	reminders := make([]*calendar.EventReminder, 0, len(specs))

	for _, spec := range specs {
		r, err := parseReminderSpec(spec)
		if err != nil {
			return nil, err
		}
		reminders = append(reminders, r)
	}

	return reminders, nil
}

// parseReminderSpec parses a single reminder specification.
// Format: <number>[w|d|h|m] [popup|email|sms]
func parseReminderSpec(spec string) (*calendar.EventReminder, error) {
	parts := strings.Fields(spec)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty reminder specification")
	}

	// Parse time component
	timeStr := parts[0]
	method := "popup" // default

	if len(parts) > 1 {
		method = strings.ToLower(parts[1])
	}

	// Parse duration
	minutes, err := parseDurationToMinutes(timeStr)
	if err != nil {
		return nil, err
	}

	// Validate method
	switch method {
	case "popup", "email", "sms":
		// valid
	default:
		return nil, fmt.Errorf("invalid reminder method: %s (must be popup, email, or sms)", method)
	}

	return &calendar.EventReminder{
		Method:  method,
		Minutes: int64(minutes),
	}, nil
}

// parseDurationToMinutes parses duration strings like "15", "15m", "1h", "2d", "1w".
func parseDurationToMinutes(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Check for unit suffix
	lastChar := s[len(s)-1]
	var multiplier int
	var numStr string

	switch lastChar {
	case 'w', 'W':
		multiplier = 7 * 24 * 60 // weeks to minutes
		numStr = s[:len(s)-1]
	case 'd', 'D':
		multiplier = 24 * 60 // days to minutes
		numStr = s[:len(s)-1]
	case 'h', 'H':
		multiplier = 60 // hours to minutes
		numStr = s[:len(s)-1]
	case 'm', 'M':
		multiplier = 1 // already minutes
		numStr = s[:len(s)-1]
	default:
		// No unit, assume minutes
		multiplier = 1
		numStr = s
	}

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %s", numStr)
	}

	return num * multiplier, nil
}
```

Add import for strconv:

```go
import (
	// ... existing imports ...
	"strconv"
	"strings"
)
```

**Step 4: Run tests to verify they pass**

Run: `go test . -run TestRunEventsCreate -v`
Run: `go test . -run TestRunEventsQuickAdd -v`
Expected: Both PASS

**Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "$(cat <<'EOF'
feat: add events create and quickadd commands

Implement event creation subcommands:
- events create: Full event specification with all fields
- events quickadd: Natural language event creation via Google API
- Support for attendees, reminders, colors, all-day events
- Reminder parsing with human-readable format (15m, 1h, 2d, 1w)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Create Event Update and Delete Commands

**Files:**
- Modify: `/home/wes/Devel/gwcli/events.go`
- Modify: `/home/wes/Devel/gwcli/events_test.go`

**Step 1: Write the failing tests**

Add to `/home/wes/Devel/gwcli/events_test.go`:

```go
func TestRunEventsUpdate(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "PATCH" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/event1") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{
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
```

**Step 2: Run tests to verify they fail**

Run: `go test . -run TestRunEventsUpdate -v`
Run: `go test . -run TestRunEventsDelete -v`
Expected: Both FAIL

**Step 3: Add update and delete commands**

Add to `/home/wes/Devel/gwcli/events.go`:

```go
// updateEventOptions holds options for updating an event.
type updateEventOptions struct {
	summary     string
	description string
	location    string
	start       string
	end         string
	colorID     string
}

// runEventsUpdate updates an existing event via PATCH.
func runEventsUpdate(ctx context.Context, conn *gwcli.CmdG, calendarID, eventID string, opts updateEventOptions, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}
	if eventID == "" {
		return fmt.Errorf("event ID is required")
	}

	out.writeVerbose("Updating event %s in calendar %s...", eventID, calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	// Build patch object with only specified fields
	event := &calendar.Event{}
	hasChanges := false

	if opts.summary != "" {
		event.Summary = opts.summary
		hasChanges = true
	}
	if opts.description != "" {
		event.Description = opts.description
		hasChanges = true
	}
	if opts.location != "" {
		event.Location = opts.location
		hasChanges = true
	}
	if opts.start != "" {
		event.Start = &calendar.EventDateTime{DateTime: opts.start}
		hasChanges = true
	}
	if opts.end != "" {
		event.End = &calendar.EventDateTime{DateTime: opts.end}
		hasChanges = true
	}
	if opts.colorID != "" {
		event.ColorId = opts.colorID
		hasChanges = true
	}

	if !hasChanges {
		return fmt.Errorf("no changes specified")
	}

	updated, err := svc.Events.Patch(calendarID, eventID, event).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}

	if out.json {
		return out.writeJSON(eventOutputFromEvent(updated, calendarID))
	}

	out.writeMessage(fmt.Sprintf("Updated event %q (ID: %s)", updated.Summary, updated.Id))
	return nil
}

// runEventsDelete deletes an event.
func runEventsDelete(ctx context.Context, conn *gwcli.CmdG, calendarID, eventID string, force bool, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}
	if eventID == "" {
		return fmt.Errorf("event ID is required")
	}

	out.writeVerbose("Deleting event %s from calendar %s...", eventID, calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	if err := svc.Events.Delete(calendarID, eventID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	if out.json {
		return out.writeJSON(map[string]string{"deleted": eventID})
	}

	out.writeMessage(fmt.Sprintf("Deleted event %s", eventID))
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test . -run TestRunEventsUpdate -v`
Run: `go test . -run TestRunEventsDelete -v`
Expected: Both PASS

**Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "$(cat <<'EOF'
feat: add events update and delete commands

Implement event modification subcommands:
- events update: PATCH semantics for selective field updates
- events delete: Remove event by ID with optional --force

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3: Advanced Queries (Tasks 8-10)

### Task 8: Create Text Search Query Command

**Files:**
- Modify: `/home/wes/Devel/gwcli/events.go`
- Modify: `/home/wes/Devel/gwcli/events_test.go`

**Step 1: Write the failing test**

Add to `/home/wes/Devel/gwcli/events_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunEventsSearch -v`
Expected: FAIL (runEventsSearch doesn't exist)

**Step 3: Add search command**

Add to `/home/wes/Devel/gwcli/events.go`:

```go
// runEventsSearch searches for events across calendars.
func runEventsSearch(ctx context.Context, conn *gwcli.CmdG, calendarIDs []string, query, timeMin, timeMax string, maxResults int, out *outputWriter) error {
	if query == "" {
		return fmt.Errorf("search query is required")
	}
	if len(calendarIDs) == 0 {
		calendarIDs = []string{"primary"}
	}

	out.writeVerbose("Searching for %q across %d calendars...", query, len(calendarIDs))

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	var allEvents []eventOutput

	for _, calID := range calendarIDs {
		call := svc.Events.List(calID).Context(ctx).
			Q(query).
			SingleEvents(true).
			OrderBy("startTime")

		if maxResults > 0 {
			call = call.MaxResults(int64(maxResults))
		}
		if timeMin != "" {
			call = call.TimeMin(timeMin)
		}
		if timeMax != "" {
			call = call.TimeMax(timeMax)
		}

		resp, err := call.Do()
		if err != nil {
			out.writeVerbose("Warning: failed to search calendar %s: %v", calID, err)
			continue
		}

		for _, ev := range resp.Items {
			allEvents = append(allEvents, eventOutputFromEvent(ev, calID))
		}
	}

	if out.json {
		return out.writeJSON(allEvents)
	}

	if len(allEvents) == 0 {
		out.writeMessage("No matching events found.")
		return nil
	}

	headers := []string{"DATE", "TIME", "SUMMARY", "CALENDAR", "ID"}
	rows := make([][]string, len(allEvents))
	for i, ev := range allEvents {
		date := ""
		timeStr := ""
		if ev.Start != "" {
			t, err := time.Parse(time.RFC3339, ev.Start)
			if err == nil {
				date = t.Format("2006-01-02")
				timeStr = t.Format("15:04")
			}
		} else if ev.StartDate != "" {
			date = ev.StartDate
			timeStr = "all-day"
		}
		rows[i] = []string{
			date,
			timeStr,
			truncateString(ev.Summary, 35),
			truncateString(ev.CalendarID, 20),
			ev.ID,
		}
	}
	return out.writeTable(headers, rows)
}
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunEventsSearch -v`
Expected: PASS

**Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "$(cat <<'EOF'
feat: add events search command

Implement full-text search across calendars:
- Search by query text across event title, description, location
- Support searching multiple calendars
- Optional date range filtering

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Create Updates Query Command

**Files:**
- Modify: `/home/wes/Devel/gwcli/events.go`
- Modify: `/home/wes/Devel/gwcli/events_test.go`

**Step 1: Write the failing test**

Add to `/home/wes/Devel/gwcli/events_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunEventsUpdated -v`
Expected: FAIL (runEventsUpdated doesn't exist)

**Step 3: Add updated events command**

Add to `/home/wes/Devel/gwcli/events.go`:

```go
// runEventsUpdated finds events modified since a specific time.
func runEventsUpdated(ctx context.Context, conn *gwcli.CmdG, calendarID, updatedMin, timeMin, timeMax string, maxResults int, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}
	if updatedMin == "" {
		return fmt.Errorf("updated-min timestamp is required")
	}

	out.writeVerbose("Fetching events updated since %s from calendar %s...", updatedMin, calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	call := svc.Events.List(calendarID).Context(ctx).
		UpdatedMin(updatedMin).
		SingleEvents(true).
		OrderBy("updated")

	if maxResults > 0 {
		call = call.MaxResults(int64(maxResults))
	}
	if timeMin != "" {
		call = call.TimeMin(timeMin)
	}
	if timeMax != "" {
		call = call.TimeMax(timeMax)
	}

	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to list updated events: %w", err)
	}

	if out.json {
		output := make([]eventOutput, len(resp.Items))
		for i, ev := range resp.Items {
			output[i] = eventOutputFromEvent(ev, calendarID)
		}
		return out.writeJSON(output)
	}

	if len(resp.Items) == 0 {
		out.writeMessage("No events updated since %s", updatedMin)
		return nil
	}

	headers := []string{"UPDATED", "SUMMARY", "STATUS", "ID"}
	rows := make([][]string, len(resp.Items))
	for i, ev := range resp.Items {
		updated := ""
		if ev.Updated != "" {
			t, err := time.Parse(time.RFC3339, ev.Updated)
			if err == nil {
				updated = t.Format("2006-01-02 15:04")
			} else {
				updated = ev.Updated[:16]
			}
		}
		rows[i] = []string{
			updated,
			truncateString(ev.Summary, 40),
			ev.Status,
			ev.Id,
		}
	}
	return out.writeTable(headers, rows)
}
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunEventsUpdated -v`
Expected: PASS

**Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "$(cat <<'EOF'
feat: add events updated command

Implement query for recently modified events:
- Find events updated since a timestamp
- Useful for sync/polling scenarios
- Optional date range filtering

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Create Conflicts Query Command

**Files:**
- Modify: `/home/wes/Devel/gwcli/events.go`
- Modify: `/home/wes/Devel/gwcli/events_test.go`

**Step 1: Write the failing test**

Add to `/home/wes/Devel/gwcli/events_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunEventsConflicts -v`
Expected: FAIL (runEventsConflicts doesn't exist)

**Step 3: Add conflicts detection command**

Add to `/home/wes/Devel/gwcli/events.go`:

```go
// conflictOutput represents a scheduling conflict.
type conflictOutput struct {
	Event1 eventOutput `json:"event1"`
	Event2 eventOutput `json:"event2"`
}

// runEventsConflicts detects scheduling conflicts (overlapping events).
func runEventsConflicts(ctx context.Context, conn *gwcli.CmdG, calendarID, timeMin, timeMax string, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}

	out.writeVerbose("Checking for conflicts in calendar %s...", calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	call := svc.Events.List(calendarID).Context(ctx).
		SingleEvents(true).
		OrderBy("startTime")

	// Default to 30 days if no range specified
	if timeMin == "" {
		timeMin = time.Now().Format(time.RFC3339)
	}
	call = call.TimeMin(timeMin)

	if timeMax == "" {
		timeMax = time.Now().AddDate(0, 0, 30).Format(time.RFC3339)
	}
	call = call.TimeMax(timeMax)

	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	// Find overlapping events
	conflicts := findConflicts(resp.Items, calendarID)

	if out.json {
		return out.writeJSON(conflicts)
	}

	if len(conflicts) == 0 {
		out.writeMessage("No scheduling conflicts found.")
		return nil
	}

	out.writeMessage(fmt.Sprintf("Found %d conflicts:\n", len(conflicts)))
	for i, c := range conflicts {
		out.writeMessage(fmt.Sprintf("Conflict %d:", i+1))
		out.writeMessage(fmt.Sprintf("  Event 1: %s (%s)", c.Event1.Summary, c.Event1.Start))
		out.writeMessage(fmt.Sprintf("  Event 2: %s (%s)", c.Event2.Summary, c.Event2.Start))
		out.writeMessage("")
	}
	return nil
}

// findConflicts detects overlapping events.
func findConflicts(events []*calendar.Event, calendarID string) []conflictOutput {
	var conflicts []conflictOutput

	for i := 0; i < len(events); i++ {
		ev1 := events[i]
		start1, end1 := parseEventTimes(ev1)
		if start1.IsZero() || end1.IsZero() {
			continue
		}

		for j := i + 1; j < len(events); j++ {
			ev2 := events[j]
			start2, end2 := parseEventTimes(ev2)
			if start2.IsZero() || end2.IsZero() {
				continue
			}

			// Events are sorted by start time, so if ev2 starts after ev1 ends, no more conflicts for ev1
			if start2.After(end1) || start2.Equal(end1) {
				break
			}

			// Check overlap: ev1 overlaps ev2 if start1 < end2 && start2 < end1
			if start1.Before(end2) && start2.Before(end1) {
				conflicts = append(conflicts, conflictOutput{
					Event1: eventOutputFromEvent(ev1, calendarID),
					Event2: eventOutputFromEvent(ev2, calendarID),
				})
			}
		}
	}

	return conflicts
}

// parseEventTimes extracts start and end times from an event.
func parseEventTimes(ev *calendar.Event) (start, end time.Time) {
	if ev.Start == nil || ev.End == nil {
		return
	}

	if ev.Start.DateTime != "" {
		start, _ = time.Parse(time.RFC3339, ev.Start.DateTime)
	}
	if ev.End.DateTime != "" {
		end, _ = time.Parse(time.RFC3339, ev.End.DateTime)
	}

	return start, end
}
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunEventsConflicts -v`
Expected: PASS

**Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "$(cat <<'EOF'
feat: add events conflicts command

Implement scheduling conflict detection:
- Detect overlapping events in date range
- Default 30-day lookahead
- Efficient algorithm using sorted events

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4: ICS Import (Tasks 11-12)

### Task 11: Create ICS Parser Package

**Files:**
- Create: `/home/wes/Devel/gwcli/pkg/gwcli/gcal/ics.go`
- Create: `/home/wes/Devel/gwcli/pkg/gwcli/gcal/ics_test.go`

**Step 1: Add go-ical dependency**

Run: `go get github.com/emersion/go-ical`

**Step 2: Write the failing test**

Create `/home/wes/Devel/gwcli/pkg/gwcli/gcal/ics_test.go`:

```go
package gcal

import (
	"strings"
	"testing"
)

func TestParseICS(t *testing.T) {
	icsData := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Test//Test//EN
BEGIN:VEVENT
UID:test-event-1@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Test Meeting
LOCATION:Conference Room A
DESCRIPTION:Test description
END:VEVENT
END:VCALENDAR`

	events, err := ParseICS(strings.NewReader(icsData))
	if err != nil {
		t.Fatalf("ParseICS() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	ev := events[0]
	if ev.Summary != "Test Meeting" {
		t.Errorf("expected summary 'Test Meeting', got %q", ev.Summary)
	}
	if ev.Location != "Conference Room A" {
		t.Errorf("expected location 'Conference Room A', got %q", ev.Location)
	}
	if ev.ICalUID != "test-event-1@example.com" {
		t.Errorf("expected UID 'test-event-1@example.com', got %q", ev.ICalUID)
	}
}

func TestParseICSWithRecurrence(t *testing.T) {
	icsData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:recurring@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Weekly Meeting
RRULE:FREQ=WEEKLY;BYDAY=MO
END:VEVENT
END:VCALENDAR`

	events, err := ParseICS(strings.NewReader(icsData))
	if err != nil {
		t.Fatalf("ParseICS() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if len(events[0].Recurrence) == 0 {
		t.Error("expected recurrence rule")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./pkg/gwcli/gcal/ -run TestParseICS -v`
Expected: FAIL (package/function doesn't exist)

**Step 4: Create ICS parser**

Create `/home/wes/Devel/gwcli/pkg/gwcli/gcal/ics.go`:

```go
package gcal

import (
	"fmt"
	"io"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
	"google.golang.org/api/calendar/v3"
)

// ParsedEvent represents an event parsed from ICS format.
type ParsedEvent struct {
	*calendar.Event
	ICalUID  string
	Sequence int64
}

// ParseICS parses ICS/iCalendar data and returns parsed events.
func ParseICS(r io.Reader) ([]*ParsedEvent, error) {
	dec := ical.NewDecoder(r)

	var events []*ParsedEvent

	for {
		cal, err := dec.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decoding calendar: %w", err)
		}

		for _, component := range cal.Children {
			if component.Name != ical.CompEvent {
				continue
			}

			ev, err := parseVEvent(component)
			if err != nil {
				return nil, fmt.Errorf("parsing event: %w", err)
			}

			events = append(events, ev)
		}
	}

	return events, nil
}

// parseVEvent converts an iCal VEVENT component to a ParsedEvent.
func parseVEvent(comp *ical.Component) (*ParsedEvent, error) {
	ev := &ParsedEvent{
		Event: &calendar.Event{},
	}

	// UID
	if prop := comp.Props.Get(ical.PropUID); prop != nil {
		ev.ICalUID = prop.Value
		ev.Event.ICalUID = prop.Value
	}

	// Summary (title)
	if prop := comp.Props.Get(ical.PropSummary); prop != nil {
		ev.Event.Summary = prop.Value
	}

	// Description
	if prop := comp.Props.Get(ical.PropDescription); prop != nil {
		ev.Event.Description = prop.Value
	}

	// Location
	if prop := comp.Props.Get(ical.PropLocation); prop != nil {
		ev.Event.Location = prop.Value
	}

	// Start time
	if prop := comp.Props.Get(ical.PropDateTimeStart); prop != nil {
		dt, isAllDay, err := parseICalDateTime(prop)
		if err != nil {
			return nil, fmt.Errorf("parsing start time: %w", err)
		}
		ev.Event.Start = &calendar.EventDateTime{}
		if isAllDay {
			ev.Event.Start.Date = dt.Format("2006-01-02")
		} else {
			ev.Event.Start.DateTime = dt.Format(time.RFC3339)
		}
	}

	// End time
	if prop := comp.Props.Get(ical.PropDateTimeEnd); prop != nil {
		dt, isAllDay, err := parseICalDateTime(prop)
		if err != nil {
			return nil, fmt.Errorf("parsing end time: %w", err)
		}
		ev.Event.End = &calendar.EventDateTime{}
		if isAllDay {
			ev.Event.End.Date = dt.Format("2006-01-02")
		} else {
			ev.Event.End.DateTime = dt.Format(time.RFC3339)
		}
	}

	// Duration (if no end time)
	if ev.Event.End == nil {
		if prop := comp.Props.Get(ical.PropDuration); prop != nil {
			dur, err := parseDuration(prop.Value)
			if err == nil && ev.Event.Start != nil {
				var startTime time.Time
				if ev.Event.Start.DateTime != "" {
					startTime, _ = time.Parse(time.RFC3339, ev.Event.Start.DateTime)
				}
				if !startTime.IsZero() {
					endTime := startTime.Add(dur)
					ev.Event.End = &calendar.EventDateTime{
						DateTime: endTime.Format(time.RFC3339),
					}
				}
			}
		}
	}

	// Recurrence rules
	for _, prop := range comp.Props.Values(ical.PropRecurrenceRule) {
		ev.Event.Recurrence = append(ev.Event.Recurrence, "RRULE:"+prop.Value)
	}

	// Sequence number
	if prop := comp.Props.Get(ical.PropSequence); prop != nil {
		var seq int
		fmt.Sscanf(prop.Value, "%d", &seq)
		ev.Sequence = int64(seq)
		ev.Event.Sequence = int64(seq)
	}

	// Status
	if prop := comp.Props.Get(ical.PropStatus); prop != nil {
		status := strings.ToLower(prop.Value)
		switch status {
		case "confirmed":
			ev.Event.Status = "confirmed"
		case "tentative":
			ev.Event.Status = "tentative"
		case "cancelled":
			ev.Event.Status = "cancelled"
		}
	}

	// Attendees
	for _, prop := range comp.Props.Values(ical.PropAttendee) {
		email := prop.Value
		if strings.HasPrefix(strings.ToLower(email), "mailto:") {
			email = email[7:]
		}

		attendee := &calendar.EventAttendee{
			Email: email,
		}

		if cn := prop.Params.Get("CN"); cn != "" {
			attendee.DisplayName = cn
		}

		if partstat := prop.Params.Get("PARTSTAT"); partstat != "" {
			switch strings.ToUpper(partstat) {
			case "ACCEPTED":
				attendee.ResponseStatus = "accepted"
			case "DECLINED":
				attendee.ResponseStatus = "declined"
			case "TENTATIVE":
				attendee.ResponseStatus = "tentative"
			default:
				attendee.ResponseStatus = "needsAction"
			}
		}

		ev.Event.Attendees = append(ev.Event.Attendees, attendee)
	}

	// Organizer
	if prop := comp.Props.Get(ical.PropOrganizer); prop != nil {
		email := prop.Value
		if strings.HasPrefix(strings.ToLower(email), "mailto:") {
			email = email[7:]
		}
		ev.Event.Organizer = &calendar.EventOrganizer{
			Email: email,
		}
		if cn := prop.Params.Get("CN"); cn != "" {
			ev.Event.Organizer.DisplayName = cn
		}
	}

	return ev, nil
}

// parseICalDateTime parses an iCal date/time property.
func parseICalDateTime(prop *ical.Prop) (time.Time, bool, error) {
	value := prop.Value

	// Check for VALUE=DATE (all-day event)
	if prop.Params.Get("VALUE") == "DATE" {
		t, err := time.Parse("20060102", value)
		return t, true, err
	}

	// Check for TZID
	tzid := prop.Params.Get("TZID")

	// Try various formats
	formats := []string{
		"20060102T150405Z",     // UTC
		"20060102T150405",      // Local time
		"20060102",             // Date only
	}

	for _, format := range formats {
		t, err := time.Parse(format, value)
		if err == nil {
			if tzid != "" {
				loc, err := time.LoadLocation(tzid)
				if err == nil {
					t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
				}
			}
			isAllDay := len(value) == 8
			return t, isAllDay, nil
		}
	}

	return time.Time{}, false, fmt.Errorf("unable to parse datetime: %s", value)
}

// parseDuration parses an iCal duration (ISO 8601).
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Simple parsing for common formats like PT1H, PT30M, P1D
	var dur time.Duration

	s = strings.TrimPrefix(s, "P")
	s = strings.TrimPrefix(s, "T")

	for len(s) > 0 {
		var num int
		var unit rune
		n, _ := fmt.Sscanf(s, "%d%c", &num, &unit)
		if n != 2 {
			break
		}

		switch unit {
		case 'D':
			dur += time.Duration(num) * 24 * time.Hour
		case 'H':
			dur += time.Duration(num) * time.Hour
		case 'M':
			dur += time.Duration(num) * time.Minute
		case 'S':
			dur += time.Duration(num) * time.Second
		}

		// Move past the parsed portion
		idx := strings.IndexAny(s, "DHMS")
		if idx >= 0 {
			s = s[idx+1:]
		} else {
			break
		}
	}

	return dur, nil
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./pkg/gwcli/gcal/ -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add pkg/gwcli/gcal/ics.go pkg/gwcli/gcal/ics_test.go go.mod go.sum
git commit -m "$(cat <<'EOF'
feat: add ICS parser package for calendar import

Implement ICS/iCalendar parsing:
- Parse VCALENDAR containers with VEVENT components
- Extract all standard fields (summary, location, description, etc.)
- Handle date-time formats with timezone support
- Parse recurrence rules (RRULE)
- Parse attendees and organizer
- Support duration as alternative to end time

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: Create Events Import Command

**Files:**
- Modify: `/home/wes/Devel/gwcli/events.go`
- Modify: `/home/wes/Devel/gwcli/events_test.go`

**Step 1: Write the failing test**

Add to `/home/wes/Devel/gwcli/events_test.go`:

```go
func TestRunEventsImport(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/calendar/v3/calendars/primary/events/import") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{
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
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunEventsImport -v`
Expected: FAIL (runEventsImport doesn't exist)

**Step 3: Add import command**

Add to `/home/wes/Devel/gwcli/events.go`:

```go
import (
	// ... existing imports ...
	"github.com/wesnick/gwcli/pkg/gwcli/gcal"
)

// importResult represents the result of an import operation.
type importResult struct {
	Imported   int      `json:"imported"`
	Skipped    int      `json:"skipped"`
	Failed     int      `json:"failed"`
	FailedUIDs []string `json:"failedUids,omitempty"`
}

// runEventsImport imports events from ICS format.
func runEventsImport(ctx context.Context, conn *gwcli.CmdG, calendarID string, r io.Reader, dryRun bool, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}

	out.writeVerbose("Parsing ICS data...")

	// Parse ICS
	events, err := gcal.ParseICS(r)
	if err != nil {
		return fmt.Errorf("failed to parse ICS: %w", err)
	}

	if len(events) == 0 {
		out.writeMessage("No events found in ICS data.")
		return nil
	}

	out.writeVerbose("Found %d events to import", len(events))

	if dryRun {
		out.writeMessage(fmt.Sprintf("Dry run: would import %d events", len(events)))
		if out.json {
			return out.writeJSON(importResult{Imported: len(events)})
		}
		return nil
	}

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	result := importResult{}

	for _, ev := range events {
		// Use import API if event has iCalUID
		if ev.ICalUID != "" {
			_, err := svc.Events.Import(calendarID, ev.Event).Context(ctx).Do()
			if err != nil {
				// Check for duplicate (409 Conflict)
				if strings.Contains(err.Error(), "409") {
					result.Skipped++
					out.writeVerbose("Skipped duplicate: %s", ev.ICalUID)
					continue
				}
				result.Failed++
				result.FailedUIDs = append(result.FailedUIDs, ev.ICalUID)
				out.writeVerbose("Failed to import %s: %v", ev.ICalUID, err)
				continue
			}
			result.Imported++
			out.writeVerbose("Imported: %s", ev.Event.Summary)
		} else {
			// Fall back to insert for events without UID
			_, err := svc.Events.Insert(calendarID, ev.Event).Context(ctx).Do()
			if err != nil {
				result.Failed++
				out.writeVerbose("Failed to insert event: %v", err)
				continue
			}
			result.Imported++
		}
	}

	if out.json {
		return out.writeJSON(result)
	}

	out.writeMessage(fmt.Sprintf("Import complete: %d imported, %d skipped, %d failed",
		result.Imported, result.Skipped, result.Failed))
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunEventsImport -v`
Expected: PASS

**Step 5: Commit**

```bash
git add events.go events_test.go
git commit -m "$(cat <<'EOF'
feat: add events import command for ICS files

Implement ICS/iCalendar import:
- Parse ICS files and import events to calendar
- Graceful import via import API (preserves iCalUID, avoids duplicates)
- Fallback to insert for events without UID
- Dry-run mode for validation
- Report imported/skipped/failed counts

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5: CLI Wiring (Tasks 13-14)

### Task 13: Wire Calendar Commands into Kong CLI

**Files:**
- Modify: `/home/wes/Devel/gwcli/main.go`

**Step 1: Add Calendars command struct to CLI**

Find the CLI struct in `main.go` and add:

```go
Calendars struct {
	List struct {
		MinAccessRole string `name:"min-access-role" help:"Minimum access role (freeBusyReader, reader, writer, owner)"`
	} `cmd:"" help:"List all accessible calendars"`
} `cmd:"" help:"Google Calendar operations"`
```

**Step 2: Add Events command struct to CLI**

```go
Events struct {
	List struct {
		CalendarID   string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		TimeMin      string `name:"time-min" help:"Start time (RFC3339)"`
		TimeMax      string `name:"time-max" help:"End time (RFC3339)"`
		Query        string `name:"query" short:"q" help:"Search query"`
		MaxResults   int    `name:"max-results" default:"25" help:"Maximum events to return"`
		SingleEvents bool   `name:"single-events" default:"true" help:"Expand recurring events"`
	} `cmd:"" help:"List events in a calendar"`

	Read struct {
		CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		EventID    string `arg:"" required:"" help:"Event ID"`
	} `cmd:"" help:"Get event details"`

	Create struct {
		CalendarID  string   `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		Summary     string   `name:"summary" short:"s" required:"" help:"Event title"`
		Description string   `name:"description" short:"d" help:"Event description"`
		Location    string   `name:"location" short:"l" help:"Event location"`
		Start       string   `name:"start" required:"" help:"Start time (RFC3339)"`
		End         string   `name:"end" help:"End time (RFC3339, default: start + 1 hour)"`
		AllDay      bool     `name:"all-day" help:"Create all-day event"`
		Attendees   []string `name:"attendee" short:"a" help:"Attendee email (can repeat)"`
		Reminders   []string `name:"reminder" short:"r" help:"Reminder spec (e.g., '15m popup', '1h email')"`
		ColorID     string   `name:"color" help:"Event color ID"`
	} `cmd:"" help:"Create a new event"`

	QuickAdd struct {
		CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		Text       string `arg:"" required:"" help:"Natural language event description"`
	} `cmd:"" name:"quickadd" help:"Create event from natural language"`

	Update struct {
		CalendarID  string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		EventID     string `arg:"" required:"" help:"Event ID to update"`
		Summary     string `name:"summary" short:"s" help:"New event title"`
		Description string `name:"description" short:"d" help:"New event description"`
		Location    string `name:"location" short:"l" help:"New event location"`
		Start       string `name:"start" help:"New start time (RFC3339)"`
		End         string `name:"end" help:"New end time (RFC3339)"`
		ColorID     string `name:"color" help:"New event color ID"`
	} `cmd:"" help:"Update an event"`

	Delete struct {
		CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		EventID    string `arg:"" required:"" help:"Event ID to delete"`
		Force      bool   `name:"force" short:"f" help:"Skip confirmation"`
	} `cmd:"" help:"Delete an event"`

	Search struct {
		Query       string   `arg:"" required:"" help:"Search query"`
		CalendarIDs []string `name:"calendar" short:"c" help:"Calendar IDs to search (can repeat)"`
		TimeMin     string   `name:"time-min" help:"Start time (RFC3339)"`
		TimeMax     string   `name:"time-max" help:"End time (RFC3339)"`
		MaxResults  int      `name:"max-results" default:"25" help:"Maximum events to return"`
	} `cmd:"" help:"Search for events across calendars"`

	Updated struct {
		CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		UpdatedMin string `name:"updated-min" required:"" help:"Return events updated after this time (RFC3339)"`
		TimeMin    string `name:"time-min" help:"Start time (RFC3339)"`
		TimeMax    string `name:"time-max" help:"End time (RFC3339)"`
		MaxResults int    `name:"max-results" default:"25" help:"Maximum events to return"`
	} `cmd:"" help:"Find events modified since a timestamp"`

	Conflicts struct {
		CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		TimeMin    string `name:"time-min" help:"Start time (RFC3339, default: now)"`
		TimeMax    string `name:"time-max" help:"End time (RFC3339, default: now + 30 days)"`
	} `cmd:"" help:"Detect scheduling conflicts"`

	Import struct {
		CalendarID string `arg:"" optional:"" help:"Calendar ID (default: primary)"`
		File       string `name:"file" short:"f" required:"" help:"ICS file path (- for stdin)"`
		DryRun     bool   `name:"dry-run" help:"Parse and validate without importing"`
	} `cmd:"" help:"Import events from ICS file"`
} `cmd:"" help:"Google Calendar event operations"`
```

**Step 3: Add case handlers in the switch statement**

Add to the `switch ctx.Command()` block:

```go
// Calendar commands
case "calendars list":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	if err := runCalendarsList(context.Background(), conn, cli.Calendars.List.MinAccessRole, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

// Event commands
case "events list", "events list <calendar-id>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	if err := runEventsList(context.Background(), conn, cli.Events.List.CalendarID,
		cli.Events.List.TimeMin, cli.Events.List.TimeMax, cli.Events.List.Query,
		cli.Events.List.MaxResults, cli.Events.List.SingleEvents, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events read <event-id>", "events read <calendar-id> <event-id>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	if err := runEventsRead(context.Background(), conn, cli.Events.Read.CalendarID,
		cli.Events.Read.EventID, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events create", "events create <calendar-id>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	opts := createEventOptions{
		summary:     cli.Events.Create.Summary,
		description: cli.Events.Create.Description,
		location:    cli.Events.Create.Location,
		start:       cli.Events.Create.Start,
		end:         cli.Events.Create.End,
		allDay:      cli.Events.Create.AllDay,
		attendees:   cli.Events.Create.Attendees,
		reminders:   cli.Events.Create.Reminders,
		colorID:     cli.Events.Create.ColorID,
	}
	if err := runEventsCreate(context.Background(), conn, cli.Events.Create.CalendarID,
		opts, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events quickadd <text>", "events quickadd <calendar-id> <text>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	if err := runEventsQuickAdd(context.Background(), conn, cli.Events.QuickAdd.CalendarID,
		cli.Events.QuickAdd.Text, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events update <event-id>", "events update <calendar-id> <event-id>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	opts := updateEventOptions{
		summary:     cli.Events.Update.Summary,
		description: cli.Events.Update.Description,
		location:    cli.Events.Update.Location,
		start:       cli.Events.Update.Start,
		end:         cli.Events.Update.End,
		colorID:     cli.Events.Update.ColorID,
	}
	if err := runEventsUpdate(context.Background(), conn, cli.Events.Update.CalendarID,
		cli.Events.Update.EventID, opts, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events delete <event-id>", "events delete <calendar-id> <event-id>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	if err := runEventsDelete(context.Background(), conn, cli.Events.Delete.CalendarID,
		cli.Events.Delete.EventID, cli.Events.Delete.Force, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events search <query>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	if err := runEventsSearch(context.Background(), conn, cli.Events.Search.CalendarIDs,
		cli.Events.Search.Query, cli.Events.Search.TimeMin, cli.Events.Search.TimeMax,
		cli.Events.Search.MaxResults, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events updated", "events updated <calendar-id>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	if err := runEventsUpdated(context.Background(), conn, cli.Events.Updated.CalendarID,
		cli.Events.Updated.UpdatedMin, cli.Events.Updated.TimeMin, cli.Events.Updated.TimeMax,
		cli.Events.Updated.MaxResults, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events conflicts", "events conflicts <calendar-id>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	if err := runEventsConflicts(context.Background(), conn, cli.Events.Conflicts.CalendarID,
		cli.Events.Conflicts.TimeMin, cli.Events.Conflicts.TimeMax, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}

case "events import", "events import <calendar-id>":
	conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
	if err != nil {
		out.writeError(err)
		os.Exit(3)
	}
	var reader io.Reader
	if cli.Events.Import.File == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(cli.Events.Import.File)
		if err != nil {
			out.writeError(fmt.Errorf("failed to open file: %w", err))
			os.Exit(2)
		}
		defer f.Close()
		reader = f
	}
	if err := runEventsImport(context.Background(), conn, cli.Events.Import.CalendarID,
		reader, cli.Events.Import.DryRun, out); err != nil {
		out.writeError(err)
		os.Exit(2)
	}
```

**Step 4: Build and verify help output**

Run: `go build -o gwcli . && ./gwcli --help`
Expected: Shows `calendars` and `events` commands

Run: `./gwcli calendars --help`
Run: `./gwcli events --help`
Expected: Shows all subcommands with proper help text

**Step 5: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add main.go
git commit -m "$(cat <<'EOF'
feat: wire calendar and events commands into Kong CLI

Add command routing for all calendar and events subcommands:
- calendars: list
- events: list, read, create, quickadd, update, delete, search, updated, conflicts, import

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 14: Update Documentation

**Files:**
- Modify: `/home/wes/Devel/gwcli/CLAUDE.md`

**Step 1: Add Calendar section to CLAUDE.md**

Add after the Tasks documentation section:

```markdown
## Google Calendar Commands

gwcli supports Google Calendar API for managing calendars and events.

### Calendars

```bash
# List all accessible calendars
gwcli calendars list
gwcli calendars list --json
gwcli calendars list --min-access-role owner
```

### Events

```bash
# List upcoming events (default: primary calendar)
gwcli events list
gwcli events list work@group.calendar.google.com

# List events in date range
gwcli events list --time-min "2024-01-01T00:00:00Z" --time-max "2024-01-31T23:59:59Z"

# Search events
gwcli events list --query "meeting"

# Get event details
gwcli events read <event-id>
gwcli events read <calendar-id> <event-id>
gwcli events read <event-id> --json

# Create event with full details
gwcli events create --summary "Team Meeting" --start "2024-01-15T10:00:00Z" --end "2024-01-15T11:00:00Z"
gwcli events create --summary "All Day Event" --start "2024-01-15" --all-day
gwcli events create --summary "Meeting" --start "2024-01-15T10:00:00Z" --attendee alice@example.com --attendee bob@example.com

# Quick add (natural language)
gwcli events quickadd "Lunch with Bob tomorrow at noon"
gwcli events quickadd <calendar-id> "Team standup every Monday at 9am"

# Update event
gwcli events update <event-id> --summary "New Title"
gwcli events update <calendar-id> <event-id> --location "Conference Room B"

# Delete event
gwcli events delete <event-id>
gwcli events delete <event-id> --force

# Search across calendars
gwcli events search "review"
gwcli events search "review" --calendar primary --calendar work@group.calendar.google.com

# Find recently updated events
gwcli events updated --updated-min "2024-01-01T00:00:00Z"

# Detect scheduling conflicts
gwcli events conflicts
gwcli events conflicts --time-max "2024-02-01T00:00:00Z"

# Import ICS file
gwcli events import --file meeting.ics
gwcli events import --file meeting.ics --dry-run
cat events.ics | gwcli events import --file -
```

### Reminders

Reminder format: `<number>[w|d|h|m] [popup|email|sms]`

Examples:
- `15` or `15m` - 15 minutes before, popup notification
- `1h` - 1 hour before, popup
- `2d popup` - 2 days before, popup
- `1w email` - 1 week before, email

```bash
gwcli events create --summary "Meeting" --start "2024-01-15T10:00:00Z" \
    --reminder "15m popup" --reminder "1h email"
```

### Service Account Usage

For Google Workspace accounts using service accounts:

```bash
# List calendars for a specific user
gwcli --user user@example.com calendars list

# Create event for a user
gwcli --user user@example.com events create --summary "Meeting" --start "2024-01-15T10:00:00Z"
```

Note: Service accounts require domain-wide delegation with the `https://www.googleapis.com/auth/calendar` scope authorized in Google Workspace Admin Console.
```

**Step 2: Update OAuth scopes documentation**

Find the "Required OAuth scopes" section and add:

```markdown
Required OAuth scopes:
- `https://www.googleapis.com/auth/gmail.modify`
- `https://www.googleapis.com/auth/gmail.settings.basic`
- `https://www.googleapis.com/auth/gmail.labels`
- `https://www.googleapis.com/auth/tasks`
- `https://www.googleapis.com/auth/calendar`
```

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: add Google Calendar documentation

Document calendar and events commands including:
- Calendar list operations
- Event CRUD operations (create, read, update, delete)
- Quick add with natural language
- Search and query operations
- Conflict detection
- ICS import
- Reminder format specification
- Service account usage

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Phase 6: Final Verification (Task 15)

### Task 15: Run Full Test Suite and Final Verification

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 2: Build the binary**

Run: `go build -o gwcli .`
Expected: Builds successfully with no errors

**Step 3: Verify help output**

Run: `./gwcli --help`
Expected: Shows all commands including calendars and events

Run: `./gwcli calendars --help`
Run: `./gwcli events --help`
Expected: Shows all subcommands with proper help text

**Step 4: Run linter**

Run: `go vet ./...`
Expected: No warnings

**Step 5: Test with real credentials (manual)**

If you have OAuth credentials configured:
```bash
# Calendar operations
./gwcli calendars list
./gwcli events list
./gwcli events quickadd "Test event tomorrow at 3pm"
./gwcli events list --query "Test"
./gwcli events conflicts
# Clean up test event
./gwcli events delete <event-id> --force
```

**Step 6: Final commit (if any fixes needed)**

```bash
git status
# If changes needed, commit them
```

---

## Summary

This plan implements Google Calendar integration with:

1. **Authentication**: Adds `calendar.CalendarScope` to OAuth2 and service account configurations
2. **Service Layer**: Adds `*calendar.Service` to CmdG struct with proper initialization
3. **Calendar Commands**: list (with access role filtering)
4. **Event Commands**: list, read, create, quickadd, update, delete
5. **Query Commands**: search (multi-calendar), updated, conflicts
6. **Import**: ICS/iCalendar parsing and graceful import
7. **Utilities**: Reminder parsing, date formatting, conflict detection
8. **Testing**: Mock HTTP transport tests for all commands
9. **Documentation**: Updated CLAUDE.md with comprehensive usage examples

**Features NOT Implemented (per requirements exclusions):**
- Week/month calendar views (return structured data, not formatted output)
- ICS export (optional in spec, can be added later)
- Calendar caching (events change frequently, not practical)

**Total Files:**
- Modified: 4 (gmailctl/auth.go, connection.go, main.go, CLAUDE.md)
- Created: 6 (calendars.go, calendars_test.go, events.go, events_test.go, pkg/gwcli/gcal/ics.go, pkg/gwcli/gcal/ics_test.go)
- Tests: 1 existing modified (connection_test.go, auth_test.go)

**Estimated Commits:** 15
