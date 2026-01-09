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
		"20060102T150405Z", // UTC
		"20060102T150405",  // Local time
		"20060102",         // Date only
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
