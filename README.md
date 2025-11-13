# gmailcli - Command-line Gmail Client

A command-line interface for Gmail, optimized for non-interactive use, shell scripting, and AI agent integration.

## About This Fork

This is a fork of [cmdg](https://github.com/ThomasHabets/cmdg) by Thomas Habets.

**Key differences from original cmdg:**
- **Removed TUI (Text User Interface)** - cmdg was a Pine/Alpine-like interactive email client
- **Pure CLI tool** - gmailcli is designed for shell scripting and piping to other tools
- **No lynx dependency** - HTML emails are output as raw HTML, pipe to `html2md` or similar tools
- **Simplified for automation** - All commands work non-interactively

Original cmdg: Copyright Thomas Habets <thomas@habets.se> 2015-2021

## License

This software is dual-licensed GPL and "Thomas is allowed to release a binary version that adds shared API keys and nothing else" (inherited from original cmdg).

See [LICENSE](LICENSE) for full GPL v2 text.

## Introduction

gmailcli provides command-line access to Gmail using the Gmail API. It supports:

- Listing, reading, searching messages
- Sending emails with attachments
- Label management
- Batch operations via stdin
- JSON output for easy parsing

## Why gmailcli?

### Benefits over IMAP
* **No passwords on disk** - OAuth2 is used instead. Access can be revoked at [Google Security Settings](https://security.google.com/settings/security/permissions)
* **Native Gmail labels** - No awkward IMAP folder mapping
* **Google Contacts integration** - Uses your actual contact list
* **Better security** - Application-specific access, not full account credentials

### Benefits over Gmail web UI
* **Shell scripting** - Pipe, grep, awk, and process emails in scripts
* **AI agent friendly** - Structured output (JSON) for programmatic access
* **Fast** - No browser overhead
* **Works remotely** - SSH-friendly, no graphics needed
* **Composable** - Unix philosophy: do one thing well, pipe to other tools

### Design for AI Agents and Automation

gmailcli is designed to be easily used by AI agents and shell scripts:

```bash
# Get raw HTML email, convert to markdown
gmailcli messages read --prefer-html <msg-id> | html2md

# Get plain text (default)
gmailcli messages read <msg-id>

# Get structured JSON output
gmailcli --json messages list --label INBOX

# Batch operations via stdin
echo "msg1\nmsg2\nmsg3" | gmailcli messages mark-read --stdin

# Search and process
gmailcli messages search "from:example.com" --json | jq '.[] | .subject'
```

## Installing

### Building from source

```bash
go build ./cmd/gmailcli
sudo cp gmailcli /usr/local/bin
```

## Configuring

You need to configure `gmailcli` to provide OAuth2 authentication to Gmail:

1. Go to the [Google Developers Console](https://console.developers.google.com/apis)
2. Select an existing project or create a new project
3. Enable these three APIs:
   - Gmail API: `https://console.developers.google.com/apis/api/gmail.googleapis.com/overview`
   - Google Drive API: `https://console.developers.google.com/apis/api/drive.googleapis.com/overview`
   - People API: `https://console.developers.google.com/apis/api/people.googleapis.com/overview`
4. Navigate to "OAuth consent screen" and fill it out
5. Add scopes (or manually add these URLs):
   - Gmail API: `https://www.googleapis.com/auth/gmail.modify`
   - Google Drive API: `https://www.googleapis.com/auth/drive.appdata`
   - People API: `https://www.googleapis.com/auth/contacts.readonly`
6. Navigate to "Credentials" page
7. Click "+ CREATE CREDENTIALS"
8. Select "OAuth client ID"
9. Set "Application type" to "Desktop app"
10. Click "CREATE" and copy the Client ID and Client Secret

Then run:

```bash
gmailcli configure
# Enter Client ID and Client Secret when prompted
# Follow the browser authentication flow
```

This creates `~/.cmdg/cmdg.conf` (note: still uses the same config path as cmdg).

## Usage Examples

### Reading Messages

```bash
# List inbox messages
gmailcli messages list

# List unread messages only
gmailcli messages list --unread-only

# Read a message (plain text preferred)
gmailcli messages read <message-id>

# Read message preferring HTML output
gmailcli messages read --prefer-html <message-id>
gmailcli messages read -H <message-id>

# Get just headers
gmailcli messages read --headers-only <message-id>

# Get raw RFC822 format
gmailcli messages read --raw <message-id>

# JSON output
gmailcli --json messages read <message-id>
```

### Searching

```bash
# Search messages
gmailcli messages search "from:example.com subject:important"

# Limit results
gmailcli messages search "is:unread" --limit 10
```

### Sending

```bash
# Send email (body from stdin)
echo "Email body" | gmailcli messages send \
  --to recipient@example.com \
  --subject "Hello"

# Send with attachments
gmailcli messages send \
  --to recipient@example.com \
  --subject "Files attached" \
  --body "See attached files" \
  --attach file1.pdf \
  --attach file2.jpg
```

### Labels

```bash
# List all labels
gmailcli labels list

# Create a label
gmailcli labels create "MyLabel"

# Apply label to message
gmailcli labels apply "MyLabel" --message <message-id>

# Batch apply label (via stdin)
echo "msg1\nmsg2\nmsg3" | gmailcli labels apply "Archive" --stdin
```

### Attachments

```bash
# List attachments in a message
gmailcli attachments list <message-id>

# Download all attachments
gmailcli attachments download <message-id>

# Download specific attachment
gmailcli attachments download <message-id> --attachment-id <att-id>

# Download to specific file
gmailcli attachments download <message-id> --output myfile.pdf
```

### Batch Operations

```bash
# Mark multiple messages as read
cat message-ids.txt | gmailcli messages mark-read --stdin

# Delete multiple messages
gmailcli messages search "older_than:1y" --json | \
  jq -r '.[].id' | \
  gmailcli messages delete --stdin --force
```

## JSON Output

All commands support `--json` flag for structured output:

```bash
gmailcli --json messages list | jq '.[0]'
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

Since gmailcli outputs raw HTML (not rendered), you can pipe to converters:

```bash
# Install html2markdown (or similar tool)
go install github.com/suntong/html2md@latest

# Convert HTML emails to markdown
gmailcli messages read -H <msg-id> | html2md

# Or use pandoc
gmailcli messages read -H <msg-id> | pandoc -f html -t markdown
```

## Differences from Original cmdg

| Feature | cmdg (original) | gmailcli (this fork) |
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

For issues specific to gmailcli (CLI functionality), open an issue on this repository.

For general Gmail API questions or issues inherited from original cmdg, see the [original cmdg repository](https://github.com/ThomasHabets/cmdg).
