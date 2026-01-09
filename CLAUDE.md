# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**gwcli** is a command-line Gmail and Google Tasks client forked from cmdg. Unlike the original cmdg (which was a TUI email client), gwcli is a pure CLI tool designed for non-interactive use, shell scripting, and AI agent integration.

**Key architectural change from cmdg:** The TUI components have been completely removed. All interactive terminal UI code (Pine/Alpine-like interface) has been stripped out, leaving only the CLI command structure.

## Build & Development Commands

```bash
# Build the binary
go build -o gwcli .
# OR use justfile
just build

# Run tests
go test ./...
just test

# Run without building
go run . <subcommand>
just run <subcommand>

# Install to GOPATH/bin
go install .
just install

# Format and lint
go fmt ./...
go vet ./...
just vet
```

## Testing Individual Components

```bash
# Test specific package
go test ./pkg/gwcli
go test ./pkg/gwcli/gmailctl/...
go test ./pkg/gpg

# Run specific test
go test -run TestFunctionName ./pkg/gwcli

# Verbose test output
go test -v ./...
```

## Code Architecture

### Command Structure (Kong CLI)

The CLI uses **Kong** for command parsing (not Cobra). Command definitions are in `main.go` as a struct:

```go
type CLI struct {
    Messages struct {
        List struct { ... } `cmd:""`
        Read struct { ... } `cmd:""`
    } `cmd:""`
    Labels struct { ... } `cmd:""`
}
```

Commands are dispatched via switch statement on `ctx.Command()` which returns strings like `"messages list"` or `"messages read <message-id>"`.

### Core Package: pkg/gwcli

**`pkg/gwcli/connection.go`** contains the main `CmdG` struct which manages:
- Gmail, Drive, and People API clients
- Message and label caches
- OAuth2 authentication
- Connection state

The `CmdG` struct is the central object passed to all command handlers.

**`pkg/gwcli/message.go`** handles message parsing, MIME multipart processing, and email body extraction (both plain text and HTML).

**`pkg/gwcli/configure.go`** handles OAuth2 setup flow.

### Command Handlers (root directory)

- **`messages.go`** - All message operations (list, read, search, send, delete, mark read/unread)
- **`labels.go`** - Label operations (list, create, delete, apply, remove)
- **`attachments.go`** - Attachment listing and downloading
- **`batch.go`** - Batch operations (reading multiple message IDs from stdin)
- **`output.go`** - Output formatting (JSON vs tabular text)
- **`format.go`** - Email formatting (markdown, HTML, plain text with YAML frontmatter)
- **`tasklists.go`** - Task list operations (list, create, delete)
- **`tasks.go`** - Task operations (list, read, create, complete, delete)

Each command handler follows the pattern:
```go
func runMessagesXXX(ctx context.Context, conn *cmdg.CmdG, ..., out *outputWriter) error
```

### Output System

The `outputWriter` (in `output.go`) abstracts JSON vs text output:
- `out.writeJSON(data)` - JSON output
- `out.writeTable(headers, rows)` - Tabular text output
- `out.writeMessage(msg)` - Plain text message

All commands check `out.json` flag to determine output format.

### Authentication Flow

1. User runs `gwcli configure`
2. `runConfigure()` calls `gwcli.Configure()` which opens browser for OAuth consent
3. Saves tokens to `~/.config/gwcli/token.json` (JSON format)
4. Subsequent commands load config via `getConnection()` which creates authenticated `CmdG` instance

### Batch Operations

Commands support `--stdin` flag to read multiple message IDs from stdin (one per line). Implemented in `batch.go`:

```go
func readIDsFromStdin() ([]string, error)
```

This enables pipeable workflows:
```bash
gwcli messages search "old" | jq -r '.[].id' | gwcli messages delete --stdin --force
```

## Important Implementation Details

### Gmail API Data Levels

Messages can be fetched at different detail levels (defined in `connection.go`):
- `LevelEmpty` - Nothing
- `LevelMinimal` - ID, labels only
- `LevelMetadata` - ID, labels, headers
- `LevelFull` - ID, labels, headers, payload (complete message)

Use minimal levels for listing, full level for reading content.

### Label Resolution

Labels can be specified by name OR ID. The code normalizes them:
```go
// In messages.go and labels.go
for _, l := range conn.Labels() {
    if strings.EqualFold(l.Label, labelName) || l.ID == labelID {
        // Found it
    }
}
```

System labels (INBOX, UNREAD, SENT, etc.) are uppercase IDs.

### Email Output Formats

The `gwcli messages read` command supports three output formats:

1. **Markdown (default)**: Converts HTML email to markdown with YAML frontmatter
   ```bash
   gwcli messages read <message-id>
   ```

2. **Raw HTML** (`--raw-html`): Outputs raw HTML body with HTML-formatted headers/attachments
   ```bash
   gwcli messages read <message-id> --raw-html
   ```

3. **Plain Text** (`--prefer-plain`): Outputs plain text body with YAML frontmatter
   ```bash
   gwcli messages read <message-id> --prefer-plain
   ```

**Frontmatter includes actionable IDs:**
- `message_id`: For follow-up operations
- `thread_id`: For thread operations
- `attachments[].attachment_id`: For downloading attachments

**JSON output** (`--json`) includes all body formats: `body`, `bodyHtml`, `bodyMarkdown`

**Body Selection Logic:**
- Markdown mode: HTML → markdown (fallback: plain text → snippet)
- HTML mode: HTML (fallback: plain text wrapped in `<pre>`)
- Plain text mode: Plain text (fallback: HTML stripped → snippet)

Email body extraction logic (in `message.go`):
1. Check for `text/plain` or `text/html` parts
2. For multipart messages, recursively search parts
3. Fall back to snippet if no body found

## Configuration

### Configuration Location

Location: `~/.config/gwcli/`

Required files:
- `credentials.json` - OAuth client credentials from Google Cloud Console
- `token.json` - Auto-generated OAuth access/refresh tokens

Optional files:
- `config.jsonnet` - gmailctl-compatible label definitions (Jsonnet format)

### Authentication Setup

gwcli supports two authentication methods:

#### OAuth 2.0 (Regular Gmail Accounts)

1. Go to https://console.developers.google.com
2. Create a new project (or select existing)
3. Enable Gmail API
4. Create OAuth 2.0 Client ID (Desktop app)
5. Download credentials as JSON
6. Save to `~/.config/gwcli/credentials.json`
7. Run `gwcli configure`

Required OAuth scopes:
- `https://www.googleapis.com/auth/gmail.modify`
- `https://www.googleapis.com/auth/gmail.settings.basic`
- `https://www.googleapis.com/auth/gmail.labels`
- `https://www.googleapis.com/auth/tasks`

#### Service Account (Google Workspace Domain-Wide Delegation)

For Google Workspace accounts, you can use a service account to impersonate users without requiring browser-based OAuth:

1. Go to https://console.developers.google.com
2. Create a new project (or select existing)
3. Enable Gmail API
4. Create a Service Account
5. Enable domain-wide delegation for the service account
6. Download the service account credentials as JSON
7. Save to `~/.config/gwcli/credentials.json`
8. In your Google Workspace Admin Console, authorize the service account with the required scopes:
   - `https://www.googleapis.com/auth/gmail.modify`
   - `https://www.googleapis.com/auth/gmail.settings.basic`
   - `https://www.googleapis.com/auth/gmail.labels`
   - `https://www.googleapis.com/auth/tasks`
9. Use the `--user` flag to specify which user to impersonate:
   ```bash
   gwcli --user user@example.com messages list
   ```

**Note**: Service accounts automatically detect the credential type. No `configure` step is needed for service accounts.

**Alternative**: You can also create a separate config directory for each user:
```bash
gwcli --config ~/.config/gwcli/user1 --user user1@example.com messages list
gwcli --config ~/.config/gwcli/user2 --user user2@example.com messages list
```

### Label Management

**Label Discovery:**
gwcli reads label definitions exclusively from:
1. `~/.config/gwcli/config.jsonnet` (required - no API fallback)
2. System labels (INBOX, TRASH, etc.) are automatically added

If config.jsonnet is missing, gwcli will error with a helpful message pointing to gmailctl setup instructions.

**Label Operations:**
- `gwcli labels list` - List all labels
- `gwcli labels apply <label> --message <id>` - Apply label to message
- `gwcli labels remove <label> --message <id>` - Remove label from message

**Creating/Deleting Labels:**
Use gmailctl for label management:
```bash
# Install gmailctl
go install github.com/mbrt/gmailctl/cmd/gmailctl@latest

# Edit label config
gmailctl edit

# See: https://github.com/mbrt/gmailctl for full documentation
```

### gmailctl Integration

gwcli integrates gmailctl functionality natively for filter and label management. The gmailctl library code has been vendored into `pkg/gwcli/gmailctl/`, providing native access to filter parsing, diff computation, and Gmail API operations.

**Commands (in gmailctl.go):**
- `gwcli gmailctl download [-o file]` - Download filters from Gmail to config.jsonnet
- `gwcli gmailctl apply [-y]` - Apply config.jsonnet to Gmail (with optional skip confirmation)
- `gwcli gmailctl diff` - Show diff between local config and Gmail

**Implementation:**
The gmailctl commands are implemented natively using vendored gmailctl library code in `pkg/gwcli/gmailctl/`. This provides:
- Native integration with gwcli's authentication (OAuth and service accounts)
- Better error handling and messaging
- No external binary dependency

The commands use `gwcli.InitializeAuth()` to obtain an authenticated Gmail service, then wrap it with `gmailctl.NewGmailAPI()` for filter/label operations.

**For advanced gmailctl commands** (edit, test, debug), users can install and run gmailctl directly:
```bash
gmailctl --config ~/.config/gwcli <command>
```

Example `~/.config/gwcli/config.jsonnet`:

```jsonnet
{
  version: 'v1alpha3',
  labels: [
    { name: 'work' },
    { name: 'personal' },
    { name: 'receipts' },
  ],
  rules: [
    {
      filter: { from: 'example.com' },
      actions: { labels: ['work'] }
    }
  ]
}
```

You can also symlink to gmailctl's config:
```bash
ln -s ~/.gmailctl/config.jsonnet ~/.config/gwcli/config.jsonnet
```

## Google Tasks Commands

gwcli supports Google Tasks API for managing task lists and tasks.

### Task Lists

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

### Tasks

```bash
# List tasks in a task list
gwcli tasks list <tasklist-id>
gwcli tasks list <tasklist-id> --include-completed
gwcli tasks list <tasklist-id> --json

# Create a new task
gwcli tasks create <tasklist-id> --title "Review PR"
gwcli tasks create <tasklist-id> --title "Review PR" --notes "Check tests" --due "2024-12-31T00:00:00Z"

# Read task details
gwcli tasks read <tasklist-id> <task-id>
gwcli tasks read <tasklist-id> <task-id> --json

# Mark task as completed
gwcli tasks complete <tasklist-id> <task-id>

# Delete a task
gwcli tasks delete <tasklist-id> <task-id>
gwcli tasks delete <tasklist-id> <task-id> --force
```

### Service Account Usage

For Google Workspace accounts using service accounts:

```bash
# List task lists for a specific user
gwcli --user user@example.com tasklists list

# Create task for a user
gwcli --user user@example.com tasks create <tasklist-id> --title "New Task"
```

Note: Service accounts require domain-wide delegation with the `https://www.googleapis.com/auth/tasks` scope authorized in Google Workspace Admin Console.

## Dependencies

- **Kong** (`github.com/alecthomas/kong`) - CLI parsing (not Cobra)
- **Google APIs** - Gmail API, Tasks API
- **golang.org/x/oauth2** - OAuth2 authentication
- **logrus** - Logging
- **html-to-markdown/v2** (`github.com/JohannesKaufmann/html-to-markdown/v2`) - HTML to Markdown conversion
- **yaml.v3** (`gopkg.in/yaml.v3`) - YAML marshaling for frontmatter
- **go-jsonnet** (`github.com/google/go-jsonnet`) - Jsonnet parsing for gmailctl integration

## Common Gotchas

1. **Config path**: gwcli uses `~/.config/gwcli/` directory (gmailctl-compatible authentication)
2. **Kong syntax**: Command matching uses exact strings like `"messages list"` not path-style routes
3. **No interactive prompts**: All commands must work non-interactively (for scripting)
4. **Label IDs vs Names**: Always handle both - users may provide either
5. **Label create/delete**: Use gmailctl for creating/deleting labels, not gwcli

## Error Handling Pattern

```go
if err != nil {
    out.writeError(err)
    os.Exit(2)  // Exit code 2 for command errors
}
```

Exit codes:
- `0` - Success
- `1` - Unknown command
- `2` - Command execution error
- `3` - Connection/config error
