# gwcli - Command-line Gmail, Tasks & Calendar Client

A command-line interface for Gmail, Google Tasks, and Google Calendar, optimized for non-interactive use, shell scripting, and AI agent integration.

## About This Project

gwcli combines functionality from three open-source projects:

| Project | Author | What gwcli uses |
|---------|--------|-----------------|
| [cmdg](https://github.com/ThomasHabets/cmdg) | Thomas Habets | Gmail API client, message handling, and core architecture (TUI removed) |
| [gmailctl](https://github.com/mbrt/gmailctl) | Michele Bertasi | Authentication system, label/filter management, Jsonnet config parsing |
| [gtasks](https://github.com/BRO3886/gtasks) | Siddhartha Varma | Google Tasks API integration patterns |

**Key characteristics:**
- **Pure CLI tool** - Designed for shell scripting and piping to other tools (no TUI)
- **Non-interactive** - All commands work without user interaction
- **Unified authentication** - Single credential set for Gmail, Tasks, and Calendar APIs
- **gmailctl compatible** - Uses same Jsonnet config format for labels and filters

## License

This software is dual-licensed GPL and "Thomas is allowed to release a binary version that adds shared API keys and nothing else" (inherited from original cmdg).

See [LICENSE](LICENSE) for full GPL v2 text.

Additional components:
- gmailctl code: MIT License (Copyright Michele Bertasi)
- gtasks patterns: Apache License 2.0 (Copyright Siddhartha Varma)

## OAuth Scopes

gwcli requires the following OAuth 2.0 scopes. When setting up OAuth credentials in Google Cloud Console or authorizing domain-wide delegation for service accounts, you must enable all five scopes.

### Required Scopes

| Scope | Description | Classification |
|-------|-------------|----------------|
| `https://www.googleapis.com/auth/gmail.modify` | Read/write all messages and threads (except permanent deletion) | Restricted |
| `https://www.googleapis.com/auth/gmail.settings.basic` | Manage basic mail settings | Restricted |
| `https://www.googleapis.com/auth/gmail.labels` | Create, read, update, and delete labels | Non-sensitive |
| `https://www.googleapis.com/auth/tasks` | Create, edit, organize, and delete tasks | Sensitive |
| `https://www.googleapis.com/auth/calendar` | Read/write access to calendars and events | Sensitive |

### Command-to-Scope Matrix

This table shows which scopes are required for each command group:

| Command | `gmail.modify` | `gmail.settings.basic` | `gmail.labels` | `tasks` | `calendar` |
|---------|:--------------:|:----------------------:|:--------------:|:-------:|:----------:|
| **Messages** |
| `messages list` | Required | - | - | - | - |
| `messages read` | Required | - | - | - | - |
| `messages search` | Required | - | - | - | - |
| `messages send` | Required | - | - | - | - |
| `messages delete` | Required | - | - | - | - |
| `messages mark-read` | Required | - | - | - | - |
| `messages mark-unread` | Required | - | - | - | - |
| `messages move` | Required | - | - | - | - |
| **Labels** |
| `labels list` | - | - | Required | - | - |
| `labels apply` | Required | - | Required | - | - |
| `labels remove` | Required | - | Required | - | - |
| **Attachments** |
| `attachments list` | Required | - | - | - | - |
| `attachments download` | Required | - | - | - | - |
| **gmailctl** |
| `gmailctl download` | - | Required | Required | - | - |
| `gmailctl apply` | - | Required | Required | - | - |
| `gmailctl diff` | - | Required | Required | - | - |
| **Task Lists** |
| `tasklists list` | - | - | - | Required | - |
| `tasklists create` | - | - | - | Required | - |
| `tasklists delete` | - | - | - | Required | - |
| **Tasks** |
| `tasks list` | - | - | - | Required | - |
| `tasks read` | - | - | - | Required | - |
| `tasks create` | - | - | - | Required | - |
| `tasks complete` | - | - | - | Required | - |
| `tasks delete` | - | - | - | Required | - |
| **Calendars** |
| `calendars list` | - | - | - | - | Required |
| **Events** |
| `events list` | - | - | - | - | Required |
| `events read` | - | - | - | - | Required |
| `events create` | - | - | - | - | Required |
| `events quickadd` | - | - | - | - | Required |
| `events update` | - | - | - | - | Required |
| `events delete` | - | - | - | - | Required |
| `events search` | - | - | - | - | Required |
| `events updated` | - | - | - | - | Required |
| `events conflicts` | - | - | - | - | Required |
| `events import` | - | - | - | - | Required |
| **Auth** |
| `auth token-info` | - | - | - | - | - |
| `configure` | - | - | - | - | - |

**Note:** The `gmail.modify` scope provides broad message access. Google considers this a "restricted" scope requiring app verification for public distribution. For personal use or within your organization, verification is not required.

## Introduction

gwcli provides command-line access to Gmail, Google Tasks, and Google Calendar using their respective APIs. It supports:

- **Gmail**: Listing, reading, searching, and sending messages with attachments
- **Gmail**: Label management and batch operations via stdin
- **Tasks**: Managing task lists and tasks (create, read, complete, delete)
- **Calendar**: Managing calendars and events (list, create, update, delete, search, conflicts, import)
- **gmailctl**: Native integration for filter/label management
- **Output**: JSON output for easy parsing and automation

## Why gwcli?

gwcli is designed to be easily used by AI agents and shell scripts:

```bash
# Get raw HTML email, convert to markdown
gwcli messages read --prefer-html <msg-id> | html2md

# Get plain text (default)
gwcli messages read <msg-id>

# Get structured JSON output
gwcli --json messages list --label INBOX

# Batch operations via stdin
echo "msg1\nmsg2\nmsg3" | gwcli messages mark-read --stdin

# Search and process
gwcli messages search "from:example.com" --json | jq '.[] | .subject'

# List task lists
gwcli --json tasklists list | jq '.[].title'

# Create a task
gwcli tasks create <tasklist-id> --title "Review code" --due "2025-01-15T00:00:00Z"

# List upcoming calendar events
gwcli events list --json | jq '.[].summary'

# Quick add event with natural language
gwcli events quickadd "Team meeting tomorrow at 3pm"

# Check for scheduling conflicts
gwcli events conflicts
```

## Installing

### Using go install (recommended)

Install directly from GitHub:

```bash
go install github.com/wesnick/gwcli@latest
```

This will install `gwcli` to your `$GOPATH/bin` or `$GOBIN` directory. Make sure this directory is in your `PATH`.

### Building from source

```bash
go build .
sudo cp gwcli /usr/local/bin
```

Or use the justfile:

```bash
just build
sudo cp dist/gwcli /usr/local/bin
```

## Setup Protocol

1. **Build or install gwcli** – `just build` writes `dist/gwcli`, or `go install github.com/wesnick/gwcli@latest` drops the binary in your `GOBIN`.
2. **Prepare the config directory** – `mkdir -p ~/.config/gwcli` and copy in `credentials.json`. If you already have gmailctl state, symlink or copy `config.jsonnet` into this path.
3. **Authorize** – For personal Gmail, run `gwcli configure` to generate `token.json`. For Workspace service accounts, skip this and use `--user you@example.com` on every invocation.
4. **Sync filter config** – `gwcli gmailctl download` pulls the current Gmail filters to `config.jsonnet`. Edit as needed, then run `gwcli gmailctl diff` / `gwcli gmailctl apply` to preview and push updates.
5. **Use gwcli commands** – e.g., `gwcli messages list`, `gwcli tasklists list`, `gwcli --user admin@example.com labels list`.

**Note:** gmailctl installation is optional. gwcli has native gmailctl integration for download/apply/diff commands. Install gmailctl separately only if you need its advanced commands like `edit`, `test`, or `debug`.

## Configuring

gwcli reads all authentication material and gmailctl rules from `~/.config/gwcli/`. Pick the authentication path that matches your account type and place the files called out below.

### OAuth (Desktop flow)

1. Visit the [Google Cloud Console](https://console.developers.google.com/), create (or reuse) a project, and enable the **Gmail API**, **Google Tasks API**, and **Google Calendar API**.
2. Configure the OAuth consent screen and add the scopes gwcli needs (see [OAuth Scopes](#oauth-scopes) for details):
   - `https://www.googleapis.com/auth/gmail.modify`
   - `https://www.googleapis.com/auth/gmail.settings.basic`
   - `https://www.googleapis.com/auth/gmail.labels`
   - `https://www.googleapis.com/auth/tasks`
   - `https://www.googleapis.com/auth/calendar`
3. Create an **OAuth Client ID** of type *Desktop app* and download the JSON credentials to `~/.config/gwcli/credentials.json`.
4. Run `gwcli configure` (or `just configure`) to finish the flow. The command prints a URL, prompts for the returned code, and writes `token.json` in the same directory.

Once both files exist you can run every command with your personal Gmail account.

### Service Accounts (Google Workspace)

gwcli can reuse gmailctl's service-account authenticator to impersonate Workspace users:

1. In the Cloud Console, enable **Gmail API**, **Google Tasks API**, and **Google Calendar API**, create a Service Account, enable **Domain-wide Delegation**, and download the JSON key to `~/.config/gwcli/credentials.json`.
2. In the Admin Console (`Security → API controls → Domain-wide delegation`) authorize the client ID from the JSON file with all five scopes:
   - `https://www.googleapis.com/auth/gmail.modify`
   - `https://www.googleapis.com/auth/gmail.settings.basic`
   - `https://www.googleapis.com/auth/gmail.labels`
   - `https://www.googleapis.com/auth/tasks`
   - `https://www.googleapis.com/auth/calendar`
3. Skip `gwcli configure` (service accounts do not use OAuth tokens). Instead, pass `--user user@example.com` to every gwcli command to select the mailbox:
   ```bash
   gwcli --user ops@example.com messages list --label SRE
   gwcli --user ops@example.com tasklists list
   gwcli --user ops@example.com events list
   ```
4. Rotate the service-account key the same way you would for gmailctl; gwcli simply streams the file on every invocation.

### Configuration Files

gwcli stores configuration in `~/.config/gwcli/`:
- `credentials.json` – OAuth or service-account credentials (you provide this)
- `token.json` – OAuth access/refresh tokens (auto-generated during `gwcli configure`)
- `config.jsonnet` – gmailctl label/filter definitions that gwcli consumes at startup

**Note:** gwcli embeds gmailctl’s config reader, so the same Jsonnet file powers both tools.

## gmailctl Integration

gwcli vendors gmailctl's config reader and authentication helpers so both tools operate on the same Jsonnet file and credential set. The CLI refuses to start label-aware commands unless `~/.config/gwcli/config.jsonnet` exists, ensuring every run honors the labels and filters checked into gmailctl.

### Configuration File

gwcli reads label definitions from `~/.config/gwcli/config.jsonnet` (Jsonnet format, same as gmailctl). This file defines:
- **Labels**: Custom labels you've created
- **Rules**: Filter rules for automatic email processing (applied via gmailctl)

Example `config.jsonnet`:

```jsonnet
{
  version: "v1alpha3",
  labels: [
    { name: "work" },
    { name: "personal" },
    { name: "receipts" },
  ],
  rules: [
    {
      filter: { from: "example.com" },
      actions: { labels: ["work"] }
    }
  ]
}
```

### Using gmailctl Commands

gwcli provides built-in wrappers for the most common gmailctl operations. These commands automatically use gwcli's config directory (`~/.config/gwcli`), so you don't need to specify `--config` flags.

If you prefer task shortcuts, the `just gmailctl-download`, `just gmailctl-diff`, and `just gmailctl-apply` recipes simply call the corresponding gwcli subcommands.

#### Download existing filters from Gmail

```bash
# Download current Gmail filters to config.jsonnet
gwcli gmailctl download

# Download to a specific file
gwcli gmailctl download -o my-filters.jsonnet
```

#### Apply local config to Gmail

```bash
# Apply config.jsonnet to Gmail (with confirmation prompt)
gwcli gmailctl apply

# Apply without confirmation
gwcli gmailctl apply -y
```

#### Show differences

```bash
# Show diff between local config and Gmail settings
gwcli gmailctl diff
```

### Manual gmailctl Usage

If you prefer to use gmailctl directly (or need access to advanced commands like `edit`, `init`, `test`), run gmailctl with the `--config` flag pointing to gwcli's config directory:

```bash
# Download filters
gmailctl --config ~/.config/gwcli download

# Edit config interactively
gmailctl --config ~/.config/gwcli edit

# Run config tests
gmailctl --config ~/.config/gwcli test

# Show annotated config
gmailctl --config ~/.config/gwcli debug
```

### Installing gmailctl (Optional)

gwcli has native gmailctl integration for the most common operations (download, apply, diff). However, if you need advanced features like interactive editing or config testing, install the gmailctl binary:

```bash
go install github.com/mbrt/gmailctl/cmd/gmailctl@latest
```

After installation, verify it's available:

```bash
gmailctl version
```

### Workflow Example

Typical workflow for managing labels and filters:

```bash
# 1. Download your current Gmail filters
gwcli gmailctl download

# 2. Edit config.jsonnet in your favorite editor
vim ~/.config/gwcli/config.jsonnet

# 3. Preview changes before applying
gwcli gmailctl diff

# 4. Apply the changes to Gmail
gwcli gmailctl apply

# 5. Use the labels in gwcli
gwcli messages list --label work
gwcli labels list
```

## Usage Examples

### Reading Messages

```bash
# List inbox messages
gwcli messages list

# List unread messages only
gwcli messages list --unread-only

# Read a message (default: markdown output)
gwcli messages read <message-id>

# Read message with raw HTML output
gwcli messages read --raw-html <message-id>

# Prefer plain text body over HTML
gwcli messages read --prefer-plain <message-id>

# Get just headers
gwcli messages read --headers-only <message-id>

# Get raw RFC822 format
gwcli messages read --raw <message-id>

# JSON output
gwcli --json messages read <message-id>
```

### Searching

```bash
# Search messages
gwcli messages search "from:example.com subject:important"

# Limit results
gwcli messages search "is:unread" --limit 10
```

### Sending

```bash
# Send email (body from stdin)
echo "Email body" | gwcli messages send \
  --to recipient@example.com \
  --subject "Hello"

# Send with attachments
gwcli messages send \
  --to recipient@example.com \
  --subject "Files attached" \
  --body "See attached files" \
  --attach file1.pdf \
  --attach file2.jpg
```

### Labels

```bash
# List all labels
gwcli labels list

# Apply label to message
gwcli labels apply "MyLabel" --message <message-id>

# Batch apply label (via stdin)
echo "msg1\nmsg2\nmsg3" | gwcli labels apply "Archive" --stdin

# Remove a label from a message
gwcli labels remove "Archive" --message <message-id>
```

**Note:** Label and filter definitions come from `config.jsonnet`. Create, rename, or delete labels through gmailctl (`just gmailctl-download` → edit → `just gmailctl-apply`) so gwcli stays in sync.

### Attachments

```bash
# List attachments in a message (shows index for easy selection)
gwcli attachments list <message-id>

# Download all attachments (defaults to ~/Downloads)
gwcli attachments download <message-id>

# Download specific attachment by index
gwcli attachments download <message-id> --index 0

# Download multiple attachments
gwcli attachments download <message-id> --index 0,1,2
gwcli attachments download <message-id> -i 0 -i 1

# Download by filename pattern (glob)
gwcli attachments download <message-id> --filename "*.pdf"
gwcli attachments download <message-id> -f "invoice*.xlsx"

# Download to specific directory
gwcli attachments download <message-id> --output-dir ./attachments

# Download single attachment to specific file
gwcli attachments download <message-id> --index 0 --output myfile.pdf
```

**Note:** Attachments are automatically numbered (index: 0, 1, 2...) when viewing messages. Use the index for reliable selection. Filename conflicts are handled automatically with ` (n)` suffix.

### Task Lists

```bash
# List all task lists
gwcli tasklists list

# Create a new task list
gwcli tasklists create "Work Projects"

# Delete a task list
gwcli tasklists delete <tasklist-id> --force
```

### Tasks

```bash
# List tasks in a task list
gwcli tasks list <tasklist-id>

# Include completed tasks
gwcli tasks list <tasklist-id> --include-completed

# Create a new task
gwcli tasks create <tasklist-id> --title "Review PR #42"

# Create task with notes and due date
gwcli tasks create <tasklist-id> \
  --title "Submit report" \
  --notes "Q4 financial summary" \
  --due "2025-01-15T00:00:00Z"

# View task details
gwcli tasks read <tasklist-id> <task-id>

# Mark task as completed
gwcli tasks complete <tasklist-id> <task-id>

# Delete a task
gwcli tasks delete <tasklist-id> <task-id> --force
```

### Calendars

```bash
# List all accessible calendars
gwcli calendars list

# List calendars with JSON output
gwcli --json calendars list

# Filter by minimum access role
gwcli calendars list --min-access-role owner
```

### Events

```bash
# List upcoming events (default: primary calendar)
gwcli events list

# List events from a specific calendar
gwcli events list work@group.calendar.google.com

# List events in a date range
gwcli events list --time-min "2025-01-01T00:00:00Z" --time-max "2025-01-31T23:59:59Z"

# Search events by text
gwcli events list --query "meeting"

# Get event details
gwcli events read <event-id>
gwcli events read <calendar-id> <event-id>

# Create event with full details
gwcli events create --summary "Team Meeting" \
  --start "2025-01-15T10:00:00Z" \
  --end "2025-01-15T11:00:00Z" \
  --location "Conference Room A"

# Create all-day event
gwcli events create --summary "Company Holiday" --start "2025-01-20" --all-day

# Create event with attendees and reminders
gwcli events create --summary "Planning Session" \
  --start "2025-01-15T14:00:00Z" \
  --attendee alice@example.com \
  --attendee bob@example.com \
  --reminder "15m popup" \
  --reminder "1h email"

# Quick add with natural language
gwcli events quickadd "Lunch with Bob tomorrow at noon"
gwcli events quickadd "Team standup every Monday at 9am"

# Update an event
gwcli events update <event-id> --summary "Updated Title"
gwcli events update <event-id> --location "Room B"

# Delete an event
gwcli events delete <event-id>
gwcli events delete <event-id> --force

# Search across multiple calendars
gwcli events search "review" --calendar primary --calendar work@group.calendar.google.com

# Find recently updated events
gwcli events updated --updated-min "2025-01-01T00:00:00Z"

# Detect scheduling conflicts
gwcli events conflicts
gwcli events conflicts --time-max "2025-02-01T00:00:00Z"

# Import events from ICS file
gwcli events import --file meeting.ics
gwcli events import --file meeting.ics --dry-run
cat events.ics | gwcli events import --file -
```

**Reminder format:** `<number>[w|d|h|m] [popup|email]`
- `15` or `15m` - 15 minutes before, popup notification
- `1h` - 1 hour before, popup
- `2d popup` - 2 days before, popup
- `1w email` - 1 week before, email

### Batch Operations

```bash
# Mark multiple messages as read
cat message-ids.txt | gwcli messages mark-read --stdin

# Delete multiple messages
gwcli messages search "older_than:1y" --json | \
  jq -r '.[].id' | \
  gwcli messages delete --stdin --force
```

## JSON Output

All commands support `--json` flag for structured output:

```bash
gwcli --json messages list | jq '.[0]'
{
  "id": "18f4a2b3c5d6e7f8",
  "threadId": "18f4a2b3c5d6e7f8",
  "labels": ["INBOX", "UNREAD"],
  "date": "Jan 02",
  "from": "sender@example.com",
  "subject": "Example Subject",
  "snippet": "Email preview text..."
}
```

## Output Formats

By default, `gwcli messages read` converts HTML email bodies to markdown automatically (using the html-to-markdown library). No external tools are needed.

| Flag | Output Format |
|------|---------------|
| (default) | Markdown with YAML frontmatter |
| `--raw-html` | Raw HTML with HTML-formatted headers |
| `--prefer-plain` | Plain text with YAML frontmatter |
| `--raw` | Raw RFC822 format |

```bash
# Default: HTML converted to markdown
gwcli messages read <msg-id>

# Raw HTML (if you need the original HTML)
gwcli messages read --raw-html <msg-id>

# Plain text only
gwcli messages read --prefer-plain <msg-id>
```

## Comparison with Source Projects

gwcli combines and extends functionality from three projects:

### vs cmdg

| Feature | cmdg | gwcli |
|---------|------|-------|
| Interactive TUI | Yes (Pine/Alpine-like) | Removed |
| Command-line interface | No | Yes |
| HTML rendering | Uses lynx | Raw HTML/Markdown output |
| Scripting friendly | Limited | Designed for it |
| JSON output | No | Yes |
| Batch operations | No | Yes (via stdin) |

### vs gmailctl

| Feature | gmailctl | gwcli |
|---------|----------|-------|
| Filter management | Primary focus | Integrated (download/apply/diff) |
| Message operations | No | Yes (list, read, search, send) |
| Label operations | Create/delete via config | Apply/remove + config sync |
| Config format | Jsonnet | Same (compatible) |
| Authentication | OAuth + Service Account | Same (shared code) |

### vs gtasks

| Feature | gtasks | gwcli |
|---------|--------|-------|
| Task lists | Yes | Yes |
| Tasks CRUD | Yes | Yes |
| Gmail integration | No | Yes |
| Unified credentials | N/A | Yes (single config dir) |
| Service account support | No | Yes |

### Calendar Features

gwcli provides comprehensive Google Calendar support:

| Feature | Description |
|---------|-------------|
| Calendar listing | List all accessible calendars with access roles |
| Event CRUD | Create, read, update, delete events |
| Quick add | Natural language event creation ("Lunch tomorrow at noon") |
| Search | Full-text search across calendars |
| Conflict detection | Find overlapping events in a time range |
| ICS import | Import events from iCalendar files |
| Reminders | Configurable popup/email reminders |
| Attendees | Add attendees when creating events |
| All-day events | Create and manage all-day events |
| Service accounts | Full Workspace delegation support |

## Contributing

This fork focuses on CLI automation and simplicity. PRs welcome for:
- Bug fixes
- Additional CLI commands
- Better JSON output structures
- Shell completion scripts


## Support

For issues specific to gwcli (CLI functionality), open an issue on this repository.
