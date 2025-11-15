# gmailctl - Command-Line Gmail Client

A standalone CLI tool for Gmail operations with a resource-oriented command structure.

## Installation

```bash
go build ./cmd/gmailctl
```

## Configuration

First-time setup requires OAuth authentication:

```bash
./gmailctl configure
```

This will:
1. Open your browser for Google OAuth
2. Save credentials to `~/.cmdg/cmdg.conf`
3. Share authentication with cmdg TUI

## Usage

### Messages

**List messages:**
```bash
gmailctl messages list                      # List inbox
gmailctl messages list --label "Work"       # List specific label
gmailctl messages list --unread-only        # Unread only
gmailctl messages list --json               # JSON output
```

**Read message:**
```bash
gmailctl messages read <message-id>
gmailctl messages read <message-id> --json
gmailctl messages read <message-id> --raw          # RFC822 format
gmailctl messages read <message-id> --headers-only
```

**Search messages:**
```bash
gmailctl messages search "from:user@example.com"
gmailctl messages search "subject:urgent is:unread"
gmailctl messages search "has:attachment after:2025/01/01"
```

**Send message:**
```bash
gmailctl messages send \
  --to user@example.com \
  --subject "Hello" \
  --body "Message text"

# With attachments
gmailctl messages send \
  --to user@example.com \
  --subject "Files" \
  --attach file1.pdf --attach file2.jpg \
  --body "See attached"

# Read body from stdin
echo "Message content" | gmailctl messages send \
  --to user@example.com \
  --subject "Test"
```

**Delete messages:**
```bash
gmailctl messages delete <message-id> --force

# Batch delete from search
gmailctl messages search "from:spam@example.com" --json | \
  jq -r '.[].id' | \
  gmailctl messages delete --stdin --force
```

**Mark read/unread:**
```bash
gmailctl messages mark-read <message-id>
gmailctl messages mark-unread <message-id>

# Batch operations
gmailctl messages list --unread-only --json | \
  jq -r '.[].id' | \
  gmailctl messages mark-read --stdin
```

**Move messages:**
```bash
gmailctl messages move <message-id> --to "Archive"
gmailctl messages move <message-id> --to "Work/Projects"

# Batch move
cat message-ids.txt | gmailctl messages move --stdin --to "Done"
```

### Labels

**List labels:**
```bash
gmailctl labels list
gmailctl labels list --system      # System labels only
gmailctl labels list --user-only   # User labels only
gmailctl labels list --json
```

**Create label:**
```bash
gmailctl labels create "Work/Projects"
gmailctl labels create "Important" --color "#ff0000"
```

**Delete label:**
```bash
gmailctl labels delete "Old Label" --force
```

**Apply/remove labels:**
```bash
gmailctl labels apply "Important" --message <message-id>
gmailctl labels remove "Spam" --message <message-id>

# Batch operations
gmailctl messages search "from:boss@company.com" --json | \
  jq -r '.[].id' | \
  gmailctl labels apply "Important" --stdin
```

### Attachments

**List attachments:**
```bash
gmailctl attachments list <message-id>
gmailctl attachments list <message-id> --json
```

**Download attachments:**
```bash
# Download all attachments
gmailctl attachments download <message-id>

# Download to specific directory
gmailctl attachments download <message-id> --output-dir ./downloads

# Download specific attachment
gmailctl attachments download <message-id> \
  --attachment-id <att-id> \
  --output custom-name.pdf
```

## Global Flags

- `--config <path>` - Config file path (default: ~/.cmdg/cmdg.conf)
- `--json` - JSON output format
- `--verbose` - Verbose logging
- `--no-color` - Disable colored output

## Exit Codes

- 0: Success
- 1: User error (invalid arguments)
- 2: API error
- 3: Authentication error
- 4: Not found

## Examples

**Archive read messages:**
```bash
gmailctl messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gmailctl messages move --stdin --to "Archive"
```

**Download all attachments from a label:**
```bash
gmailctl messages list --label "Invoices" --json | \
  jq -r '.[].id' | \
  while read id; do
    gmailctl attachments download "$id" --output-dir ./invoices
  done
```

**Delete old promotional emails:**
```bash
gmailctl messages search "category:promotions older_than:30d" --json | \
  jq -r '.[].id' | \
  gmailctl messages delete --stdin --force
```

## Authentication

gmailctl shares the same OAuth configuration as cmdg. If you already use cmdg, no additional setup is needed.

Config location: `~/.cmdg/cmdg.conf`

## License

Same as cmdg project.
