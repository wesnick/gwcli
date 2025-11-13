# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**gmailcli** is a command-line Gmail client forked from cmdg. Unlike the original cmdg (which was a TUI email client), gmailcli is a pure CLI tool designed for non-interactive use, shell scripting, and AI agent integration.

**Key architectural change from cmdg:** The TUI components have been completely removed. All interactive terminal UI code (Pine/Alpine-like interface) has been stripped out, leaving only the CLI command structure.

## Build & Development Commands

```bash
# Build the binary
go build -o gmailcli ./cmd/gmailcli
# OR use justfile
just build

# Run tests
go test ./...
just test

# Run without building
go run ./cmd/gmailcli <subcommand>
just run <subcommand>

# Install to GOPATH/bin
go install ./cmd/gmailcli
just install

# Format and lint
go fmt ./...
go vet ./...
just vet
```

## Testing Individual Components

```bash
# Test specific package
go test ./pkg/cmdg
go test ./pkg/dialog
go test ./pkg/gpg

# Run specific test
go test -run TestFunctionName ./pkg/cmdg

# Verbose test output
go test -v ./...
```

## Code Architecture

### Command Structure (Kong CLI)

The CLI uses **Kong** for command parsing (not Cobra). Command definitions are in `cmd/gmailcli/main.go` as a struct:

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

### Core Package: pkg/cmdg

**`pkg/cmdg/connection.go`** contains the main `CmdG` struct which manages:
- Gmail, Drive, and People API clients
- Message and label caches
- OAuth2 authentication
- Connection state

The `CmdG` struct is the central object passed to all command handlers.

**`pkg/cmdg/message.go`** handles message parsing, MIME multipart processing, and email body extraction (both plain text and HTML).

**`pkg/cmdg/configure.go`** handles OAuth2 setup flow.

### Command Handlers (cmd/gmailcli/)

- **`messages.go`** - All message operations (list, read, search, send, delete, mark read/unread)
- **`labels.go`** - Label operations (list, create, delete, apply, remove)
- **`attachments.go`** - Attachment listing and downloading
- **`batch.go`** - Batch operations (reading multiple message IDs from stdin)
- **`output.go`** - Output formatting (JSON vs tabular text)
- **`format.go`** - Email formatting (markdown, HTML, plain text with YAML frontmatter)

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

1. User runs `gmailcli configure`
2. `runConfigure()` calls `cmdg.Configure()` which prompts for OAuth client ID/secret
3. Opens browser for OAuth consent
4. Saves tokens to `~/.cmdg/cmdg.conf` (JSON format)
5. Subsequent commands load config via `getConnection()` which creates authenticated `CmdG` instance

### Batch Operations

Commands support `--stdin` flag to read multiple message IDs from stdin (one per line). Implemented in `batch.go`:

```go
func readIDsFromStdin() ([]string, error)
```

This enables pipeable workflows:
```bash
gmailcli messages search "old" | jq -r '.[].id' | gmailcli messages delete --stdin --force
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

The `gmailcli messages read` command supports three output formats:

1. **Markdown (default)**: Converts HTML email to markdown with YAML frontmatter
   ```bash
   gmailcli messages read <message-id>
   ```

2. **Raw HTML** (`--raw-html`): Outputs raw HTML body with HTML-formatted headers/attachments
   ```bash
   gmailcli messages read <message-id> --raw-html
   ```

3. **Plain Text** (`--prefer-plain`): Outputs plain text body with YAML frontmatter
   ```bash
   gmailcli messages read <message-id> --prefer-plain
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

## Configuration File

Location: `~/.cmdg/cmdg.conf`

Format: JSON containing OAuth2 tokens and client credentials. Do not commit this file.

## Dependencies

- **Kong** (`github.com/alecthomas/kong`) - CLI parsing (not Cobra)
- **Google APIs** - Gmail, Drive, People APIs
- **golang.org/x/oauth2** - OAuth2 authentication
- **logrus** - Logging
- **html-to-markdown/v2** (`github.com/JohannesKaufmann/html-to-markdown/v2`) - HTML to Markdown conversion
- **yaml.v3** (`gopkg.in/yaml.v3`) - YAML marshaling for frontmatter

## Common Gotchas

1. **Config path**: Despite being "gmailcli", it uses `~/.cmdg/` directory for backwards compatibility with cmdg
2. **Kong syntax**: Command matching uses exact strings like `"messages list"` not path-style routes
3. **No interactive prompts**: All commands must work non-interactively (for scripting)
4. **Label IDs vs Names**: Always handle both - users may provide either

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
