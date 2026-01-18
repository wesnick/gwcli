# Go Google Calendar Library Requirements

Requirements for porting gcalcli calendar functionality to a Go library for Google Workspace integration.

**Source:** gcalcli (Python CLI tool)
**Target:** Go library (no CLI, no auth handling)
**API Version:** Google Calendar API v3
**OAuth Scope:** `https://www.googleapis.com/auth/calendar`

---

## 1. Core Event Operations

### 1.1 Create Event (Detailed)

Create events with full specification:

- **Title** (summary)
- **Start/End** datetime (with timezone support)
- **Location** (string)
- **Description** (string, supports multi-line)
- **Attendees** (list of email addresses)
- **Reminders** (list of reminder specs - see Section 5)
- **Color** (Google Calendar color ID override)

### 1.2 Quick Add Event

Create events using natural language parsing (delegates to Google's API):

- Input: Free-form text like "Lunch with Bob tomorrow at noon"
- Optional: Custom reminders to attach
- Returns: Created event

### 1.3 Get/Query Events

- Fetch single event by ID
- Query events by date range
- Full-text search across selected calendars
- Automatic pagination handling for large result sets

### 1.4 Update Event (Patch)

Modify existing event fields selectively via PATCH semantics.

### 1.5 Delete Event

Remove event by ID from specified calendar.

---

## 2. Calendar Management

### 2.1 List Calendars

Retrieve all calendars accessible to the authenticated user with metadata:

- **Calendar ID** (unique identifier for API calls)
- **Summary** (display name)
- **Access Role** (owner, writer, reader, freeBusyReader)
- **Timezone** (IANA timezone string)
- **Color** (optional color specification)

### 2.2 Calendar Selection

Support selecting a subset of calendars for operations:

- **By exact name** - Match calendar summary exactly
- **By regex pattern** - Match calendar names against regular expression
- **By ID** - Direct calendar ID specification

### 2.3 Calendar Caching

Cache calendar list to reduce API calls:

- Store calendar list locally (suggest using Go's `encoding/gob` or JSON)
- **Refresh** - Force rebuild of cache
- **No-cache mode** - Bypass cache entirely for fresh data
- Cache location configurable

### 2.4 Access Role Filtering

Optionally filter calendars by access level (e.g., only show owned calendars, exclude freeBusyReader).

---

## 3. Queries & Views

### 3.1 Agenda Query

Retrieve events within a date range across selected calendars:

- **Start/End** - Datetime bounds for query
- **Options:**
  - Exclude events that have already started
  - Exclude declined events
  - Filter cancelled events (automatic)
- Returns events sorted chronologically

### 3.2 Text Search Query

Full-text search for events:

- Search term matched against event title, description, location, attendees
- Scoped to date range (optional)
- Searches across all selected calendars

### 3.3 Updates Query

Find events modified since a specific datetime:

- Input: Last-updated timestamp, optional date range bounds
- Returns: Events with `updated` field >= timestamp
- Useful for sync/polling scenarios

### 3.4 Conflicts Query

Detect scheduling conflicts (overlapping events):

- Scoped to date range (default: 30 days lookahead)
- Optional text filter to narrow scope
- Returns pairs/groups of overlapping events
- Algorithm: Track active events chronologically, flag overlaps

### 3.5 Calendar Views (Week/Month)

Generate structured calendar view data:

**Week View:**

- Configurable number of weeks
- Option to exclude weekends (5-day view)
- Week start configurable (Sunday or Monday)

**Month View:**

- Display events in month grid structure
- Configurable number of months

**View Output:**

- Return structured data (not formatted strings) for integration flexibility
- Include: date cells, events per cell, current-time marker position

---

## 4. Import/Export

### 4.1 ICS/iCalendar Import

Parse and import events from ICS/vCalendar format.

**Parsing Requirements:**

- Parse VCALENDAR containers with VEVENT components
- Extract fields:
  - Summary (title)
  - Location
  - DTSTART / DTEND (with timezone handling)
  - DURATION (alternative to DTEND)
  - Description
  - RRULE (recurrence rules)
  - Organizer and attendees
  - iCalendar UID and sequence number
  - Attachments

**Import Methods:**

1. **Graceful Import (preferred):**
   - Use `events().import_()` API endpoint
   - Requires iCalUID present in event
   - Avoids duplicates on re-import (API returns 409 for existing)
   - Preserves external event identity

2. **Legacy Insert (fallback):**
   - Use `events().insert()` for events incompatible with import API
   - May create duplicates on re-import

**Import Options:**

- **Dry-run mode** - Parse and validate without importing
- **Custom reminders** - Attach reminders to imported events
- **Verbose mode** - Return detailed import results per event
- **Error collection** - Gather failed events for retry/export

### 4.2 JSON Output

Serialize events and query results to JSON:

- Full event data as returned by API
- Include computed fields (parsed start/end datetimes, duration)
- Configurable detail level (see Section 6)

### 4.3 ICS Export (Optional)

Generate ICS format from events for external sharing:

- Single event export
- Batch export from query results
- Preserve recurrence rules

---

## 5. Reminders

### 5.1 Reminder Specification Format

Parse human-readable reminder strings into API format.

**Time Units:**

| Unit | Meaning |
|------|---------|
| `m` | minutes (default if no unit specified) |
| `h` | hours |
| `d` | days |
| `w` | weeks |

**Methods:**

| Method | Description |
|--------|-------------|
| `popup` | Desktop/mobile notification (default) |
| `email` | Email reminder |
| `sms` | SMS notification (if supported by account) |

**Format:** `<number>[w|d|h|m] [method]`

**Examples:**

- `"15"` - 15 minutes, popup
- `"15m email"` - 15 minutes, email
- `"1h"` - 1 hour, popup
- `"2d popup"` - 2 days, popup
- `"1w email"` - 1 week, email

### 5.2 Reminder Operations

**Add Reminders to Event:**

- Attach list of parsed reminders to new or existing events
- Option to use calendar's default reminders instead of explicit list
- Option for no reminders (override defaults)

**Reminder Data Structure:**

```go
type Reminder struct {
    Method  string // "popup", "email", "sms"
    Minutes int    // minutes before event
}
```

### 5.3 Reminder Extraction

Extract and format reminders from existing events:

- Parse `event.reminders.overrides` array
- Detect if using default reminders (`event.reminders.useDefault`)
- Convert minutes back to human-readable format for display

---

## 6. Event Details Extraction

### 6.1 Core Event Fields

Extract and normalize standard event data:

| Field | Source | Description |
|-------|--------|-------------|
| **ID** | `event.id` | Unique event identifier |
| **Title** | `event.summary` | Event title (fallback: "(No title)") |
| **Start** | `event.start.dateTime` or `event.start.date` | Start time (parsed to native datetime) |
| **End** | `event.end.dateTime` or `event.end.date` | End time (parsed to native datetime) |
| **Location** | `event.location` | Location string |
| **Description** | `event.description` | Full event description |
| **Status** | `event.status` | confirmed, tentative, cancelled |
| **Created** | `event.created` | Creation timestamp |
| **Updated** | `event.updated` | Last modification timestamp |

### 6.2 Computed Fields

Derive additional useful data:

- **Duration** - Calculate from end - start
- **All-day flag** - Detect when using `date` (not `dateTime`) fields
- **Is recurring** - Check for `recurringEventId` presence
- **Has started** - Compare start to current time
- **Has ended** - Compare end to current time

### 6.3 People & Participation

| Field | Source | Description |
|-------|--------|-------------|
| **Organizer** | `event.organizer` | Email and display name |
| **Creator** | `event.creator` | Who created the event |
| **Attendees** | `event.attendees[]` | List with email, name, response status |
| **Self response** | `attendees[].self` + `responseStatus` | Current user's RSVP (accepted, declined, tentative, needsAction) |

### 6.4 Conference Data

Extract video conferencing details from `event.conferenceData`:

- **Conference solution** - Google Meet, Hangouts, Zoom, etc.
- **Entry points** - Array of join methods:
  - Video URI
  - Phone numbers with PIN
  - SIP URI
  - Platform-specific options
- **Conference ID** - Meeting identifier

### 6.5 Additional Metadata

| Field | Source | Description |
|-------|--------|-------------|
| **HTML Link** | `event.htmlLink` | Direct link to event in Google Calendar |
| **Color** | `event.colorId` | Event color override ID |
| **Visibility** | `event.visibility` | default, public, private |
| **Attachments** | `event.attachments[]` | File attachments with title, URL, mime type |
| **Recurrence** | `event.recurrence[]` | RRULE strings for recurring events |
| **Calendar** | (from query context) | Parent calendar ID and name |

### 6.6 Configurable Detail Levels

Support presets for common use cases:

| Preset | Fields Included |
|--------|-----------------|
| **Minimal** | ID, title, start, end |
| **Standard** | Above + location, description, calendar |
| **Full** | All available fields |
| **Custom** | Caller specifies which fields to include |

---

## 7. Utilities & Error Handling

### 7.1 Date/Time Parsing

Flexible parsing for user-friendly input.

**Absolute Formats:**

- ISO 8601: `2024-12-31`, `2024-12-31T14:30:00`
- Common formats: `Dec 31, 2024`, `31/12/2024`, `12/31/2024`
- Date only: `Jan 4th`, `2nd Jan`

**Relative/Natural Language:**

- `today`, `tomorrow`, `yesterday`
- `next Monday`, `last Friday`
- `in 2 hours`, `3 days ago`
- Combined: `tomorrow 10am`, `next Tuesday at 3pm`

**Suggested Go Libraries:**

- `araddon/dateparse`
- `olebedev/when`
- `tj/go-naturaldate`

### 7.2 Duration Parsing

Parse human-readable durations:

| Format | Example | Result |
|--------|---------|--------|
| Minutes only | `30` | 30 minutes |
| Hours:Minutes | `1:30` | 1 hour 30 minutes |
| Named units | `1d 2h 30m` | 1 day, 2 hours, 30 minutes |
| Natural | `2 hours` | 2 hours |

### 7.3 Timezone Handling

- Default to system local timezone
- Support explicit timezone specification (IANA names)
- Convert API responses to local timezone
- Preserve original timezone in all-day events (date-only)

### 7.4 API Rate Limiting & Retry

Implement exponential backoff for API resilience.

**Trigger Conditions:**

- HTTP 403 with `rateLimitExceeded`
- HTTP 403 with `userRateLimitExceeded`
- HTTP 429 Too Many Requests

**Backoff Strategy:**

- Initial delay: 1 second
- Exponential multiplier: 2x
- Max retries: 5
- Add random jitter to prevent thundering herd

### 7.5 Error Types

Define structured errors for caller handling:

| Error Type | Description |
|------------|-------------|
| `ErrNotFound` | Event or calendar does not exist |
| `ErrPermissionDenied` | Insufficient access rights |
| `ErrRateLimited` | API quota exceeded (after retries exhausted) |
| `ErrInvalidInput` | Malformed input (bad date, invalid calendar ID) |
| `ErrConflict` | Duplicate event on import (409 response) |
| `ErrAPIError` | Generic API error with status code and message |
| `ErrParsing` | Failed to parse ICS or date/time input |

### 7.6 Pagination Handling

Transparently handle paginated API responses:

- Automatically follow `nextPageToken` for event lists
- Configurable max results per request (default: 250)
- Optional callback/channel for streaming results
- Support early termination

### 7.7 Year 2038 Consideration

Filter or warn on events with dates >= 2038 if using 32-bit timestamps internally. Less relevant in Go with 64-bit `time.Time`, but document for awareness.

---

## Appendix A: Google Calendar API Reference

**Key API Methods:**

| Operation | API Method |
|-----------|------------|
| List calendars | `calendarList.list()` |
| List events | `events.list()` |
| Get event | `events.get()` |
| Insert event | `events.insert()` |
| Import event | `events.import()` |
| Update event | `events.patch()` |
| Delete event | `events.delete()` |
| Quick add | `events.quickAdd()` |

**Go Client Library:** `google.golang.org/api/calendar/v3`

---

## Appendix B: Excluded Features

The following gcalcli features are explicitly excluded from this port:

- **Command-line interface** - Library only, no CLI
- **Authentication/OAuth flow** - Caller provides authenticated client
- **TSV output format** - Not needed for programmatic use
- **TSV batch operations (agendaupdate)** - Use direct API calls instead
- **Terminal formatting** - Colors, ASCII art calendars (return structured data instead)
- **Remind daemon mode** - Application-level concern
- **Configuration file handling** - Caller manages configuration
