package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/calendar/v3"
)

// createEventOptions holds the options for creating an event.
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

// eventOutput represents an event for JSON output.
type eventOutput struct {
	ID            string           `json:"id"`
	CalendarID    string           `json:"calendarId,omitempty"`
	Summary       string           `json:"summary"`
	Description   string           `json:"description,omitempty"`
	Location      string           `json:"location,omitempty"`
	Start         string           `json:"start,omitempty"`
	End           string           `json:"end,omitempty"`
	StartDate     string           `json:"startDate,omitempty"`
	EndDate       string           `json:"endDate,omitempty"`
	AllDay        bool             `json:"allDay,omitempty"`
	Status        string           `json:"status,omitempty"`
	HTMLLink      string           `json:"htmlLink,omitempty"`
	Attendees     []attendeeOutput `json:"attendees,omitempty"`
	Organizer     *organizerOutput `json:"organizer,omitempty"`
	Reminders     *remindersOutput `json:"reminders,omitempty"`
	RecurringID   string           `json:"recurringEventId,omitempty"`
	Recurrence    []string         `json:"recurrence,omitempty"`
	ConferenceURL string           `json:"conferenceUrl,omitempty"`
	Created       string           `json:"created,omitempty"`
	Updated       string           `json:"updated,omitempty"`
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
	UseDefault bool             `json:"useDefault"`
	Overrides  []reminderOutput `json:"overrides,omitempty"`
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

// runEventsCreate creates a new calendar event.
func runEventsCreate(ctx context.Context, conn *gwcli.CmdG, calendarID string, opts createEventOptions, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}

	if opts.summary == "" {
		return fmt.Errorf("summary is required")
	}
	if opts.start == "" {
		return fmt.Errorf("start time is required")
	}

	out.writeVerbose("Creating event in calendar %s...", calendarID)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	event := &calendar.Event{
		Summary:     opts.summary,
		Description: opts.description,
		Location:    opts.location,
	}

	// Handle start and end times
	if opts.allDay {
		event.Start = &calendar.EventDateTime{Date: opts.start}
		if opts.end == "" {
			// For all-day events, default end to the next day
			startDate, err := time.Parse("2006-01-02", opts.start)
			if err == nil {
				event.End = &calendar.EventDateTime{Date: startDate.AddDate(0, 0, 1).Format("2006-01-02")}
			} else {
				event.End = &calendar.EventDateTime{Date: opts.start}
			}
		} else {
			event.End = &calendar.EventDateTime{Date: opts.end}
		}
	} else {
		event.Start = &calendar.EventDateTime{DateTime: opts.start}
		if opts.end == "" {
			// Default to 1 hour duration
			startTime, err := time.Parse(time.RFC3339, opts.start)
			if err == nil {
				event.End = &calendar.EventDateTime{DateTime: startTime.Add(time.Hour).Format(time.RFC3339)}
			} else {
				event.End = &calendar.EventDateTime{DateTime: opts.start}
			}
		} else {
			event.End = &calendar.EventDateTime{DateTime: opts.end}
		}
	}

	// Handle attendees
	if len(opts.attendees) > 0 {
		attendees := make([]*calendar.EventAttendee, len(opts.attendees))
		for i, email := range opts.attendees {
			attendees[i] = &calendar.EventAttendee{Email: email}
		}
		event.Attendees = attendees
	}

	// Handle reminders
	if len(opts.reminders) > 0 {
		overrides, err := parseReminders(opts.reminders)
		if err != nil {
			return fmt.Errorf("failed to parse reminders: %w", err)
		}
		event.Reminders = &calendar.EventReminders{
			UseDefault: false,
			Overrides:  overrides,
		}
	}

	// Handle color
	if opts.colorID != "" {
		event.ColorId = opts.colorID
	}

	createdEvent, err := svc.Events.Insert(calendarID, event).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	if out.json {
		return out.writeJSON(eventOutputFromEvent(createdEvent, calendarID))
	}

	// Text output
	out.writeMessage(fmt.Sprintf("Created event: %s", createdEvent.Summary))
	date, timeStr := formatEventTime(createdEvent)
	if timeStr == "all-day" {
		out.writeMessage(fmt.Sprintf("Date: %s (all day)", date))
	} else {
		out.writeMessage(fmt.Sprintf("When: %s %s", date, timeStr))
	}
	if createdEvent.HtmlLink != "" {
		out.writeMessage(fmt.Sprintf("Link: %s", createdEvent.HtmlLink))
	}
	out.writeMessage(fmt.Sprintf("ID: %s", createdEvent.Id))

	return nil
}

// runEventsQuickAdd creates an event using natural language text.
func runEventsQuickAdd(ctx context.Context, conn *gwcli.CmdG, calendarID, text string, out *outputWriter) error {
	if calendarID == "" {
		calendarID = "primary"
	}

	if text == "" {
		return fmt.Errorf("text is required")
	}

	out.writeVerbose("Quick adding event to calendar %s: %q", calendarID, text)

	svc := conn.CalendarService()
	if svc == nil {
		return fmt.Errorf("calendar service not initialized")
	}

	createdEvent, err := svc.Events.QuickAdd(calendarID, text).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to quick add event: %w", err)
	}

	if out.json {
		return out.writeJSON(eventOutputFromEvent(createdEvent, calendarID))
	}

	// Text output
	out.writeMessage(fmt.Sprintf("Created event: %s", createdEvent.Summary))
	date, timeStr := formatEventTime(createdEvent)
	if timeStr == "all-day" {
		out.writeMessage(fmt.Sprintf("Date: %s (all day)", date))
	} else {
		out.writeMessage(fmt.Sprintf("When: %s %s", date, timeStr))
	}
	if createdEvent.HtmlLink != "" {
		out.writeMessage(fmt.Sprintf("Link: %s", createdEvent.HtmlLink))
	}
	out.writeMessage(fmt.Sprintf("ID: %s", createdEvent.Id))

	return nil
}

// parseReminders parses a list of reminder specifications.
func parseReminders(specs []string) ([]*calendar.EventReminder, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	reminders := make([]*calendar.EventReminder, len(specs))
	for i, spec := range specs {
		minutes, method, err := parseReminderSpec(spec)
		if err != nil {
			return nil, fmt.Errorf("invalid reminder %q: %w", spec, err)
		}
		reminders[i] = &calendar.EventReminder{
			Method:  method,
			Minutes: minutes,
		}
	}
	return reminders, nil
}

// parseReminderSpec parses a single reminder specification like "15m popup" or "1h email".
func parseReminderSpec(spec string) (minutes int64, method string, err error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, "", fmt.Errorf("empty reminder specification")
	}

	parts := strings.Fields(spec)
	if len(parts) == 0 {
		return 0, "", fmt.Errorf("empty reminder specification")
	}

	// Parse duration
	minutes, err = parseDurationToMinutes(parts[0])
	if err != nil {
		return 0, "", err
	}

	// Parse method (default to popup)
	method = "popup"
	if len(parts) > 1 {
		m := strings.ToLower(parts[1])
		if m == "email" || m == "popup" {
			method = m
		} else {
			return 0, "", fmt.Errorf("invalid reminder method %q (must be 'email' or 'popup')", parts[1])
		}
	}

	return minutes, method, nil
}

// parseDurationToMinutes parses duration strings like "15", "15m", "1h", "2d", "1w" to minutes.
func parseDurationToMinutes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	// Check for suffix
	last := s[len(s)-1]
	var multiplier int64 = 1
	var numStr string

	switch last {
	case 'm':
		numStr = s[:len(s)-1]
		multiplier = 1
	case 'h':
		numStr = s[:len(s)-1]
		multiplier = 60
	case 'd':
		numStr = s[:len(s)-1]
		multiplier = 60 * 24
	case 'w':
		numStr = s[:len(s)-1]
		multiplier = 60 * 24 * 7
	default:
		// If no suffix, assume minutes
		if last >= '0' && last <= '9' {
			numStr = s
			multiplier = 1
		} else {
			return 0, fmt.Errorf("invalid duration suffix %q", string(last))
		}
	}

	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number %q: %w", numStr, err)
	}

	return num * multiplier, nil
}
