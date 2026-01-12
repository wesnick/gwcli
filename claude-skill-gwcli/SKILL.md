---
name: gwcli
description: This skill should be used when working with Gmail, Google Tasks, or Google Calendar operations via the gwcli command-line tool. Use this skill when the user asks to interact with Gmail (read/send/search emails, manage labels, download attachments), manage Google Tasks (create/complete tasks), work with Google Calendar (create/list events), or needs help with gwcli commands.
---

# gwcli

## Overview

This skill provides comprehensive guidance for using gwcli, a command-line client for Gmail, Google Tasks, and Google Calendar with a resource-oriented interface (similar to kubectl). It enables automation of operations including message management, label operations, attachment handling, task management, and calendar events through a Unix-friendly CLI.

## Core Capabilities

gwcli provides these main resource types:

1. **Messages** - Read, send, search, delete, mark read/unread, and move emails
2. **Labels** - List labels, apply/remove labels to messages (creation via gmailctl)
3. **Attachments** - List and download email attachments
4. **Gmailctl** - Manage filters and labels via gmailctl integration (download, apply, diff)
5. **Task Lists** - List, create, and delete Google Task lists
6. **Tasks** - List, create, read, complete, and delete tasks
7. **Calendars** - List accessible Google Calendars
8. **Events** - List, create, read, update, delete, search, and import calendar events

## When to Use This Skill

Use this skill when the user:
- Asks to perform Gmail operations via command line
- Needs to automate email workflows (e.g., "archive all read messages", "download invoices")
- Requests batch operations on multiple emails
- Wants to search, filter, or process Gmail messages programmatically
- Needs to manage labels or organize emails
- Wants to download attachments in bulk
- Needs to manage Google Tasks (create, complete, list tasks)
- Wants to work with Google Calendar (create events, check schedule, find conflicts)

## Quick Start

### Initial Setup

Before using gwcli, configure OAuth authentication:

```bash
gwcli configure
```

This opens a browser for Google OAuth and saves credentials to `~/.config/gwcli/`.

**Required configuration files:**
- `~/.config/gwcli/credentials.json` - OAuth credentials from Google Console
- `~/.config/gwcli/token.json` - Auto-generated access token
- `~/.config/gwcli/config.jsonnet` - **Required** label definitions (gmailctl format)

**Important:** gwcli requires `config.jsonnet` to load label definitions. If missing, commands will fail with an error. See the gmailctl Integration section below for setup.

### Basic Command Pattern

gwcli follows a resource-oriented structure:

```bash
gwcli <resource> <action> [arguments] [flags]
```

**Example:**
```bash
gwcli messages list --unread-only
gwcli labels list
gwcli attachments download <message-id>
gwcli tasks list <tasklist-id>
gwcli events list
```

### gmailctl Integration

gwcli integrates with [gmailctl](https://github.com/mbrt/gmailctl) for label and filter management. Labels are defined in `~/.config/gwcli/config.jsonnet` using gmailctl's Jsonnet format.

**Setup config.jsonnet:**

1. Install gmailctl:
```bash
go install github.com/mbrt/gmailctl/cmd/gmailctl@latest
```

2. Download existing Gmail filters (creates config.jsonnet):
```bash
gwcli gmailctl download
```

3. Or create manually:
```bash
cat > ~/.config/gwcli/config.jsonnet << 'EOF'
{
  version: "v1alpha3",
  labels: [
    { name: "work" },
    { name: "personal" },
    { name: "receipts" },
  ],
  rules: []
}
EOF
```

**gwcli gmailctl wrapper commands:**

These automatically use `~/.config/gwcli` directory (no need for `--config` flag):

```bash
# Download existing filters from Gmail
gwcli gmailctl download

# Preview changes before applying
gwcli gmailctl diff

# Apply config.jsonnet to Gmail (creates filters)
gwcli gmailctl apply

# Apply without confirmation
gwcli gmailctl apply -y
```

**Manual gmailctl usage:**

For advanced commands (init, edit, test, debug), use gmailctl directly:

```bash
# Edit config interactively
gmailctl --config ~/.config/gwcli edit

# Run tests
gmailctl --config ~/.config/gwcli test

# Show annotated config
gmailctl --config ~/.config/gwcli debug
```

**Typical workflow:**

```bash
# 1. Download current Gmail setup
gwcli gmailctl download

# 2. Edit config.jsonnet to add labels/rules
vim ~/.config/gwcli/config.jsonnet

# 3. Preview changes
gwcli gmailctl diff

# 4. Apply to Gmail
gwcli gmailctl apply

# 5. Use labels in gwcli
gwcli labels list
gwcli messages list --label work
```

## Common Workflows

### Reading and Searching Email

**List messages:**
```bash
# List inbox
gwcli messages list

# List specific label
gwcli messages list --label "Work"

# Unread only
gwcli messages list --unread-only

# Output as JSON for processing
gwcli messages list --json
```

**Search with Gmail query syntax:**
```bash
# Search by sender
gwcli messages search "from:user@example.com"

# Complex queries
gwcli messages search "subject:urgent is:unread has:attachment"

# Date-based searches
gwcli messages search "after:2025/10/01 before:2025/11/01"
gwcli messages search "older_than:30d category:promotions"
```

**Read individual messages:**
```bash
# Read message (default: markdown output with YAML frontmatter)
gwcli messages read <message-id>

# Raw HTML output
gwcli messages read <message-id> --raw-html

# Plain text output
gwcli messages read <message-id> --prefer-plain

# Headers only
gwcli messages read <message-id> --headers-only

# Raw RFC822 format
gwcli messages read <message-id> --raw

# JSON output (includes all body formats)
gwcli messages read <message-id> --json
```

**Output Formats for `messages read`:**

| Flag | Output Format |
|------|---------------|
| (default) | Markdown with YAML frontmatter (HTML auto-converted to markdown) |
| `--raw-html` | Raw HTML with HTML-formatted headers |
| `--prefer-plain` | Plain text with YAML frontmatter |
| `--raw` | Raw RFC822 format |

### Sending Email

**Send simple email:**
```bash
gwcli messages send \
  --to user@example.com \
  --subject "Hello" \
  --body "Message text"
```

**Send with attachments:**
```bash
gwcli messages send \
  --to user@example.com \
  --subject "Documents" \
  --attach report.pdf \
  --attach data.xlsx \
  --body "Please review"
```

**Multiple recipients:**
```bash
gwcli messages send \
  --to user1@example.com \
  --to user2@example.com \
  --cc manager@example.com \
  --bcc admin@example.com \
  --subject "Team Update" \
  --body "Update text"
```

**Body from stdin:**
```bash
cat message.txt | gwcli messages send \
  --to user@example.com \
  --subject "Report"
```

### Batch Operations

gwcli supports `--stdin` for batch processing. Common pattern:

```bash
gwcli messages search "<query>" --json | \
  jq -r '.[].id' | \
  gwcli messages <operation> --stdin [flags]
```

**Archive read messages:**
```bash
gwcli messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gwcli messages move --stdin --to "Archive"
```

**Delete old promotional emails:**
```bash
gwcli messages search "category:promotions older_than:30d" --json | \
  jq -r '.[].id' | \
  gwcli messages delete --stdin --force
```

**Apply label to search results:**
```bash
gwcli messages search "from:vip@company.com is:unread" --json | \
  jq -r '.[].id' | \
  gwcli labels apply "VIP-Unread" --stdin
```

**Bulk mark as read:**
```bash
gwcli messages list --unread-only --json | \
  jq -r '.[].id' | \
  gwcli messages mark-read --stdin
```

### Label Management

**List labels:**
```bash
# All labels (loaded from config.jsonnet + system labels)
gwcli labels list

# User-created only
gwcli labels list --user-only

# System labels only
gwcli labels list --system

# JSON output
gwcli labels list --json
```

**Create/delete labels:**

Labels are managed via `config.jsonnet`, not directly by gwcli. To add labels:

1. Edit config.jsonnet:
```bash
vim ~/.config/gwcli/config.jsonnet
```

2. Add labels to the labels array:
```jsonnet
{
  version: "v1alpha3",
  labels: [
    { name: "work" },
    { name: "personal" },
    { name: "work/projects" },     // Nested label
    { name: "urgent" },
  ],
  rules: []
}
```

3. Apply to Gmail:
```bash
gwcli gmailctl apply
```

Or use gmailctl's interactive editor:
```bash
gmailctl --config ~/.config/gwcli edit
```

**Apply/remove labels to messages:**
```bash
# Apply to single message
gwcli labels apply "Important" --message <message-id>

# Remove from single message
gwcli labels remove "Spam" --message <message-id>

# Batch apply from search
gwcli messages search "subject:invoice has:attachment" --json | \
  jq -r '.[].id' | \
  gwcli labels apply "Invoices" --stdin
```

### Attachment Operations

**List attachments:**
```bash
# List with index numbers for easy selection
gwcli attachments list <message-id>

# JSON output includes index field
gwcli attachments list <message-id> --json
```

**Download attachments:**
```bash
# Download all attachments (defaults to ~/Downloads)
gwcli attachments download <message-id>

# Download specific attachment by index (0-based)
gwcli attachments download <message-id> --index 0
gwcli attachments download <message-id> -i 0

# Download multiple attachments (comma-separated or multiple flags)
gwcli attachments download <message-id> --index 0,1,2
gwcli attachments download <message-id> -i 0 -i 1 -i 2

# Download by filename pattern (glob matching)
gwcli attachments download <message-id> --filename "*.pdf"
gwcli attachments download <message-id> -f "invoice*.xlsx"

# Download to specific directory
gwcli attachments download <message-id> --output-dir ./downloads

# Download single attachment to specific file
gwcli attachments download <message-id> --index 0 --output report.pdf
```

**Important notes:**
- Attachments are indexed starting at 0 (shown in `messages read` and `attachments list`)
- Default download location is `~/Downloads` (created if doesn't exist)
- Filename conflicts are automatically resolved with ` (n)` suffix (e.g., `file.pdf` → `file (1).pdf`)
- Use `--index` or `--filename` for reliable selection (NOT attachment IDs, which change between API calls)

**Download all attachments from label:**
```bash
gwcli messages list --label "Invoices" --json | \
  jq -r '.[].id' | \
  while read id; do
    gwcli attachments download "$id" --output-dir ./invoices
  done
```

### Google Tasks

**Task Lists:**
```bash
# List all task lists
gwcli tasklists list
gwcli tasklists list --json

# Create a new task list
gwcli tasklists create "Work Projects"

# Delete a task list
gwcli tasklists delete <tasklist-id>
gwcli tasklists delete <tasklist-id> --force
```

**Tasks:**
```bash
# List tasks in a task list
gwcli tasks list <tasklist-id>
gwcli tasks list <tasklist-id> --include-completed
gwcli tasks list <tasklist-id> --json

# Create a new task
gwcli tasks create <tasklist-id> --title "Review PR"
gwcli tasks create <tasklist-id> --title "Review PR" --notes "Check tests" --due "2025-12-31T00:00:00Z"

# Read task details
gwcli tasks read <tasklist-id> <task-id>
gwcli tasks read <tasklist-id> <task-id> --json

# Mark task as completed
gwcli tasks complete <tasklist-id> <task-id>

# Delete a task
gwcli tasks delete <tasklist-id> <task-id>
gwcli tasks delete <tasklist-id> <task-id> --force
```

### Google Calendar

**Calendars:**
```bash
# List all accessible calendars
gwcli calendars list
gwcli calendars list --json
gwcli calendars list --min-access-role owner
```

**Events:**
```bash
# List upcoming events (default: primary calendar)
gwcli events list
gwcli events list work@group.calendar.google.com

# List events in date range
gwcli events list --time-min "2025-01-01T00:00:00Z" --time-max "2025-01-31T23:59:59Z"

# Search events
gwcli events list --query "meeting"

# Get event details
gwcli events read <event-id>
gwcli events read <calendar-id> <event-id>
gwcli events read <event-id> --json

# Create event with full details
gwcli events create --summary "Team Meeting" --start "2025-01-15T10:00:00Z" --end "2025-01-15T11:00:00Z"
gwcli events create --summary "All Day Event" --start "2025-01-15" --all-day
gwcli events create --summary "Meeting" --start "2025-01-15T10:00:00Z" --attendee alice@example.com --attendee bob@example.com

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
gwcli events updated --updated-min "2025-01-01T00:00:00Z"

# Detect scheduling conflicts
gwcli events conflicts
gwcli events conflicts --time-max "2025-02-01T00:00:00Z"

# Import ICS file
gwcli events import --file meeting.ics
gwcli events import --file meeting.ics --dry-run
cat events.ics | gwcli events import --file -
```

**Reminders:**

Reminder format: `<number>[w|d|h|m] [popup|email]`

Examples:
- `15` or `15m` - 15 minutes before, popup notification
- `1h` - 1 hour before, popup
- `2d popup` - 2 days before, popup
- `1w email` - 1 week before, email

```bash
gwcli events create --summary "Meeting" --start "2025-01-15T10:00:00Z" \
    --reminder "15m popup" --reminder "1h email"
```

## Global Flags

Available on all commands:

- `--config <path>` - Config directory path (default: ~/.config/gwcli)
- `--user <email>` - User email for service account impersonation
- `--json` - Output in JSON format for programmatic processing
- `--verbose` - Enable verbose logging (shows which config.jsonnet is loaded)
- `--no-color` - Disable colored output

## Important Behaviors

### Safety Features

**Destructive operations require confirmation:**
- `gwcli messages delete` requires `--force` flag
- `gwcli tasklists delete` requires `--force` flag
- `gwcli tasks delete` requires `--force` flag
- `gwcli events delete` requires `--force` flag

This prevents accidental data loss.

### Move Semantics

The `move` command:
- Removes the `INBOX` label
- Adds the target label
- Use for archiving or organizing messages

```bash
# Archive (remove from inbox)
gwcli messages move <message-id> --to "Archive"

# Move to project folder
gwcli messages move <message-id> --to "Work/Projects/Q4"
```

### Label Resolution

Labels can be referenced by:
- Name (case-insensitive): `"Important"`, `"work/projects"`
- ID: `Label_123`

Nested labels use `/` separator: `"Work/Projects/Q4"`

### Output Formats

**Text mode (default):**
- Human-readable tables
- Colored output (disable with `--no-color`)
- Good for interactive use

**JSON mode (`--json` flag):**
- Machine-readable structured data
- Essential for piping to `jq` or other tools
- Required for batch processing workflows

### Exit Codes

- `0` - Success
- `1` - User error (invalid arguments)
- `2` - API error
- `3` - Authentication error
- `4` - Not found

Use for error handling in scripts:

```bash
if ! gwcli messages read "$msg_id" 2>/dev/null; then
  case $? in
    3) echo "Authentication failed" ;;
    4) echo "Message not found" ;;
    *) echo "Error occurred" ;;
  esac
fi
```

## Gmail Query Syntax

Use in `gwcli messages search "<query>"`:

**Sender/Recipient:**
- `from:user@example.com` - From specific sender
- `to:user@example.com` - To specific recipient

**Content:**
- `subject:keyword` - Subject contains keyword
- `has:attachment` - Has attachments
- `filename:pdf` - Specific attachment type

**Status:**
- `is:unread` - Unread messages
- `is:read` - Read messages
- `is:starred` - Starred messages
- `is:important` - Important messages

**Date:**
- `after:2025/01/01` - After specific date
- `before:2025/12/31` - Before specific date
- `older_than:30d` - Older than 30 days
- `newer_than:7d` - Newer than 7 days

**Categories/Labels:**
- `category:promotions` - In Gmail category
- `category:social` - Social category
- `category:updates` - Updates category
- `label:Important` - Has specific label

**Combine with boolean operators:**
```bash
# AND (implicit)
"from:boss@company.com is:unread"

# OR
"from:alice@example.com OR from:bob@example.com"

# NOT
"from:notifications -subject:merged"
```

## Advanced Patterns

### Conditional Processing

Process messages based on content:

```bash
gwcli messages list --label "Inbox" --json | \
  jq -r '.[] | select(.from | contains("@company.com")) | .id' | \
  gwcli labels apply "Internal" --stdin
```

### Periodic Cleanup

Delete old messages by category:

```bash
# Delete old promotions (90+ days)
gwcli messages search "category:promotions older_than:90d" --json | \
  jq -r '.[].id' | \
  gwcli messages delete --stdin --force

# Delete old social emails (60+ days)
gwcli messages search "category:social older_than:60d" --json | \
  jq -r '.[].id' | \
  gwcli messages delete --stdin --force
```

### Attachment Extraction Pipeline

Extract and organize attachments:

```bash
LABEL="Invoices"
OUTPUT_DIR="./invoices/$(date +%Y-%m)"

gwcli messages list --label "$LABEL" --json | \
  jq -r '.[].id' | \
  while read id; do
    echo "Processing message: $id"
    gwcli attachments download "$id" --output-dir "$OUTPUT_DIR"
  done

echo "Downloaded to: $OUTPUT_DIR"
```

### Smart Label Application

Apply labels based on sender domain:

**Note:** Labels must be defined in `~/.config/gwcli/config.jsonnet` first.

```bash
DOMAINS=("company.com" "partner.org" "client.net")

for domain in "${DOMAINS[@]}"; do
  label="Emails/${domain}"
  gwcli messages search "from:@${domain} newer_than:7d" --json | \
    jq -r '.[].id' | \
    gwcli labels apply "$label" --stdin
done
```

### Email Monitoring

Monitor for specific emails:

```bash
QUERY="subject:urgent is:unread"

while true; do
  COUNT=$(gwcli messages search "$QUERY" --json | jq 'length')

  if [ "$COUNT" -gt 0 ]; then
    echo "[$(date)] Found $COUNT urgent unread messages"
    gwcli messages search "$QUERY" --json | \
      jq -r '.[] | "\(.from): \(.subject)"'
  fi

  sleep 300  # Check every 5 minutes
done
```

### Calendar Conflict Detection

Check for scheduling conflicts before creating events:

```bash
# Check next week for conflicts
gwcli events conflicts --time-max "$(date -d '+7 days' -Iseconds)"

# Create event only if no conflicts at that time
gwcli events create --summary "Planning Meeting" \
  --start "2025-01-15T14:00:00Z" \
  --end "2025-01-15T15:00:00Z"
```

### Task Management Workflow

```bash
# Get default task list ID
TASKLIST=$(gwcli tasklists list --json | jq -r '.[0].id')

# Create tasks from a list
while read task; do
  gwcli tasks create "$TASKLIST" --title "$task"
done << EOF
Review PR #123
Update documentation
Send weekly report
EOF

# List incomplete tasks
gwcli tasks list "$TASKLIST" --json | jq '.[] | select(.status != "completed")'
```

## Working with JSON Output

When using `--json`, pipe through `jq` for filtering:

**Extract specific fields:**
```bash
gwcli messages list --json | jq '.[] | {id, subject, from}'
```

**Filter by criteria:**
```bash
# Messages from specific domain
gwcli messages list --json | \
  jq '.[] | select(.from | contains("@company.com"))'

# Messages with attachments (check labels)
gwcli messages list --json | \
  jq '.[] | select(.labels | contains(["IMPORTANT"]))'

# Unread messages only
gwcli messages list --json | \
  jq '.[] | select(.labels | contains(["UNREAD"]))'
```

**Count and statistics:**
```bash
# Count unread messages
gwcli messages list --json | \
  jq '[.[] | select(.labels | contains(["UNREAD"]))] | length'

# Count by label
gwcli messages list --json | \
  jq 'group_by(.labels[]) | map({label: .[0].labels[0], count: length})'
```

## Service Account Usage

For Google Workspace accounts using service accounts with domain-wide delegation:

```bash
# List messages for a specific user
gwcli --user user@example.com messages list

# List task lists for a user
gwcli --user user@example.com tasklists list

# Create calendar event for a user
gwcli --user user@example.com events create --summary "Meeting" --start "2025-01-15T10:00:00Z"
```

Note: Service accounts require domain-wide delegation with appropriate scopes authorized in Google Workspace Admin Console.

## Resources

### references/commands.md

Complete command reference with detailed documentation of all commands, flags, and options. Use this reference when:
- Looking up specific command syntax
- Understanding flag options
- Finding examples of command usage
- Learning about exit codes and output formats

Load with: `Read references/commands.md`

## Implementation Guidelines

When helping users with gwcli:

1. **Always check authentication first** - Ensure `gwcli configure` has been run
2. **Ensure config.jsonnet exists** - gwcli requires `~/.config/gwcli/config.jsonnet` for label definitions. Use `gwcli gmailctl download` to create it if missing
3. **Use JSON for automation** - Add `--json` flag when building pipelines
4. **Include safety flags** - Remind users about `--force` for destructive operations
5. **Leverage stdin for batches** - Use `--stdin` pattern for processing multiple messages
6. **Provide complete commands** - Include all necessary flags in examples
7. **Test incrementally** - Suggest testing commands on small datasets first
8. **Handle errors gracefully** - Include error checking in automation scripts
9. **Use verbose mode for debugging** - Add `--verbose` when troubleshooting (shows which config.jsonnet is loaded)
10. **Manage labels via gmailctl** - Labels are created/deleted in config.jsonnet, not via gwcli commands

## Common User Requests

### "Archive all read emails"
```bash
gwcli messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gwcli messages move --stdin --to "Archive"
```

### "Delete emails from a sender"
```bash
gwcli messages search "from:unwanted@spam.com" --json | \
  jq -r '.[].id' | \
  gwcli messages delete --stdin --force
```

### "Download all PDF attachments"
```bash
gwcli messages search "has:attachment filename:pdf" --json | \
  jq -r '.[].id' | \
  while read id; do
    gwcli attachments download "$id" --filename "*.pdf" --output-dir ./pdfs
  done
```

### "Label emails by project"
```bash
gwcli messages search "subject:ProjectX" --json | \
  jq -r '.[].id' | \
  gwcli labels apply "Work/ProjectX" --stdin
```

### "Send email to multiple recipients"
```bash
gwcli messages send \
  --to person1@example.com \
  --to person2@example.com \
  --to person3@example.com \
  --subject "Team Update" \
  --body "Weekly update text"
```

### "Find and extract invoices"
```bash
gwcli messages search "subject:invoice has:attachment" --json | \
  jq -r '.[].id' | \
  while read id; do
    gwcli attachments download "$id" --output-dir ./invoices
    echo "$id" | gwcli labels apply "Processed" --stdin
  done
```

### "Create a task with due date"
```bash
gwcli tasks create <tasklist-id> \
  --title "Complete quarterly report" \
  --notes "Include Q4 financials" \
  --due "2025-01-31T00:00:00Z"
```

### "Schedule a meeting"
```bash
gwcli events create \
  --summary "Project Review" \
  --start "2025-01-20T14:00:00Z" \
  --end "2025-01-20T15:00:00Z" \
  --location "Conference Room A" \
  --attendee alice@example.com \
  --attendee bob@example.com \
  --reminder "15m popup"
```

### "Check my schedule for conflicts"
```bash
gwcli events conflicts --time-max "$(date -d '+14 days' -Iseconds)"
```
