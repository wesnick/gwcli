# gmailcli Command Reference

Complete reference for all gmailcli commands, flags, and usage patterns.

## Command Structure

gmailcli follows a resource-oriented command pattern:

```
gmailcli <resource> <action> [arguments] [flags]
```

## Resources

- **messages** - Email message operations
- **labels** - Gmail label management
- **attachments** - Attachment operations

## Global Flags

Available on all commands:

- `--config <path>` - Config file path (default: ~/.cmdg/cmdg.conf)
- `--json` - Output results in JSON format
- `--verbose` - Enable verbose logging
- `--no-color` - Disable colored output

## Messages Commands

### gmailcli messages list

List messages from inbox or a specific label.

**Syntax:**
```bash
gmailcli messages list [flags]
```

**Flags:**
- `--label <name>` - List messages with specific label (default: INBOX)
- `--unread-only` - Only show unread messages
- `--limit <n>` - Maximum number of messages to retrieve (default: 100)
- `--json` - Output as JSON array
- `--no-color` - Disable colored output

**Output Fields (JSON):**
- `id` - Message ID
- `threadId` - Thread ID
- `snippet` - Message preview text
- `from` - Sender email/name
- `to` - Recipient(s)
- `subject` - Email subject
- `date` - Date sent (RFC3339 format)
- `labels` - Array of label names

**Examples:**
```bash
# List inbox
gmailcli messages list

# List work emails in JSON
gmailcli messages list --label "Work" --json

# List unread messages only
gmailcli messages list --unread-only

# List from custom label
gmailcli messages list --label "Projects/2025"
```

### gmailcli messages read

Read a single message by ID.

**Syntax:**
```bash
gmailcli messages read <message-id> [flags]
```

**Flags:**
- `--raw` - Output raw RFC822 format
- `--headers-only` - Show only headers
- `--lynx <path>` - Use lynx to render HTML (default: output raw HTML)
- `--json` - Output as JSON

**Output:**
- Default: Headers, plain text body, HTML body, and attachment list
- `--raw`: Raw RFC822 message
- `--headers-only`: Just the email headers
- `--json`: Structured JSON with all fields

**Examples:**
```bash
# Read message
gmailcli messages read 18a1b2c3d4e5f678

# Read with lynx rendering
gmailcli messages read 18a1b2c3d4e5f678 --lynx /usr/bin/lynx

# Get raw message
gmailcli messages read 18a1b2c3d4e5f678 --raw

# Get as JSON
gmailcli messages read 18a1b2c3d4e5f678 --json
```

### gmailcli messages search

Search messages using Gmail query syntax.

**Syntax:**
```bash
gmailcli messages search "<query>" [flags]
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
gmailcli messages search "from:boss@company.com"

# Search unread with attachment
gmailcli messages search "is:unread has:attachment"

# Search by date range
gmailcli messages search "after:2025/10/01 before:2025/11/01"

# Complex query
gmailcli messages search "from:notifications@github.com subject:merged is:unread"

# Get results as JSON for piping
gmailcli messages search "label:Invoices" --json
```

### gmailcli messages send

Send an email message.

**Syntax:**
```bash
gmailcli messages send [flags]
```

**Flags:**
- `--to <email>` - Recipient (required, can be repeated)
- `--cc <email>` - CC recipient (can be repeated)
- `--bcc <email>` - BCC recipient (can be repeated)
- `--subject <text>` - Email subject (required)
- `--body <text>` - Email body (if omitted, reads from stdin)
- `--attach <file>` - Attach file (can be repeated)
- `--html` - Send body as HTML
- `--json` - Output result as JSON

**Body Input:**
- Use `--body` flag to specify inline
- Omit `--body` to read from stdin
- Use `--html` for HTML formatted bodies

**Examples:**
```bash
# Simple text email
gmailcli messages send \
  --to user@example.com \
  --subject "Hello" \
  --body "This is the message"

# With attachments
gmailcli messages send \
  --to user@example.com \
  --subject "Documents" \
  --attach report.pdf \
  --attach invoice.xlsx \
  --body "Please review the attached documents"

# Multiple recipients
gmailcli messages send \
  --to user1@example.com \
  --to user2@example.com \
  --cc manager@example.com \
  --subject "Team Update" \
  --body "Here is the update"

# Body from stdin
echo "Message content" | gmailcli messages send \
  --to user@example.com \
  --subject "Test"

# HTML email
gmailcli messages send \
  --to user@example.com \
  --subject "HTML Test" \
  --body "<h1>Hello</h1><p>This is HTML</p>" \
  --html
```

### gmailcli messages delete

Delete messages (move to trash).

**Syntax:**
```bash
gmailcli messages delete <message-id> [flags]
# OR
gmailcli messages delete --stdin [flags]
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
gmailcli messages delete 18a1b2c3d4e5f678 --force

# Batch delete from search
gmailcli messages search "from:spam@example.com" --json | \
  jq -r '.[].id' | \
  gmailcli messages delete --stdin --force

# Delete old promotions
gmailcli messages search "category:promotions older_than:60d" --json | \
  jq -r '.[].id' | \
  gmailcli messages delete --stdin --force
```

### gmailcli messages mark-read

Mark messages as read.

**Syntax:**
```bash
gmailcli messages mark-read <message-id> [flags]
# OR
gmailcli messages mark-read --stdin [flags]
```

**Flags:**
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Examples:**
```bash
# Mark single message
gmailcli messages mark-read 18a1b2c3d4e5f678

# Mark all unread as read
gmailcli messages list --unread-only --json | \
  jq -r '.[].id' | \
  gmailcli messages mark-read --stdin

# Mark search results as read
gmailcli messages search "label:Notifications" --json | \
  jq -r '.[].id' | \
  gmailcli messages mark-read --stdin
```

### gmailcli messages mark-unread

Mark messages as unread.

**Syntax:**
```bash
gmailcli messages mark-unread <message-id> [flags]
# OR
gmailcli messages mark-unread --stdin [flags]
```

**Flags:**
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Examples:**
```bash
# Mark single message
gmailcli messages mark-unread 18a1b2c3d4e5f678

# Mark important messages as unread
gmailcli messages search "from:boss@company.com" --json | \
  jq -r '.[].id' | \
  gmailcli messages mark-unread --stdin
```

### gmailcli messages move

Move messages to a different label (removes INBOX, adds target).

**Syntax:**
```bash
gmailcli messages move <message-id> --to <label> [flags]
# OR
gmailcli messages move --stdin --to <label> [flags]
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
gmailcli messages move 18a1b2c3d4e5f678 --to "Archive"

# Move to nested label
gmailcli messages move 18a1b2c3d4e5f678 --to "Work/Projects/Q4"

# Batch move read messages
gmailcli messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gmailcli messages move --stdin --to "Archive"

# Move search results
gmailcli messages search "label:Done older_than:90d" --json | \
  jq -r '.[].id' | \
  gmailcli messages move --stdin --to "Archive"
```

## Labels Commands

### gmailcli labels list

List all Gmail labels.

**Syntax:**
```bash
gmailcli labels list [flags]
```

**Flags:**
- `--system` - Show only system labels (INBOX, SPAM, etc.)
- `--user-only` - Show only user-created labels
- `--json` - Output as JSON array

**Output Fields:**
- `id` - Label ID
- `name` - Label name
- `type` - SYSTEM or USER
- `messagesTotal` - Total messages with this label
- `messagesUnread` - Unread messages with this label

**Examples:**
```bash
# List all labels
gmailcli labels list

# List user labels only
gmailcli labels list --user-only

# Get as JSON
gmailcli labels list --json

# List system labels
gmailcli labels list --system
```

### gmailcli labels create

Create a new label.

**Syntax:**
```bash
gmailcli labels create <name> [flags]
```

**Flags:**
- `--color <hex>` - Label color in hex format (e.g., #ff0000)
- `--json` - Output result as JSON

**Label Names:**
- Use `/` for nested labels (e.g., "Work/Projects")
- Case sensitive
- Can contain spaces

**Examples:**
```bash
# Create simple label
gmailcli labels create "Important"

# Create with color
gmailcli labels create "Urgent" --color "#ff0000"

# Create nested label
gmailcli labels create "Work/Projects/Q4-2025"

# Create with spaces
gmailcli labels create "Client Emails"
```

### gmailcli labels delete

Delete a label.

**Syntax:**
```bash
gmailcli labels delete <name> [flags]
```

**Flags:**
- `--force` - Required to confirm deletion
- `--json` - Output result as JSON

**Safety:**
- Requires `--force` flag
- Does not delete messages, only removes the label

**Examples:**
```bash
# Delete label
gmailcli labels delete "Old Project" --force

# Delete nested label
gmailcli labels delete "Archive/2020" --force
```

### gmailcli labels apply

Apply a label to messages.

**Syntax:**
```bash
gmailcli labels apply <name> --message <id> [flags]
# OR
gmailcli labels apply <name> --stdin [flags]
```

**Flags:**
- `--message <id>` - Apply to specific message
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Examples:**
```bash
# Apply to single message
gmailcli labels apply "Important" --message 18a1b2c3d4e5f678

# Apply to search results
gmailcli messages search "from:vip@company.com" --json | \
  jq -r '.[].id' | \
  gmailcli labels apply "VIP" --stdin

# Tag invoices
gmailcli messages search "subject:invoice has:attachment" --json | \
  jq -r '.[].id' | \
  gmailcli labels apply "Invoices" --stdin
```

### gmailcli labels remove

Remove a label from messages.

**Syntax:**
```bash
gmailcli labels remove <name> --message <id> [flags]
# OR
gmailcli labels remove <name> --stdin [flags]
```

**Flags:**
- `--message <id>` - Remove from specific message
- `--stdin` - Read message IDs from stdin
- `--json` - Output result as JSON

**Examples:**
```bash
# Remove from single message
gmailcli labels remove "Spam" --message 18a1b2c3d4e5f678

# Remove from multiple messages
gmailcli messages search "label:Todo is:read" --json | \
  jq -r '.[].id' | \
  gmailcli labels remove "Todo" --stdin
```

## Attachments Commands

### gmailcli attachments list

List attachments in a message.

**Syntax:**
```bash
gmailcli attachments list <message-id> [flags]
```

**Flags:**
- `--json` - Output as JSON array

**Output Fields:**
- `attachmentId` - Attachment ID for downloading
- `filename` - Original filename
- `mimeType` - MIME type
- `size` - Size in bytes

**Examples:**
```bash
# List attachments
gmailcli attachments list 18a1b2c3d4e5f678

# Get as JSON
gmailcli attachments list 18a1b2c3d4e5f678 --json
```

### gmailcli attachments download

Download attachments from a message.

**Syntax:**
```bash
gmailcli attachments download <message-id> [flags]
```

**Flags:**
- `--attachment-id <id>` - Download specific attachment only
- `--output-dir <path>` - Output directory (default: current directory)
- `--output <filename>` - Output filename (for single attachment)
- `--json` - Output result as JSON

**Behavior:**
- Without `--attachment-id`: Downloads all attachments
- Preserves original filenames unless `--output` specified
- Creates output directory if it doesn't exist

**Examples:**
```bash
# Download all attachments to current directory
gmailcli attachments download 18a1b2c3d4e5f678

# Download to specific directory
gmailcli attachments download 18a1b2c3d4e5f678 --output-dir ./downloads

# Download specific attachment
gmailcli attachments download 18a1b2c3d4e5f678 \
  --attachment-id ANGjdJ9... \
  --output report.pdf

# Download all from label
gmailcli messages list --label "Invoices" --json | \
  jq -r '.[].id' | \
  while read id; do
    gmailcli attachments download "$id" --output-dir ./invoices
  done
```

## Configuration

### Initial Setup

```bash
gmailcli configure
```

This opens a browser for Google OAuth authentication and saves credentials to `~/.cmdg/cmdg.conf`.

### Config File Location

Default: `~/.cmdg/cmdg.conf`

Override with `--config` flag:
```bash
gmailcli --config /custom/path/config.conf messages list
```

### Authentication

gmailcli shares OAuth configuration with cmdg. If cmdg is already configured, gmailcli works immediately without additional setup.

## Exit Codes

- `0` - Success
- `1` - User error (invalid arguments)
- `2` - API error
- `3` - Authentication error
- `4` - Not found

Use exit codes for error handling in scripts:

```bash
if gmailcli messages read "$msg_id" 2>/dev/null; then
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
18a1b2c3d4e5f678   Meeting Tomorrow       alice@example.com  2025-11-01
19b2c3d4e5f6789a   Project Update         bob@example.com    2025-10-31
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
    "to": ["me@example.com"],
    "date": "2025-11-01T10:30:00Z",
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
gmailcli messages search "<query>" --json | \
  jq -r '.[].id' | \
  gmailcli messages <operation> --stdin [flags]
```

### Common Workflows

**Archive read messages:**
```bash
gmailcli messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gmailcli messages move --stdin --to "Archive"
```

**Delete spam from sender:**
```bash
gmailcli messages search "from:spam@example.com" --json | \
  jq -r '.[].id' | \
  gmailcli messages delete --stdin --force
```

**Apply label to unread from VIP:**
```bash
gmailcli messages search "from:vip@company.com is:unread" --json | \
  jq -r '.[].id' | \
  gmailcli labels apply "VIP-Unread" --stdin
```

**Download all invoices:**
```bash
gmailcli messages search "subject:invoice has:attachment" --json | \
  jq -r '.[].id' | \
  while read id; do
    gmailcli attachments download "$id" --output-dir ./invoices
  done
```

### Error Handling in Batches

Batch operations report errors but continue processing:

```bash
Processing 100 messages...
Progress: 10/100
Progress: 20/100
...
Completed: 95 successful, 5 failed
```

Failed items are logged when `--verbose` is enabled.
