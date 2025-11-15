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

## Global Flags

Available on all commands:

- `--config <path>` - Config file path (default: ~/.cmdg/cmdg.conf)
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
gwcli messages read 18a1b2c3d4e5f678

# Read with lynx rendering
gwcli messages read 18a1b2c3d4e5f678 --lynx /usr/bin/lynx

# Get raw message
gwcli messages read 18a1b2c3d4e5f678 --raw

# Get as JSON
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
- `messagesTotal` - Total messages with this label
- `messagesUnread` - Unread messages with this label

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

### gwcli labels create

Create a new label.

**Syntax:**
```bash
gwcli labels create <name> [flags]
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
gwcli labels create "Important"

# Create with color
gwcli labels create "Urgent" --color "#ff0000"

# Create nested label
gwcli labels create "Work/Projects/Q4-2025"

# Create with spaces
gwcli labels create "Client Emails"
```

### gwcli labels delete

Delete a label.

**Syntax:**
```bash
gwcli labels delete <name> [flags]
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
gwcli labels delete "Old Project" --force

# Delete nested label
gwcli labels delete "Archive/2020" --force
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
- `attachmentId` - Attachment ID for downloading
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
gwcli attachments download 18a1b2c3d4e5f678

# Download to specific directory
gwcli attachments download 18a1b2c3d4e5f678 --output-dir ./downloads

# Download specific attachment
gwcli attachments download 18a1b2c3d4e5f678 \
  --attachment-id ANGjdJ9... \
  --output report.pdf

# Download all from label
gwcli messages list --label "Invoices" --json | \
  jq -r '.[].id' | \
  while read id; do
    gwcli attachments download "$id" --output-dir ./invoices
  done
```

## Configuration

### Initial Setup

```bash
gwcli configure
```

This opens a browser for Google OAuth authentication and saves credentials to `~/.cmdg/cmdg.conf`.

### Config File Location

Default: `~/.cmdg/cmdg.conf`

Override with `--config` flag:
```bash
gwcli --config /custom/path/config.conf messages list
```

### Authentication

gwcli shares OAuth configuration with cmdg. If cmdg is already configured, gwcli works immediately without additional setup.

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

```bash
Processing 100 messages...
Progress: 10/100
Progress: 20/100
...
Completed: 95 successful, 5 failed
```

Failed items are logged when `--verbose` is enabled.
