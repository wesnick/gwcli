---
name: gmailcli
description: This skill should be used when working with Gmail operations via the gmailcli command-line tool. Use this skill when the user asks to interact with Gmail (read/send/search emails, manage labels, download attachments), automate email workflows, or needs help with gmailcli commands.
---

# gmailcli

## Overview

This skill provides comprehensive guidance for using gmailcli, a command-line Gmail client with a resource-oriented interface (similar to kubectl). It enables automation of Gmail operations including message management, label operations, and attachment handling through a Unix-friendly CLI.

## Core Capabilities

gmailcli provides three main resource types:

1. **Messages** - Read, send, search, delete, mark read/unread, and move emails
2. **Labels** - List, create, delete, apply, and remove Gmail labels
3. **Attachments** - List and download email attachments

## When to Use This Skill

Use this skill when the user:
- Asks to perform Gmail operations via command line
- Needs to automate email workflows (e.g., "archive all read messages", "download invoices")
- Requests batch operations on multiple emails
- Wants to search, filter, or process Gmail messages programmatically
- Needs to manage labels or organize emails
- Wants to download attachments in bulk

## Quick Start

### Initial Setup

Before using gmailcli, configure OAuth authentication:

```bash
gmailcli configure
```

This opens a browser for Google OAuth and saves credentials to `~/.cmdg/cmdg.conf`. The tool shares authentication with cmdg TUI if already configured.

### Basic Command Pattern

gmailcli follows a resource-oriented structure:

```bash
gmailcli <resource> <action> [arguments] [flags]
```

**Example:**
```bash
gmailcli messages list --unread-only
gmailcli labels create "Work/Projects"
gmailcli attachments download <message-id>
```

## Common Workflows

### Reading and Searching Email

**List messages:**
```bash
# List inbox
gmailcli messages list

# List specific label
gmailcli messages list --label "Work"

# Unread only
gmailcli messages list --unread-only

# Output as JSON for processing
gmailcli messages list --json
```

**Search with Gmail query syntax:**
```bash
# Search by sender
gmailcli messages search "from:user@example.com"

# Complex queries
gmailcli messages search "subject:urgent is:unread has:attachment"

# Date-based searches
gmailcli messages search "after:2025/10/01 before:2025/11/01"
gmailcli messages search "older_than:30d category:promotions"
```

**Read individual messages:**
```bash
# Read full message
gmailcli messages read <message-id>

# Headers only
gmailcli messages read <message-id> --headers-only

# Raw RFC822 format
gmailcli messages read <message-id> --raw

# JSON output
gmailcli messages read <message-id> --json
```

### Sending Email

**Send simple email:**
```bash
gmailcli messages send \
  --to user@example.com \
  --subject "Hello" \
  --body "Message text"
```

**Send with attachments:**
```bash
gmailcli messages send \
  --to user@example.com \
  --subject "Documents" \
  --attach report.pdf \
  --attach data.xlsx \
  --body "Please review"
```

**Multiple recipients:**
```bash
gmailcli messages send \
  --to user1@example.com \
  --to user2@example.com \
  --cc manager@example.com \
  --bcc admin@example.com \
  --subject "Team Update" \
  --body "Update text"
```

**Body from stdin:**
```bash
cat message.txt | gmailcli messages send \
  --to user@example.com \
  --subject "Report"
```

### Batch Operations

gmailcli supports `--stdin` for batch processing. Common pattern:

```bash
gmailcli messages search "<query>" --json | \
  jq -r '.[].id' | \
  gmailcli messages <operation> --stdin [flags]
```

**Archive read messages:**
```bash
gmailcli messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gmailcli messages move --stdin --to "Archive"
```

**Delete old promotional emails:**
```bash
gmailcli messages search "category:promotions older_than:30d" --json | \
  jq -r '.[].id' | \
  gmailcli messages delete --stdin --force
```

**Apply label to search results:**
```bash
gmailcli messages search "from:vip@company.com is:unread" --json | \
  jq -r '.[].id' | \
  gmailcli labels apply "VIP-Unread" --stdin
```

**Bulk mark as read:**
```bash
gmailcli messages list --unread-only --json | \
  jq -r '.[].id' | \
  gmailcli messages mark-read --stdin
```

### Label Management

**List labels:**
```bash
# All labels
gmailcli labels list

# User-created only
gmailcli labels list --user-only

# System labels only
gmailcli labels list --system

# JSON output
gmailcli labels list --json
```

**Create labels:**
```bash
# Simple label
gmailcli labels create "Important"

# Nested label (uses / separator)
gmailcli labels create "Work/Projects/Q4-2025"

# With color
gmailcli labels create "Urgent" --color "#ff0000"
```

**Apply/remove labels:**
```bash
# Apply to single message
gmailcli labels apply "Important" --message <message-id>

# Remove from single message
gmailcli labels remove "Spam" --message <message-id>

# Batch apply from search
gmailcli messages search "subject:invoice has:attachment" --json | \
  jq -r '.[].id' | \
  gmailcli labels apply "Invoices" --stdin
```

**Delete labels:**
```bash
# Requires --force flag for safety
gmailcli labels delete "Old Project" --force
```

### Attachment Operations

**List attachments:**
```bash
gmailcli attachments list <message-id>
gmailcli attachments list <message-id> --json
```

**Download attachments:**
```bash
# Download all attachments to current directory
gmailcli attachments download <message-id>

# Download to specific directory
gmailcli attachments download <message-id> --output-dir ./downloads

# Download specific attachment
gmailcli attachments download <message-id> \
  --attachment-id <att-id> \
  --output report.pdf
```

**Download all attachments from label:**
```bash
gmailcli messages list --label "Invoices" --json | \
  jq -r '.[].id' | \
  while read id; do
    gmailcli attachments download "$id" --output-dir ./invoices
  done
```

## Global Flags

Available on all commands:

- `--config <path>` - Config file path (default: ~/.cmdg/cmdg.conf)
- `--json` - Output in JSON format for programmatic processing
- `--verbose` - Enable verbose logging for troubleshooting
- `--no-color` - Disable colored output

## Important Behaviors

### Safety Features

**Destructive operations require confirmation:**
- `gmailcli messages delete` requires `--force` flag
- `gmailcli labels delete` requires `--force` flag

This prevents accidental data loss.

### Move Semantics

The `move` command:
- Removes the `INBOX` label
- Adds the target label
- Use for archiving or organizing messages

```bash
# Archive (remove from inbox)
gmailcli messages move <message-id> --to "Archive"

# Move to project folder
gmailcli messages move <message-id> --to "Work/Projects/Q4"
```

### Label Resolution

Labels can be referenced by:
- Name (case-insensitive): `"Important"`, `"work/projects"`
- ID: `Label_123`

Nested labels use `/` separator: `"Work/Projects/Q4"`

### Output Formats

**Text mode (default):**
- Human-readable tables
- Colored output (disable with `--no-color`)
- Good for interactive use

**JSON mode (`--json` flag):**
- Machine-readable structured data
- Essential for piping to `jq` or other tools
- Required for batch processing workflows

### Exit Codes

- `0` - Success
- `1` - User error (invalid arguments)
- `2` - API error
- `3` - Authentication error
- `4` - Not found

Use for error handling in scripts:

```bash
if ! gmailcli messages read "$msg_id" 2>/dev/null; then
  case $? in
    3) echo "Authentication failed" ;;
    4) echo "Message not found" ;;
    *) echo "Error occurred" ;;
  esac
fi
```

## Gmail Query Syntax

Use in `gmailcli messages search "<query>"`:

**Sender/Recipient:**
- `from:user@example.com` - From specific sender
- `to:user@example.com` - To specific recipient

**Content:**
- `subject:keyword` - Subject contains keyword
- `has:attachment` - Has attachments
- `filename:pdf` - Specific attachment type

**Status:**
- `is:unread` - Unread messages
- `is:read` - Read messages
- `is:starred` - Starred messages
- `is:important` - Important messages

**Date:**
- `after:2025/01/01` - After specific date
- `before:2025/12/31` - Before specific date
- `older_than:30d` - Older than 30 days
- `newer_than:7d` - Newer than 7 days

**Categories/Labels:**
- `category:promotions` - In Gmail category
- `category:social` - Social category
- `category:updates` - Updates category
- `label:Important` - Has specific label

**Combine with boolean operators:**
```bash
# AND (implicit)
"from:boss@company.com is:unread"

# OR
"from:alice@example.com OR from:bob@example.com"

# NOT
"from:notifications -subject:merged"
```

## Advanced Patterns

### Conditional Processing

Process messages based on content:

```bash
gmailcli messages list --label "Inbox" --json | \
  jq -r '.[] | select(.from | contains("@company.com")) | .id' | \
  gmailcli labels apply "Internal" --stdin
```

### Periodic Cleanup

Delete old messages by category:

```bash
#!/bin/bash
# cleanup-old-emails.sh

# Delete old promotions (90+ days)
gmailcli messages search "category:promotions older_than:90d" --json | \
  jq -r '.[].id' | \
  gmailcli messages delete --stdin --force

# Delete old social emails (60+ days)
gmailcli messages search "category:social older_than:60d" --json | \
  jq -r '.[].id' | \
  gmailcli messages delete --stdin --force
```

### Attachment Extraction Pipeline

Extract and organize attachments:

```bash
#!/bin/bash
# extract-invoices.sh

LABEL="Invoices"
OUTPUT_DIR="./invoices/$(date +%Y-%m)"

mkdir -p "$OUTPUT_DIR"

gmailcli messages list --label "$LABEL" --json | \
  jq -r '.[].id' | \
  while read id; do
    echo "Processing message: $id"
    gmailcli attachments download "$id" --output-dir "$OUTPUT_DIR"
  done

echo "Downloaded to: $OUTPUT_DIR"
```

### Smart Label Application

Apply labels based on sender domain:

```bash
#!/bin/bash
# auto-label-by-domain.sh

# Get company domains from config
DOMAINS=("company.com" "partner.org" "client.net")

for domain in "${DOMAINS[@]}"; do
  label="Emails/${domain}"

  # Create label if it doesn't exist
  gmailcli labels create "$label" 2>/dev/null || true

  # Apply to recent emails from domain
  gmailcli messages search "from:@${domain} newer_than:7d" --json | \
    jq -r '.[].id' | \
    gmailcli labels apply "$label" --stdin
done
```

### Email Monitoring

Monitor for specific emails:

```bash
#!/bin/bash
# monitor-urgent.sh

QUERY="subject:urgent is:unread"

while true; do
  COUNT=$(gmailcli messages search "$QUERY" --json | jq 'length')

  if [ "$COUNT" -gt 0 ]; then
    echo "[$(date)] Found $COUNT urgent unread messages"
    gmailcli messages search "$QUERY" --json | \
      jq -r '.[] | "\(.from): \(.subject)"'
  fi

  sleep 300  # Check every 5 minutes
done
```

## Working with JSON Output

When using `--json`, pipe through `jq` for filtering:

**Extract specific fields:**
```bash
gmailcli messages list --json | jq '.[] | {id, subject, from}'
```

**Filter by criteria:**
```bash
# Messages from specific domain
gmailcli messages list --json | \
  jq '.[] | select(.from | contains("@company.com"))'

# Messages with attachments (check labels)
gmailcli messages list --json | \
  jq '.[] | select(.labels | contains(["IMPORTANT"]))'

# Unread messages only
gmailcli messages list --json | \
  jq '.[] | select(.labels | contains(["UNREAD"]))'
```

**Count and statistics:**
```bash
# Count unread messages
gmailcli messages list --json | \
  jq '[.[] | select(.labels | contains(["UNREAD"]))] | length'

# Count by label
gmailcli messages list --json | \
  jq 'group_by(.labels[]) | map({label: .[0].labels[0], count: length})'
```

## Resources

### references/commands.md

Complete command reference with detailed documentation of all commands, flags, and options. Use this reference when:
- Looking up specific command syntax
- Understanding flag options
- Finding examples of command usage
- Learning about exit codes and output formats

Load with: `Read references/commands.md`

### scripts/

The scripts directory contains example automation workflows:

- `cleanup-old-emails.sh` - Periodic cleanup of old emails by category
- `extract-invoices.sh` - Batch download attachments from labeled messages
- `auto-label.sh` - Automatically apply labels based on sender rules

These scripts can be:
- Executed directly for automation
- Modified for specific use cases
- Used as templates for custom workflows

## Implementation Guidelines

When helping users with gmailcli:

1. **Always check authentication first** - Ensure `gmailcli configure` has been run
2. **Use JSON for automation** - Add `--json` flag when building pipelines
3. **Include safety flags** - Remind users about `--force` for destructive operations
4. **Leverage stdin for batches** - Use `--stdin` pattern for processing multiple messages
5. **Provide complete commands** - Include all necessary flags in examples
6. **Test incrementally** - Suggest testing commands on small datasets first
7. **Handle errors gracefully** - Include error checking in automation scripts
8. **Use verbose mode for debugging** - Add `--verbose` when troubleshooting

## Common User Requests

### "Archive all read emails"
```bash
gmailcli messages list --json | \
  jq -r '.[] | select(.labels | contains(["UNREAD"]) | not) | .id' | \
  gmailcli messages move --stdin --to "Archive"
```

### "Delete emails from a sender"
```bash
gmailcli messages search "from:unwanted@spam.com" --json | \
  jq -r '.[].id' | \
  gmailcli messages delete --stdin --force
```

### "Download all PDF attachments"
```bash
gmailcli messages search "has:attachment filename:pdf" --json | \
  jq -r '.[].id' | \
  while read id; do
    gmailcli attachments download "$id" --output-dir ./pdfs
  done
```

### "Label emails by project"
```bash
gmailcli messages search "subject:ProjectX" --json | \
  jq -r '.[].id' | \
  gmailcli labels apply "Work/ProjectX" --stdin
```

### "Send email to multiple recipients"
```bash
gmailcli messages send \
  --to person1@example.com \
  --to person2@example.com \
  --to person3@example.com \
  --subject "Team Update" \
  --body "Weekly update text"
```

### "Find and extract invoices"
```bash
# Search for invoice emails
gmailcli messages search "subject:invoice has:attachment" --json | \
  jq -r '.[].id' | \
  while read id; do
    # Download attachments
    gmailcli attachments download "$id" --output-dir ./invoices

    # Label for tracking
    echo "$id" | gmailcli labels apply "Processed" --stdin
  done
```
