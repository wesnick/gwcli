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

### Building from source

```bash
go build ./cmd/gwcli
sudo cp gwcli /usr/local/bin
```

## Configuring

You need to configure `gwcli` to provide OAuth2 authentication to Gmail:

1. Go to the [Google Developers Console](https://console.developers.google.com/apis)
2. Select an existing project or create a new project
3. Enable the Gmail API:
   - Gmail API: `https://console.developers.google.com/apis/api/gmail.googleapis.com/overview`
4. Navigate to "OAuth consent screen" and fill it out
5. Add scopes (or manually add these URLs):
   - `https://www.googleapis.com/auth/gmail.modify`
   - `https://www.googleapis.com/auth/gmail.settings.basic`
   - `https://www.googleapis.com/auth/gmail.labels`
6. Navigate to "Credentials" page
7. Click "+ CREATE CREDENTIALS"
8. Select "OAuth client ID"
9. Set "Application type" to "Desktop app"
10. Click "CREATE" and copy the Client ID and Client Secret

Then run:

```bash
gwcli configure
# Enter Client ID and Client Secret when prompted
# Follow the browser authentication flow
```

This creates `~/.config/gwcli/` directory with OAuth credentials.

**Note:** gwcli uses gmailctl-compatible OAuth scopes, so it can coexist with gmailctl installations and share label definitions.

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

# Create a label
gwcli labels create "MyLabel"

# Apply label to message
gwcli labels apply "MyLabel" --message <message-id>

# Batch apply label (via stdin)
echo "msg1\nmsg2\nmsg3" | gwcli labels apply "Archive" --stdin
```

### Attachments

```bash
# List attachments in a message
gwcli attachments list <message-id>

# Download all attachments
gwcli attachments download <message-id>

# Download specific attachment
gwcli attachments download <message-id> --attachment-id <att-id>

# Download to specific file
gwcli attachments download <message-id> --output myfile.pdf
```

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
