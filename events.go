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
