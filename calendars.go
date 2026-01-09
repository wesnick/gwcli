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
