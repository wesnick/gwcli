# gwcli - Command-line Gmail Client

A command-line interface for Gmail, optimized for non-interactive use, shell scripting, and AI agent integration.

## About This Fork

This is a fork of [cmdg](https://github.com/ThomasHabets/cmdg) by Thomas Habets.

**Key differences from original cmdg:**
- **Removed TUI (Text User Interface)** - cmdg was a Pine/Alpine-like interactive email client
- **Pure CLI tool** - gwcli is designed for shell scripting and piping to other tools
- **No lynx dependency** - HTML emails are output as raw HTML, pipe to `html2md` or similar tools
- **Simplified for automation** - All commands work non-interactively

Original cmdg: Copyright Thomas Habets <thomas@habets.se> 2015-2021

## License

This software is dual-licensed GPL and "Thomas is allowed to release a binary version that adds shared API keys and nothing else" (inherited from original cmdg).

See [LICENSE](LICENSE) for full GPL v2 text.

## Introduction

gwcli provides command-line access to Gmail using the Gmail API. It supports:

- Listing, reading, searching messages
- Sending emails with attachments
- Label management
- Batch operations via stdin
- JSON output for easy parsing

## Why gwcli?

### Benefits over IMAP
* **No passwords on disk** - OAuth2 is used instead. Access can be revoked at [Google Security Settings](https://security.google.com/settings/security/permissions)
* **Native Gmail labels** - No awkward IMAP folder mapping
* **gmailctl compatible** - Works seamlessly with gmailctl label definitions
* **Better security** - Application-specific access, not full account credentials

### Benefits over Gmail web UI
* **Shell scripting** - Pipe, grep, awk, and process emails in scripts
* **AI agent friendly** - Structured output (JSON) for programmatic access
* **Fast** - No browser overhead
* **Works remotely** - SSH-friendly, no graphics needed
* **Composable** - Unix philosophy: do one thing well, pipe to other tools

### Design for AI Agents and Automation

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

1. **Build or install gwcli** – `just build` writes `dist/gwcli`, or `go install .` drops the binary in your `GOBIN`.
2. **Install gmailctl** – `go install github.com/mbrt/gmailctl/cmd/gmailctl@latest` and confirm `gmailctl version` works; gwcli shells out to this binary.
3. **Prepare the config directory** – `mkdir -p ~/.config/gwcli` and copy in `credentials.json`, `token.json` (OAuth only), and `config.jsonnet`. If you already have gmailctl state, symlink or copy it into this path.
4. **Authorize** – For personal Gmail, run `just configure` (wrapper for `gwcli configure`) to generate `token.json`. For Workspace service accounts, skip this and plan to pass `--user you@example.com` on every gwcli invocation.
5. **Sync gmailctl state** – `just gmailctl-download` pulls the current Gmail filters, edit `~/.config/gwcli/config.jsonnet`, and then run `just gmailctl-diff` / `just gmailctl-apply` to preview and push updates.
6. **Use gwcli commands** – e.g., `gwcli messages list`, `gwcli --user admin@example.com labels list`, or `gwcli auth token-info` once authentication succeeds.

## Configuring

gwcli reads all authentication material and gmailctl rules from `~/.config/gwcli/`. Pick the authentication path that matches your account type and place the files called out below.

### OAuth (Desktop flow)

1. Visit the [Google Cloud Console](https://console.developers.google.com/), create (or reuse) a project, and enable the **Gmail API**.
2. Configure the OAuth consent screen and add the scopes gwcli needs:
   - `https://www.googleapis.com/auth/gmail.modify`
   - `https://www.googleapis.com/auth/gmail.settings.basic`
   - `https://www.googleapis.com/auth/gmail.labels`
3. Create an **OAuth Client ID** of type *Desktop app* and download the JSON credentials to `~/.config/gwcli/credentials.json`.
4. Run `gwcli configure` (or `just configure`) to finish the flow. The command prints a URL, prompts for the returned code, and writes `token.json` in the same directory.

Once both files exist you can run every command with your personal Gmail account.

### Service Accounts (Google Workspace)

gwcli can reuse gmailctl’s service-account authenticator to impersonate Workspace users:

1. In the Cloud Console, create a Service Account, enable **Domain-wide Delegation**, and download the JSON key to `~/.config/gwcli/credentials.json`.
2. In the Admin Console (`Security → API controls → Domain-wide delegation`) authorize the client ID from the JSON file with the scopes above.
3. Skip `gwcli configure` (service accounts do not use OAuth tokens). Instead, pass `--user user@example.com` to every gwcli command to select the mailbox:
   ```bash
   gwcli --user ops@example.com messages list --label SRE
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

### Installing gmailctl

The gmailctl wrapper commands require gmailctl to be installed:

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
just gmailctl-download

# 2. Edit config.jsonnet in your favorite editor
vim ~/.config/gwcli/config.jsonnet

# 3. Preview changes before applying
just gmailctl-diff

# 4. Apply the changes to Gmail
just gmailctl-apply

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

# Read a message (plain text preferred)
gwcli messages read <message-id>

# Read message preferring HTML output
gwcli messages read --prefer-html <message-id>
gwcli messages read -H <message-id>

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

## Piping HTML to Markdown

Since gwcli outputs raw HTML (not rendered), you can pipe to converters:

```bash
# Install html2markdown (or similar tool)
go install github.com/suntong/html2md@latest

# Convert HTML emails to markdown
gwcli messages read -H <msg-id> | html2md

# Or use pandoc
gwcli messages read -H <msg-id> | pandoc -f html -t markdown
```

## Differences from Original cmdg

| Feature | cmdg (original) | gwcli (this fork) |
|---------|----------------|---------------------|
| Interactive TUI | ✅ Yes (Pine/Alpine-like) | ❌ Removed |
| Command-line interface | ❌ No | ✅ Yes |
| HTML rendering | Uses lynx | Raw HTML output |
| Scripting friendly | Limited | Designed for it |
| JSON output | N/A | Yes |
| Batch operations | N/A | Yes (via stdin) |

## Contributing

This fork focuses on CLI automation and simplicity. PRs welcome for:
- Bug fixes
- Additional CLI commands
- Better JSON output structures
- Shell completion scripts

Not accepting:
- Re-adding TUI functionality (use original cmdg for that)
- Interactive prompts (breaks scripting)

## Support

For issues specific to gwcli (CLI functionality), open an issue on this repository.

For general Gmail API questions or issues inherited from original cmdg, see the [original cmdg repository](https://github.com/ThomasHabets/cmdg).
