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

func TestParseICSAllDayEvent(t *testing.T) {
	icsData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:allday@example.com
DTSTART;VALUE=DATE:20240115
DTEND;VALUE=DATE:20240116
SUMMARY:All Day Event
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
	if ev.Event.Start.Date != "2024-01-15" {
		t.Errorf("expected start date '2024-01-15', got %q", ev.Event.Start.Date)
	}
	if ev.Event.End.Date != "2024-01-16" {
		t.Errorf("expected end date '2024-01-16', got %q", ev.Event.End.Date)
	}
}

func TestParseICSWithAttendees(t *testing.T) {
	icsData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:meeting@example.com
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Team Meeting
ORGANIZER;CN=Alice:mailto:alice@example.com
ATTENDEE;CN=Bob;PARTSTAT=ACCEPTED:mailto:bob@example.com
ATTENDEE;CN=Carol;PARTSTAT=TENTATIVE:mailto:carol@example.com
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
	if ev.Event.Organizer == nil {
		t.Fatal("expected organizer to be set")
	}
	if ev.Event.Organizer.Email != "alice@example.com" {
		t.Errorf("expected organizer email 'alice@example.com', got %q", ev.Event.Organizer.Email)
	}

	if len(ev.Event.Attendees) != 2 {
		t.Fatalf("expected 2 attendees, got %d", len(ev.Event.Attendees))
	}
	if ev.Event.Attendees[0].Email != "bob@example.com" {
		t.Errorf("expected first attendee email 'bob@example.com', got %q", ev.Event.Attendees[0].Email)
	}
	if ev.Event.Attendees[0].ResponseStatus != "accepted" {
		t.Errorf("expected first attendee status 'accepted', got %q", ev.Event.Attendees[0].ResponseStatus)
	}
}

func TestParseICSMultipleEvents(t *testing.T) {
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

	events, err := ParseICS(strings.NewReader(icsData))
	if err != nil {
		t.Fatalf("ParseICS() error = %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Summary != "Event 1" {
		t.Errorf("expected first event summary 'Event 1', got %q", events[0].Summary)
	}
	if events[1].Summary != "Event 2" {
		t.Errorf("expected second event summary 'Event 2', got %q", events[1].Summary)
	}
}

func TestParseICSWithDuration(t *testing.T) {
	icsData := `BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:duration@example.com
DTSTART:20240115T100000Z
DURATION:PT1H30M
SUMMARY:Duration Event
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
	if ev.Event.End == nil || ev.Event.End.DateTime == "" {
		t.Fatal("expected end time to be calculated from duration")
	}
	// Start is 10:00, duration is 1h30m, end should be 11:30
	if !strings.Contains(ev.Event.End.DateTime, "11:30") {
		t.Errorf("expected end time to contain '11:30', got %q", ev.Event.End.DateTime)
	}
}
