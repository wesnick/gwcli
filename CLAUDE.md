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
- **`labels.go`** - Label operations (list, apply, remove)
- **`filters.go`** - Gmail filter operations (list, get, create, delete)
- **`attachments.go`** - Attachment listing and downloading
- **`batch.go`** - Batch operations (reading multiple message IDs from stdin)
- **`output.go`** - Output formatting (JSON vs tabular text)
- **`format.go`** - Email formatting (markdown, HTML, plain text with YAML frontmatter)
- **`drive_artifacts.go`** - Detect Google Drive docs/files linked from email bodies (Gemini/Meet artifact chips)
- **`artifacts.go`** - `artifacts list`/`artifacts download` commands (export/download detected Drive artifacts)
- **`drive.go`** - `drive get`/`drive export`/`list`/`search`/`update` commands (general Drive file access by ID or URL, not email-specific; includes recursive folder export)
- **`drive_write.go`** - Drive write/organize verbs: `mkdir`/`mv`/`rename`/`cp`/`rm`/`share`/`link`/`permissions` and the upload engine (multi-path, recursive, `--convert`, `--upsert`); shared `writeDriveFile` composable-JSON helper and idempotent `ensureDriveFolder`
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
- `drive_artifacts[]`: Google Drive docs/files linked from the body (see below)

**Drive artifacts** (`drive_artifacts` in frontmatter, `driveArtifacts` in
`--json`): Some emails (notably Gemini/Google Meet "Notes by Gemini" chips)
reference a Google Doc/Drive file via an in-body link rather than a MIME
attachment. These are detected and surfaced here, and can be exported/
downloaded via the `artifacts` command group (see Drive Artifact Commands).
`messages read` itself never auto-fetches their content — same as MIME
attachments. Fetching requires the full Drive scope and Google-native docs
need `drive.Files.Export`, not the Gmail attachment API. Each artifact exposes the
canonical `id` (the stable key — every URL is templated from it), `type`
(`document`/`spreadsheet`/`presentation`/`form`/`folder`/`drive-file`),
`title`, and a cleaned canonical `url`. Detection (`drive_artifacts.go`) only
keeps a link if it carries Google's `artifact-chip`/`link-button` class or its
ID appears in the decoded `X-Meet-Artifact-Email-Metadata` header, so ordinary
emails with incidental Drive links stay noise-free. Links are deduped by ID.

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
- `https://www.googleapis.com/auth/calendar`
- `https://www.googleapis.com/auth/drive` (for `artifacts download`)

**Scope change note:** the full `https://www.googleapis.com/auth/drive`
scope was added for Drive artifact export/download (full, not
`drive.readonly`, intentionally — leaves room for future write use such as
attaching Drive files to outgoing mail). Existing OAuth users must re-run
`gwcli configure` to re-consent — Google does not grant new scopes to an
already-issued `token.json`. `artifacts list` does **not** need this scope
(Gmail only); only `artifacts download` does.

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
   - `https://www.googleapis.com/auth/calendar`
   - `https://www.googleapis.com/auth/drive` (only for `artifacts download`; Gmail/Tasks/Calendar work without it). The service-account `DriveService` requests the full `drive` scope, so domain-wide delegation must authorize `https://www.googleapis.com/auth/drive` for the service account's Client ID (the numeric `client_id` from `credentials.json`). DWD token exchange is per-scope-set; an entry authorizing only `drive.readonly` will **not** satisfy it.
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
gwcli loads labels directly from the Gmail API on first access (lazily, cached
on the `CmdG` connection). All user labels are included, plus the Gmail
built-in system labels (INBOX, TRASH, UNREAD, etc.). There is no config file
involved — nothing is auto-created.

**Label Operations:**
- `gwcli labels list` - List all labels
- `gwcli labels apply <label> --message <id>` - Apply label to message
- `gwcli labels remove <label> --message <id>` - Remove label from message

Labels are created/deleted in the Gmail UI. gwcli does not manage label
creation/deletion.

### Filter Management

gwcli manages Gmail filters imperatively via direct Gmail API CRUD
(`users.settings.filters`). Implemented in `filters.go`.

- `gwcli filters list` - List all filters as a table (`--json` for full detail)
- `gwcli filters get <filter-id>` - Show one filter's details
- `gwcli filters create [flags]` - Create a filter
- `gwcli filters delete <filter-id> --force` - Delete a filter (`--force` required; non-interactive)

`filters create` flags:
- Criteria: `--from`, `--to`, `--subject`, `--query`, `--has-attachment`
  (at least one required)
- Actions: `--add-label`/`--remove-label` (repeatable, accept label name or ID),
  plus shortcuts `--archive` (remove INBOX), `--mark-read` (remove UNREAD),
  `--star`, `--important`, `--trash`, `--forward <addr>` (at least one required)

Label names in `--add-label`/`--remove-label` are resolved to IDs via the
connection's label cache (same name/ID matching used by `labels.go`). Filter
label IDs are resolved back to names for table/JSON display.

The Gmail API has no filter "update" — to change a filter, delete it and
create a new one. The canonical list of an account's filters is meant to live
in the editable skill doc (`claude-skill-gwcli/SKILL.md`) as a table that an
agent can recreate via individual `filters create` calls.

### Drive Artifact Commands

Some emails (notably Gemini/Google Meet "Notes by Gemini" chips) link a
Google Doc/Drive file in the body rather than attaching a MIME part. Phase 1
surfaces these in `messages read` frontmatter/`--json` (`drive_artifacts`).
The `artifacts` command group (mirrors `attachments` 1:1) lists and
fetches them:

```bash
# List Drive artifacts linked from a message (Gmail only — no Drive scope)
gwcli artifacts list <message-id>
gwcli artifacts list <message-id> --json

# Download/export an artifact (requires the full drive scope)
gwcli artifacts download <message-id> -i 0 --output notes.md
gwcli artifacts download <message-id> --filename "Notes*" --output-dir ~/Downloads
```

Selection flags match `attachments download`: `--index/-i` (0-based,
comma-separated/repeatable), `--filename/-f` (glob, matched against the
artifact **title**), `--output-dir` (default `~/Downloads`), `--output`
(single-artifact only). No selection = all artifacts.

**Export behavior** (`artifacts.go`): native Google-apps files are
**exported** via `drive.Files.Export` (not blob-downloaded) — Docs →
`text/markdown` (`.md`), Sheets → `text/csv`, Slides/unknown → PDF, Drawings →
PNG. Uploaded (binary) files use `Files.Get(alt=media)`. Folders are
rejected. The canonical Drive file `id` is the stable key; every URL is
templated from it.

**Auth failures are made actionable**: `wrapDriveErr` detects both the
OAuth insufficient-scope case and the service-account domain-wide-delegation
`unauthorized_client` case, and tells the user exactly how to grant the
full `drive` scope (re-run `gwcli configure`, or authorize
`https://www.googleapis.com/auth/drive` for the service account's numeric
Client ID via domain-wide delegation in the Workspace Admin console).

### Drive Commands (general, not email-specific)

The `artifacts` plumbing is file-ID-driven and source-agnostic (only
`extractDriveArtifacts` HTML scraping is email-specific). The `drive`
command group (`drive.go`) exposes it directly for any Drive file by ID
**or** Drive/Docs URL — no email involved:

```bash
# Metadata only (no download): id, name, mimeType, size, modifiedTime, owners
gwcli drive get <file-id|url>
gwcli drive get https://docs.google.com/document/d/<id>/edit --json

# Export/download by ID or URL (requires the full drive scope)
gwcli drive export <file-id|url> --output notes.md
gwcli drive export <file-id|url> --output-dir ~/Downloads
gwcli drive export <file-id|url> --export-format pdf   # override per-type default

# List / search (paginated, all drives)
gwcli drive list --query "mimeType='application/pdf'" --limit 50
gwcli drive list --folder <folder-id>
gwcli drive search "quarterly plan"

# Write ops (full drive scope, unblocked since #40 took the full scope)
gwcli drive upload ./report.pdf --folder <folder-id> --name "Q3 Report.pdf"
gwcli drive upload *.csv --folder <folder-id> --convert       # CSV->Sheet, MD->Doc, ...
gwcli drive upload ./package-dir --folder <folder-id>          # recurses, mirrors structure
gwcli drive upload ./01-catalog.csv --folder <folder-id> --upsert  # idempotent rerun
gwcli drive update <file-id|url> ./report.pdf --name "Q3 Report (final).pdf"

# Organize: folders + move turn a pile of files into a deliverable
gwcli drive mkdir "Intern Package" --folder <parent-id|url>    # reuses same-name folder by default
gwcli drive mkdir "Intern Package" --no-dedupe                 # force a fresh folder
gwcli drive mv <file-id|url> --folder <folder-id|url>          # reparent
gwcli drive rename <file-id|url> "New Name"                    # name only, content untouched
gwcli drive cp <file-id|url> --name "Copy" --folder <dest>     # template instantiation

# Delete / undo a bad upload (--force required; trash is reversible)
gwcli drive rm <file-id|url> --force                           # -> trash
gwcli drive rm <file-id|url> --force --permanent               # irreversible

# Share / link / audit
gwcli drive share <file-id|url> --email intern@example.com --role writer --notify
gwcli drive share <file-id|url> --type anyone --role reader
gwcli drive link <file-id|url>                                 # enables anyone-with-link, prints URL
gwcli drive link <file-id|url> --no-anyone                     # just print existing webViewLink
gwcli drive permissions <file-id|url>                          # audit who can access
```

`drive export` reuses `fetchDriveArtifact` (same native-export vs.
blob-download branch, folder rejection, and `wrapDriveErr` auth mapping as
`artifacts download`): Docs → `.md`, Sheets → `.csv`, Slides/unknown → PDF,
Drawings → PNG; uploaded files via `Files.Get(alt=media)`. `resolveDriveRef`
runs the argument through `parseDriveURL`; a non-URL argument is treated as a
raw file ID. Output conventions mirror `artifacts`/`attachments` (`--output`,
`--output-dir` default `~/Downloads`, exit codes 2/3). `drive get` does not
download content but still needs the Drive scope (`Files.Get` metadata).

`--export-format` overrides the per-type default for native docs:
`resolveExportFormat` (`artifacts.go`) accepts a friendly alias
(`pdf`/`md`/`docx`/`xlsx`/`csv`/`png`/...) or a raw MIME type. `drive list`
takes a raw Drive `q` expression plus an optional `--folder` (adds a
`'<id>' in parents` clause); `drive search <term>` wraps a term into
`name contains / fullText contains`. Both paginate via `Files.List` with
`SupportsAllDrives`/`IncludeItemsFromAllDrives`/`Corpora("allDrives")` and a
`--limit` cap (0 = no cap). `drive upload`/`drive update` are `Files.Create`/
`Files.Update` media uploads (write ops are intentionally unblocked because
#40 took the full `drive` scope, not `drive.readonly`).

**Write/organize verbs** (`drive_write.go`). The shared `writeDriveFile`
helper makes every write composable: `--json` always returns
`id`/`name`/`mimeType`/`size`/`webViewLink`/`parents` so the next step
(share, move, link) can chain without a search round-trip. `driveWriteFields`
is the field mask requested on every write call.

- `drive mkdir <name> [--folder <parent>] [--no-dedupe]` — `Files.Create`
  with `mimeType=application/vnd.google-apps.folder`. **Idempotent by
  default**: an existing non-trashed folder of the same name in the same
  parent is reused (`ensureDriveFolder` → `findDriveChildByName`); pass
  `--no-dedupe` to force a new folder. `--folder` accepts an ID or URL.
- `drive mv <file> --folder <dest>` — `Files.Update` with
  `addParents=<dest>` / `removeParents=<current parents>` (current parents
  read first via `Files.Get`).
- `drive rename <file> <name>` — `Files.Update` name only (no media).
- `drive cp <file> [--name] [--folder]` — `Files.Copy` (template
  instantiation).
- `drive rm <file> --force [--permanent]` — trash via
  `Files.Update(Trashed:true)` (reversible) by default; `--permanent` is
  `Files.Delete` (irreversible). **`--force` is required** (non-interactive
  rule #3).
- `drive share <file> --type --role [--email|--domain] [--notify]
  [--message]` — `Permissions.Create`. `--type` user/group needs `--email`,
  domain needs `--domain`, `anyone` needs no principal. `role=owner` with
  `type=user` sets `TransferOwnership(true)`. `--message` only sent when
  `--notify`.
- `drive link <file> [--role] [--no-anyone]` — ensures an
  `anyone`/`<role>` permission then prints `webViewLink` (paste into an
  email/message). `--no-anyone` skips the permission change and just prints
  the existing link.
- `drive permissions <file>` — paginated `Permissions.List` (audit who can
  see a doc before sharing internal notes).

**Idempotency / safe reruns.** `drive upload` takes one or more paths (shell
globs like `*.csv` expand to multiple args) and directories. A directory is
walked recursively and mirrored into Drive, creating folders via the same
idempotent `ensureDriveFolder`. `--convert` converts by local extension to
the native Google type (`driveConvertByExt`: csv/tsv/xls*→Sheet,
txt/md/doc*/html→Doc, ppt*→Slides). `--upsert` replaces an existing
same-name file in the destination (via `findDriveChildByName` →
`Files.Update` media) instead of creating `name (1)`, `name (2)` on every
rerun. `--name` is single-file only.

**Folder export.** `drive export <folder>` recurses the folder tree
(`exportDriveFolder`), exporting every file into a local directory mirroring
the Drive structure (native docs honor `--export-format`; binaries are
blob-downloaded). Single-file export behavior is unchanged.

**Export size cap:** Google Docs/Sheets/Slides `Files.Export` has a ~10 MB
limit. `wrapDriveExportErr` (`artifacts.go`) detects the
`exportSizeLimitExceeded` 403 and returns an actionable message suggesting
`--export-format pdf` (server-side PDF export is not subject to the same
cap). It falls through to `wrapDriveErr` for auth failures.

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

## Google Calendar Commands

gwcli supports Google Calendar API for managing calendars and events.

### Calendars

```bash
# List all accessible calendars
gwcli calendars list
gwcli calendars list --json
gwcli calendars list --min-access-role owner
```

### Events

```bash
# List upcoming events (default: primary calendar)
gwcli events list
gwcli events list work@group.calendar.google.com

# List events in date range
gwcli events list --time-min "2024-01-01T00:00:00Z" --time-max "2024-01-31T23:59:59Z"

# Search events
gwcli events list --query "meeting"

# Get event details
gwcli events read <event-id>
gwcli events read <calendar-id> <event-id>
gwcli events read <event-id> --json

# Create event with full details
gwcli events create --summary "Team Meeting" --start "2024-01-15T10:00:00Z" --end "2024-01-15T11:00:00Z"
gwcli events create --summary "All Day Event" --start "2024-01-15" --all-day
gwcli events create --summary "Meeting" --start "2024-01-15T10:00:00Z" --attendee alice@example.com --attendee bob@example.com

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
gwcli events updated --updated-min "2024-01-01T00:00:00Z"

# Detect scheduling conflicts
gwcli events conflicts
gwcli events conflicts --time-max "2024-02-01T00:00:00Z"

# Import ICS file
gwcli events import --file meeting.ics
gwcli events import --file meeting.ics --dry-run
cat events.ics | gwcli events import --file -
```

### Reminders

Reminder format: `<number>[w|d|h|m] [popup|email]`

Examples:
- `15` or `15m` - 15 minutes before, popup notification
- `1h` - 1 hour before, popup
- `2d popup` - 2 days before, popup
- `1w email` - 1 week before, email

```bash
gwcli events create --summary "Meeting" --start "2024-01-15T10:00:00Z" \
    --reminder "15m popup" --reminder "1h email"
```

### Service Account Usage

For Google Workspace accounts using service accounts:

```bash
# List calendars for a specific user
gwcli --user user@example.com calendars list

# Create event for a user
gwcli --user user@example.com events create --summary "Meeting" --start "2024-01-15T10:00:00Z"
```

Note: Service accounts require domain-wide delegation with the `https://www.googleapis.com/auth/calendar` scope authorized in Google Workspace Admin Console.

## Dependencies

- **Kong** (`github.com/alecthomas/kong`) - CLI parsing (not Cobra)
- **Google APIs** - Gmail API, Tasks API, Calendar API
- **golang.org/x/oauth2** - OAuth2 authentication
- **logrus** - Logging
- **html-to-markdown/v2** (`github.com/JohannesKaufmann/html-to-markdown/v2`) - HTML to Markdown conversion
- **yaml.v3** (`gopkg.in/yaml.v3`) - YAML marshaling for frontmatter
- **go-ical** (`github.com/emersion/go-ical`) - ICS/iCalendar parsing for calendar import

## Common Gotchas

1. **Config path**: gwcli uses `~/.config/gwcli/` directory
2. **Kong syntax**: Command matching uses exact strings like `"messages list"` not path-style routes
3. **No interactive prompts**: All commands must work non-interactively (for scripting). Destructive commands require an explicit `--force` flag instead of prompting (e.g. `filters delete`).
4. **Label IDs vs Names**: Always handle both - users may provide either
5. **Label create/delete**: Done in the Gmail UI; gwcli does not create/delete labels
6. **Labels load from the Gmail API**: Labels are fetched directly from the API (all user labels + system labels). There is no config file and nothing is auto-created.

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
