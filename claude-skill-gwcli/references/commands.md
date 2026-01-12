# gwcli Command Reference

Complete reference for all gwcli commands, flags, and usage patterns.

## Command Structure

gwcli follows a resource-oriented command pattern:

```
gwcli <resource> <action> [arguments] [flags]
```

## Resources

- **messages** - Email message operations
- **labels** - Gmail label management
- **attachments** - Attachment operations
- **gmailctl** - Filter and label management via gmailctl integration
- **tasklists** - Google Task list operations
- **tasks** - Google Task operations
- **calendars** - Google Calendar listing
- **events** - Google Calendar event operations

## Global Flags

Available on all commands:

- `--config <path>` - Config directory path (default: ~/.config/gwcli)
- `--user <email>` - User email for service account impersonation
- `--json` - Output results in JSON format
- `--verbose` - Enable verbose logging
- `--no-color` - Disable colored output

## Messages Commands

### gwcli messages list

List messages from inbox or a specific label.

**Syntax:**
```bash
gwcli messages list [flags]
```

**Flags:**
- `--label <name>` - List messages with specific label (default: INBOX)
- `--unread-only` - Only show unread messages
- `--limit <n>` - Maximum number of messages to retrieve (default: 50)
- `--json` - Output as JSON array
- `--no-color` - Disable colored output

**Output Fields (JSON):**
- `id` - Message ID
- `threadId` - Thread ID
- `snippet` - Message preview text
- `from` - Sender email/name
- `subject` - Email subject
- `date` - Date sent
- `labels` - Array of label IDs

**Examples:**
```bash
# List inbox
gwcli messages list

# List work emails in JSON
gwcli messages list --label "Work" --json

# List unread messages only
gwcli messages list --unread-only

# List from custom label
gwcli messages list --label "Projects/2025"
```

### gwcli messages read

Read a single message by ID.

**Syntax:**
```bash
gwcli messages read <message-id> [flags]
```

**Flags:**
- `--raw` - Output raw RFC822 format
- `--headers-only` - Show only headers
- `--raw-html` - Output raw HTML with HTML-formatted metadata
- `--prefer-plain` - Prefer plain text body over HTML
- `--json` - Output as JSON

**Output Formats:**

| Flag | Output Format |
|------|---------------|
| (default) | Markdown with YAML frontmatter (HTML auto-converted to markdown) |
| `--raw-html` | Raw HTML with HTML-formatted headers |
| `--prefer-plain` | Plain text with YAML frontmatter |
| `--raw` | Raw RFC822 format |

**Examples:**
```bash
# Read message (default markdown output)
gwcli messages read 18a1b2c3d4e5f678

# Read with raw HTML output
gwcli messages read 18a1b2c3d4e5f678 --raw-html

# Read plain text only
gwcli messages read 18a1b2c3d4e5f678 --prefer-plain

# Get raw RFC822 message
gwcli messages read 18a1b2c3d4e5f678 --raw

# Get as JSON (includes all body formats)
gwcli messages read 18a1b2c3d4e5f678 --json
```

### gwcli messages search

Search messages using Gmail query syntax.

**Syntax:**
```bash
gwcli messages search "<query>" [flags]
```

**Flags:**
- `--limit <n>` - Maximum number of results (default: 100)
- `--json` - Output as JSON array

**Query Syntax:**
Uses standard Gmail search operators:
- `from:user@example.com` - From specific sender
- `to:user@example.com` - To specific recipient
- `subject:keyword` - Subject contains keyword
- `has:attachment` - Has attachments
- `is:unread` - Unread messages
- `is:starred` - Starred messages
- `after:2025/01/01` - After date
- `before:2025/12/31` - Before date
- `older_than:30d` - Older than 30 days
- `newer_than:7d` - Newer than 7 days
- `category:promotions` - In category
- `label:Important` - Has label

**Examples:**
```bash
# Search by sender
gwcli messages search "from:boss@company.com"

# Search unread with attachment
gwcli messages search "is:unread has:attachment"

# Search by date range
gwcli messages search "after:2025/10/01 before:2025/11/01"

# Complex query
gwcli messages search "from:notifications@github.com subject:merged is:unread"

# Get results as JSON for piping
gwcli messages search "label:Invoices" --json
```

### gwcli messages send

Send an email message.

**Syntax:**
```bash
gwcli messages send [flags]
```

**Flags:**
- `--to <email>` - Recipient (required, can be repeated)
- `--cc <email>` - CC recipient (can be repeated)
- `--bcc <email>` - BCC recipient (can be repeated)
- `--subject <text>` - Email subject (required)
- `--body <text>` - Email body (if omitted, reads from stdin)
- `--attach <file>` - Attach file (can be repeated)
- `--html` - Send body as HTML
- `--thread-id <id>` - Reply to thread
- `--json` - Output result as JSON

**Body Input:**
- Use `--body` flag to specify inline
- Omit `--body` to read from stdin
- Use `--html` for HTML formatted bodies

**Examples:**
```bash
# Simple text email
gwcli messages send \
  --to user@example.com \
  --subject "Hello" \
  --body "This is the message"

# With attachments
gwcli messages send \
  --to user@example.com \
  --subject "Documents" \
  --attach report.pdf \
  --attach invoice.xlsx \
  --body "Please review the attached documents"

# Multiple recipients
gwcli messages send \
  --to user1@example.com \
  --to user2@example.com \
  --cc manager@example.com \
  --subject "Team Update" \
  --body "Here is the update"

# Body from stdin
echo "Message content" | gwcli messages send \
  --to user@example.com \
  --subject "Test"

# HTML email
gwcli messages send \
  --to user@example.com \
  --subject "HTML Test" \
  --body "<h1>Hello</h1><p>This is HTML</p>" \
  --html
```

### gwcli messages delete

Delete messages (move to trash).

**Syntax:**
```bash
gwcli messages delete <message-id> [flags]
# OR
gwcli messages delete --stdin [flags]
```

**Flags:**
- `--force` - Required to confirm deletion
- `--stdin` - Read message IDs from stdin (one per line)
- `--json` - Output result as JSON

**Safety:**
- Requires `--force` flag to prevent accidental deletion
- Batch operations via `--stdin` for automation

**Examples:**
```bash
# Delete single message
gwcli messages delete 18a1b2c3d4e5f678 --force

# Batch delete from search
gwcli messages search "from:spam@example.com" --json | \
  jq -r '.[].id' | \
  gwcli messages delete --stdin --force

# Delete old promotions
gwcli messages search "category:promotions older_than:60d" --json | \
  jq -r '.[].id' | \
  gwcli messages delete --stdin --force
```

### gwcli messages mark-read

Mark messages as read.

**Syntax:**
```bash
gwcli messages mark-read <message-id> [flags]
# OR
gwcli messages mark-read --stdin [flags]
```

**Flags:**
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Examples:**
```bash
# Mark single message
gwcli messages mark-read 18a1b2c3d4e5f678

# Mark all unread as read
gwcli messages list --unread-only --json | \
  jq -r '.[].id' | \
  gwcli messages mark-read --stdin

# Mark search results as read
gwcli messages search "label:Notifications" --json | \
  jq -r '.[].id' | \
  gwcli messages mark-read --stdin
```

### gwcli messages mark-unread

Mark messages as unread.

**Syntax:**
```bash
gwcli messages mark-unread <message-id> [flags]
# OR
gwcli messages mark-unread --stdin [flags]
```

**Flags:**
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Examples:**
```bash
# Mark single message
gwcli messages mark-unread 18a1b2c3d4e5f678

# Mark important messages as unread
gwcli messages search "from:boss@company.com" --json | \
  jq -r '.[].id' | \
  gwcli messages mark-unread --stdin
```

### gwcli messages move

Move messages to a different label (removes INBOX, adds target).

**Syntax:**
```bash
gwcli messages move <message-id> --to <label> [flags]
# OR
gwcli messages move --stdin --to <label> [flags]
```

**Flags:**
- `--to <label>` - Target label name (required)
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Behavior:**
- Removes INBOX label
- Adds target label
- Creates nested labels with `/` separator

**Examples:**
```bash
# Move to Archive
gwcli messages move 18a1b2c3d4e5f678 --to "Archive"

# Move to nested label
gwcli messages move 18a1b2c3d4e5f678 --to "Work/Projects/Q4"

# Batch move read messages
gwcli messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gwcli messages move --stdin --to "Archive"

# Move search results
gwcli messages search "label:Done older_than:90d" --json | \
  jq -r '.[].id' | \
  gwcli messages move --stdin --to "Archive"
```

## Labels Commands

### gwcli labels list

List all Gmail labels.

**Syntax:**
```bash
gwcli labels list [flags]
```

**Flags:**
- `--system` - Show only system labels (INBOX, SPAM, etc.)
- `--user-only` - Show only user-created labels
- `--json` - Output as JSON array

**Output Fields:**
- `id` - Label ID
- `name` - Label name
- `type` - SYSTEM or USER

**Examples:**
```bash
# List all labels
gwcli labels list

# List user labels only
gwcli labels list --user-only

# Get as JSON
gwcli labels list --json

# List system labels
gwcli labels list --system
```

### gwcli labels apply

Apply a label to messages.

**Syntax:**
```bash
gwcli labels apply <name> --message <id> [flags]
# OR
gwcli labels apply <name> --stdin [flags]
```

**Flags:**
- `--message <id>` - Apply to specific message
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Examples:**
```bash
# Apply to single message
gwcli labels apply "Important" --message 18a1b2c3d4e5f678

# Apply to search results
gwcli messages search "from:vip@company.com" --json | \
  jq -r '.[].id' | \
  gwcli labels apply "VIP" --stdin

# Tag invoices
gwcli messages search "subject:invoice has:attachment" --json | \
  jq -r '.[].id' | \
  gwcli labels apply "Invoices" --stdin
```

### gwcli labels remove

Remove a label from messages.

**Syntax:**
```bash
gwcli labels remove <name> --message <id> [flags]
# OR
gwcli labels remove <name> --stdin [flags]
```

**Flags:**
- `--message <id>` - Remove from specific message
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Examples:**
```bash
# Remove from single message
gwcli labels remove "Spam" --message 18a1b2c3d4e5f678

# Remove from multiple messages
gwcli messages search "label:Todo is:read" --json | \
  jq -r '.[].id' | \
  gwcli labels remove "Todo" --stdin
```

## Attachments Commands

### gwcli attachments list

List attachments in a message.

**Syntax:**
```bash
gwcli attachments list <message-id> [flags]
```

**Flags:**
- `--json` - Output as JSON array

**Output Fields:**
- `index` - Attachment index (0-based)
- `filename` - Original filename
- `mimeType` - MIME type
- `size` - Size in bytes

**Examples:**
```bash
# List attachments
gwcli attachments list 18a1b2c3d4e5f678

# Get as JSON
gwcli attachments list 18a1b2c3d4e5f678 --json
```

### gwcli attachments download

Download attachments from a message.

**Syntax:**
```bash
gwcli attachments download <message-id> [flags]
```

**Flags:**
- `--index <n>` - Download specific attachment by index (0-based, can be comma-separated or repeated)
- `-i <n>` - Short form of --index
- `--filename <pattern>` - Download attachments matching glob pattern
- `-f <pattern>` - Short form of --filename
- `--output-dir <path>` - Output directory (default: ~/Downloads)
- `--output <filename>` - Output filename (for single attachment)
- `--json` - Output result as JSON

**Behavior:**
- Without `--index` or `--filename`: Downloads all attachments
- Default download location is ~/Downloads
- Filename conflicts automatically resolved with ` (n)` suffix
- Creates output directory if it doesn't exist

**Examples:**
```bash
# Download all attachments to ~/Downloads
gwcli attachments download 18a1b2c3d4e5f678

# Download to specific directory
gwcli attachments download 18a1b2c3d4e5f678 --output-dir ./downloads

# Download specific attachment by index
gwcli attachments download 18a1b2c3d4e5f678 --index 0

# Download multiple by index
gwcli attachments download 18a1b2c3d4e5f678 --index 0,1,2
gwcli attachments download 18a1b2c3d4e5f678 -i 0 -i 1 -i 2

# Download by filename pattern
gwcli attachments download 18a1b2c3d4e5f678 --filename "*.pdf"
gwcli attachments download 18a1b2c3d4e5f678 -f "invoice*.xlsx"

# Download single attachment to specific file
gwcli attachments download 18a1b2c3d4e5f678 --index 0 --output report.pdf

# Download all from label
gwcli messages list --label "Invoices" --json | \
  jq -r '.[].id' | \
  while read id; do
    gwcli attachments download "$id" --output-dir ./invoices
  done
```

## gmailctl Commands

### gwcli gmailctl download

Download existing Gmail filters to config.jsonnet.

**Syntax:**
```bash
gwcli gmailctl download [flags]
```

**Flags:**
- `-o <file>` - Output file (default: ~/.config/gwcli/config.jsonnet)

**Examples:**
```bash
# Download to default location
gwcli gmailctl download

# Download to specific file
gwcli gmailctl download -o my-filters.jsonnet
```

### gwcli gmailctl apply

Apply config.jsonnet to Gmail.

**Syntax:**
```bash
gwcli gmailctl apply [flags]
```

**Flags:**
- `-y` - Skip confirmation prompt

**Examples:**
```bash
# Apply with confirmation
gwcli gmailctl apply

# Apply without confirmation
gwcli gmailctl apply -y
```

### gwcli gmailctl diff

Show differences between local config and Gmail.

**Syntax:**
```bash
gwcli gmailctl diff
```

**Examples:**
```bash
# Show diff
gwcli gmailctl diff
```

## Task Lists Commands

### gwcli tasklists list

List all Google Task lists.

**Syntax:**
```bash
gwcli tasklists list [flags]
```

**Flags:**
- `--json` - Output as JSON array

**Output Fields (JSON):**
- `id` - Task list ID
- `title` - Task list title
- `updated` - Last update timestamp

**Examples:**
```bash
# List all task lists
gwcli tasklists list

# Get as JSON
gwcli tasklists list --json
```

### gwcli tasklists create

Create a new task list.

**Syntax:**
```bash
gwcli tasklists create <title> [flags]
```

**Flags:**
- `--json` - Output result as JSON

**Examples:**
```bash
# Create task list
gwcli tasklists create "Work Projects"

# Create with JSON output
gwcli tasklists create "Personal Tasks" --json
```

### gwcli tasklists delete

Delete a task list.

**Syntax:**
```bash
gwcli tasklists delete <tasklist-id> [flags]
```

**Flags:**
- `--force` - Required to confirm deletion
- `--json` - Output result as JSON

**Examples:**
```bash
# Delete task list
gwcli tasklists delete abc123 --force
```

## Tasks Commands

### gwcli tasks list

List tasks in a task list.

**Syntax:**
```bash
gwcli tasks list <tasklist-id> [flags]
```

**Flags:**
- `--include-completed` - Include completed tasks
- `--json` - Output as JSON array

**Output Fields (JSON):**
- `id` - Task ID
- `title` - Task title
- `notes` - Task notes
- `status` - Task status (needsAction, completed)
- `due` - Due date (RFC3339)
- `completed` - Completion timestamp

**Examples:**
```bash
# List tasks
gwcli tasks list abc123

# Include completed tasks
gwcli tasks list abc123 --include-completed

# Get as JSON
gwcli tasks list abc123 --json
```

### gwcli tasks create

Create a new task.

**Syntax:**
```bash
gwcli tasks create <tasklist-id> [flags]
```

**Flags:**
- `--title <text>` - Task title (required)
- `--notes <text>` - Task notes
- `--due <datetime>` - Due date (RFC3339 format)
- `--json` - Output result as JSON

**Examples:**
```bash
# Create simple task
gwcli tasks create abc123 --title "Review PR"

# Create with notes and due date
gwcli tasks create abc123 \
  --title "Submit report" \
  --notes "Q4 financial summary" \
  --due "2025-01-15T00:00:00Z"
```

### gwcli tasks read

Read task details.

**Syntax:**
```bash
gwcli tasks read <tasklist-id> <task-id> [flags]
```

**Flags:**
- `--json` - Output as JSON

**Examples:**
```bash
# Read task
gwcli tasks read abc123 task456

# Get as JSON
gwcli tasks read abc123 task456 --json
```

### gwcli tasks complete

Mark a task as completed.

**Syntax:**
```bash
gwcli tasks complete <tasklist-id> <task-id> [flags]
```

**Flags:**
- `--json` - Output result as JSON

**Examples:**
```bash
# Mark task as completed
gwcli tasks complete abc123 task456
```

### gwcli tasks delete

Delete a task.

**Syntax:**
```bash
gwcli tasks delete <tasklist-id> <task-id> [flags]
```

**Flags:**
- `--force` - Required to confirm deletion
- `--json` - Output result as JSON

**Examples:**
```bash
# Delete task
gwcli tasks delete abc123 task456 --force
```

## Calendars Commands

### gwcli calendars list

List all accessible Google Calendars.

**Syntax:**
```bash
gwcli calendars list [flags]
```

**Flags:**
- `--min-access-role <role>` - Filter by minimum access role (owner, writer, reader)
- `--json` - Output as JSON array

**Output Fields (JSON):**
- `id` - Calendar ID
- `summary` - Calendar name
- `description` - Calendar description
- `accessRole` - Your access role

**Examples:**
```bash
# List all calendars
gwcli calendars list

# List only owned calendars
gwcli calendars list --min-access-role owner

# Get as JSON
gwcli calendars list --json
```

## Events Commands

### gwcli events list

List upcoming events.

**Syntax:**
```bash
gwcli events list [calendar-id] [flags]
```

**Flags:**
- `--time-min <datetime>` - Start of time range (RFC3339)
- `--time-max <datetime>` - End of time range (RFC3339)
- `--query <text>` - Full-text search
- `--json` - Output as JSON array

**Examples:**
```bash
# List upcoming events (primary calendar)
gwcli events list

# List from specific calendar
gwcli events list work@group.calendar.google.com

# List events in date range
gwcli events list --time-min "2025-01-01T00:00:00Z" --time-max "2025-01-31T23:59:59Z"

# Search events
gwcli events list --query "meeting"

# Get as JSON
gwcli events list --json
```

### gwcli events read

Read event details.

**Syntax:**
```bash
gwcli events read <event-id> [flags]
gwcli events read <calendar-id> <event-id> [flags]
```

**Flags:**
- `--json` - Output as JSON

**Examples:**
```bash
# Read event
gwcli events read event123

# Read from specific calendar
gwcli events read work@group.calendar.google.com event123

# Get as JSON
gwcli events read event123 --json
```

### gwcli events create

Create a new event.

**Syntax:**
```bash
gwcli events create [calendar-id] [flags]
```

**Flags:**
- `--summary <text>` - Event title (required)
- `--start <datetime>` - Start time (RFC3339 or YYYY-MM-DD for all-day)
- `--end <datetime>` - End time (RFC3339 or YYYY-MM-DD for all-day)
- `--all-day` - Create all-day event
- `--location <text>` - Event location
- `--description <text>` - Event description
- `--attendee <email>` - Add attendee (can be repeated)
- `--reminder <spec>` - Add reminder (format: `<number>[w|d|h|m] [popup|email]`)
- `--json` - Output result as JSON

**Reminder Format:**
- `15` or `15m` - 15 minutes before, popup
- `1h` - 1 hour before, popup
- `2d popup` - 2 days before, popup
- `1w email` - 1 week before, email

**Examples:**
```bash
# Create simple event
gwcli events create --summary "Team Meeting" \
  --start "2025-01-15T10:00:00Z" \
  --end "2025-01-15T11:00:00Z"

# Create all-day event
gwcli events create --summary "Company Holiday" \
  --start "2025-01-20" --all-day

# Create with attendees and reminders
gwcli events create --summary "Planning Session" \
  --start "2025-01-15T14:00:00Z" \
  --end "2025-01-15T15:00:00Z" \
  --location "Conference Room A" \
  --attendee alice@example.com \
  --attendee bob@example.com \
  --reminder "15m popup" \
  --reminder "1h email"
```

### gwcli events quickadd

Create event using natural language.

**Syntax:**
```bash
gwcli events quickadd <text> [flags]
gwcli events quickadd <calendar-id> <text> [flags]
```

**Flags:**
- `--json` - Output result as JSON

**Examples:**
```bash
# Quick add event
gwcli events quickadd "Lunch with Bob tomorrow at noon"

# Quick add to specific calendar
gwcli events quickadd work@group.calendar.google.com "Team standup every Monday at 9am"
```

### gwcli events update

Update an existing event.

**Syntax:**
```bash
gwcli events update <event-id> [flags]
gwcli events update <calendar-id> <event-id> [flags]
```

**Flags:**
- `--summary <text>` - New title
- `--start <datetime>` - New start time
- `--end <datetime>` - New end time
- `--location <text>` - New location
- `--description <text>` - New description
- `--json` - Output result as JSON

**Examples:**
```bash
# Update event title
gwcli events update event123 --summary "New Title"

# Update location
gwcli events update event123 --location "Room B"

# Update from specific calendar
gwcli events update work@group.calendar.google.com event123 --summary "Updated Meeting"
```

### gwcli events delete

Delete an event.

**Syntax:**
```bash
gwcli events delete <event-id> [flags]
gwcli events delete <calendar-id> <event-id> [flags]
```

**Flags:**
- `--force` - Required to confirm deletion
- `--json` - Output result as JSON

**Examples:**
```bash
# Delete event
gwcli events delete event123 --force
```

### gwcli events search

Search events across calendars.

**Syntax:**
```bash
gwcli events search <query> [flags]
```

**Flags:**
- `--calendar <id>` - Calendar to search (can be repeated)
- `--time-min <datetime>` - Start of time range
- `--time-max <datetime>` - End of time range
- `--json` - Output as JSON array

**Examples:**
```bash
# Search across all calendars
gwcli events search "review"

# Search specific calendars
gwcli events search "review" --calendar primary --calendar work@group.calendar.google.com
```

### gwcli events updated

Find recently updated events.

**Syntax:**
```bash
gwcli events updated [calendar-id] [flags]
```

**Flags:**
- `--updated-min <datetime>` - Show events updated after this time
- `--json` - Output as JSON array

**Examples:**
```bash
# Find events updated since date
gwcli events updated --updated-min "2025-01-01T00:00:00Z"
```

### gwcli events conflicts

Detect scheduling conflicts.

**Syntax:**
```bash
gwcli events conflicts [calendar-id] [flags]
```

**Flags:**
- `--time-min <datetime>` - Start of time range (default: now)
- `--time-max <datetime>` - End of time range (default: 7 days from now)
- `--json` - Output as JSON array

**Examples:**
```bash
# Check for conflicts in next 7 days
gwcli events conflicts

# Check specific time range
gwcli events conflicts --time-max "2025-02-01T00:00:00Z"
```

### gwcli events import

Import events from ICS file.

**Syntax:**
```bash
gwcli events import [calendar-id] [flags]
```

**Flags:**
- `--file <path>` - ICS file path (use `-` for stdin)
- `--dry-run` - Preview without creating events
- `--json` - Output result as JSON

**Examples:**
```bash
# Import from file
gwcli events import --file meeting.ics

# Preview import
gwcli events import --file meeting.ics --dry-run

# Import from stdin
cat events.ics | gwcli events import --file -

# Import to specific calendar
gwcli events import work@group.calendar.google.com --file meeting.ics
```

## Configuration

### Initial Setup

```bash
gwcli configure
```

This opens a browser for Google OAuth authentication and saves credentials to `~/.config/gwcli/`.

### Config Directory

Default: `~/.config/gwcli/`

Contents:
- `credentials.json` - OAuth or service account credentials
- `token.json` - OAuth access/refresh tokens (auto-generated)
- `config.jsonnet` - Label and filter definitions (gmailctl format)

Override with `--config` flag:
```bash
gwcli --config /custom/path messages list
```

### Service Account Usage

For Google Workspace accounts with domain-wide delegation:

```bash
# Specify user to impersonate
gwcli --user user@example.com messages list
gwcli --user user@example.com tasklists list
gwcli --user user@example.com events list
```

## Exit Codes

- `0` - Success
- `1` - User error (invalid arguments)
- `2` - API error
- `3` - Authentication error
- `4` - Not found

Use exit codes for error handling in scripts:

```bash
if gwcli messages read "$msg_id" 2>/dev/null; then
  echo "Success"
else
  case $? in
    3) echo "Authentication failed" ;;
    4) echo "Message not found" ;;
    *) echo "Error occurred" ;;
  esac
fi
```

## Output Formats

### Text Mode (Default)

Human-readable tables with columns:

```
ID                  SUBJECT                FROM              DATE
18a1b2c3d4e5f678   Meeting Tomorrow       alice@example.com  Jan 02
19b2c3d4e5f6789a   Project Update         bob@example.com    Jan 01
```

### JSON Mode (--json)

Machine-readable structured data for programmatic processing:

```json
[
  {
    "id": "18a1b2c3d4e5f678",
    "threadId": "18a1b2c3d4e5f678",
    "subject": "Meeting Tomorrow",
    "from": "alice@example.com",
    "date": "Jan 02",
    "snippet": "Let's meet at 2pm...",
    "labels": ["INBOX", "IMPORTANT"]
  }
]
```

## Batch Processing Patterns

### Stdin Pattern

Many commands accept `--stdin` to read IDs from standard input:

```bash
# Pattern: search | extract IDs | operate
gwcli messages search "<query>" --json | \
  jq -r '.[].id' | \
  gwcli messages <operation> --stdin [flags]
```

### Common Workflows

**Archive read messages:**
```bash
gwcli messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gwcli messages move --stdin --to "Archive"
```

**Delete spam from sender:**
```bash
gwcli messages search "from:spam@example.com" --json | \
  jq -r '.[].id' | \
  gwcli messages delete --stdin --force
```

**Apply label to unread from VIP:**
```bash
gwcli messages search "from:vip@company.com is:unread" --json | \
  jq -r '.[].id' | \
  gwcli labels apply "VIP-Unread" --stdin
```

**Download all invoices:**
```bash
gwcli messages search "subject:invoice has:attachment" --json | \
  jq -r '.[].id' | \
  while read id; do
    gwcli attachments download "$id" --output-dir ./invoices
  done
```

### Error Handling in Batches

Batch operations report errors but continue processing:

```
Processing 100 messages...
Progress: 10/100
Progress: 20/100
...
Completed: 95 successful, 5 failed
```

Failed items are logged when `--verbose` is enabled.
