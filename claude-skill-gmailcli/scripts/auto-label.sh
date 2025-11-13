#!/bin/bash
# auto-label.sh
#
# Automatically apply labels to messages based on sender domain or other criteria.
# Useful for organizing emails from specific companies, clients, or projects.

set -e

# Default configuration
CONFIG_FILE="${HOME}/.gmailcli-auto-label.conf"
DRY_RUN=false
TIME_WINDOW="7d"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --config)
      CONFIG_FILE="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --time-window)
      TIME_WINDOW="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo "Automatically label messages based on rules in config file."
      echo ""
      echo "Options:"
      echo "  --config FILE       Config file path (default: ~/.gmailcli-auto-label.conf)"
      echo "  --dry-run           Show what would be labeled without applying"
      echo "  --time-window AGE   Only process emails newer than AGE (default: 7d)"
      echo "  --help              Show this help message"
      echo ""
      echo "Config file format (one rule per line):"
      echo "  domain:company.com=Work/Company"
      echo "  from:noreply@github.com=Development/GitHub"
      echo "  subject:invoice=Finance/Invoices"
      echo "  query:is:starred has:attachment=Important/Files"
      echo ""
      echo "Example config file:"
      echo "  # Work emails"
      echo "  domain:acme.com=Work/Acme"
      echo "  domain:partner.org=Work/Partners"
      echo "  "
      echo "  # Automated notifications"
      echo "  from:notifications@github.com=Dev/GitHub"
      echo "  from:alerts@monitoring.io=Ops/Alerts"
      echo "  "
      echo "  # Categorize by content"
      echo "  subject:invoice=Finance/Invoices"
      echo "  subject:receipt=Finance/Receipts"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Run with --help for usage"
      exit 1
      ;;
  esac
done

# Check if config file exists
if [ ! -f "$CONFIG_FILE" ]; then
  echo "Config file not found: $CONFIG_FILE"
  echo ""
  echo "Create a config file with labeling rules. Example:"
  echo ""
  echo "  # Label by domain"
  echo "  domain:company.com=Work/Company"
  echo "  domain:client.net=Clients/ClientName"
  echo "  "
  echo "  # Label by sender"
  echo "  from:noreply@github.com=Development/GitHub"
  echo "  "
  echo "  # Label by subject"
  echo "  subject:invoice=Finance/Invoices"
  echo "  "
  echo "  # Label by complex query"
  echo "  query:is:starred has:attachment=Important/Files"
  echo ""
  exit 1
fi

echo "Auto-Label Script"
echo "================="
echo ""
echo "Config file: $CONFIG_FILE"
echo "Time window: $TIME_WINDOW"

if [ "$DRY_RUN" = true ]; then
  echo "** DRY RUN MODE **"
fi

echo ""

# Read config file and process rules
RULE_COUNT=0
TOTAL_LABELED=0

while IFS= read -r line; do
  # Skip empty lines and comments
  [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue

  # Parse rule: type:criteria=label
  if [[ ! "$line" =~ ^(domain|from|subject|query):(.+)=(.+)$ ]]; then
    echo "Warning: Invalid rule format: $line"
    continue
  fi

  rule_type="${BASH_REMATCH[1]}"
  criteria="${BASH_REMATCH[2]}"
  label="${BASH_REMATCH[3]}"

  RULE_COUNT=$((RULE_COUNT + 1))

  # Build Gmail query based on rule type
  case "$rule_type" in
    domain)
      query="from:@${criteria} newer_than:${TIME_WINDOW}"
      ;;
    from)
      query="from:${criteria} newer_than:${TIME_WINDOW}"
      ;;
    subject)
      query="subject:${criteria} newer_than:${TIME_WINDOW}"
      ;;
    query)
      query="${criteria} newer_than:${TIME_WINDOW}"
      ;;
    *)
      echo "Warning: Unknown rule type: $rule_type"
      continue
      ;;
  esac

  echo "Rule $RULE_COUNT: $rule_type:$criteria → $label"

  # Search for messages
  MESSAGE_IDS=$(gmailcli messages search "$query" --json 2>/dev/null | jq -r '.[].id')

  if [ -z "$MESSAGE_IDS" ]; then
    echo "  No messages found"
    echo ""
    continue
  fi

  MESSAGE_COUNT=$(echo "$MESSAGE_IDS" | wc -l)
  echo "  Found $MESSAGE_COUNT messages"

  if [ "$DRY_RUN" = true ]; then
    echo "  [DRY RUN] Would apply label '$label' to $MESSAGE_COUNT messages"
  else
    # Create label if it doesn't exist
    gmailcli labels create "$label" 2>/dev/null || true

    # Apply label
    echo "  Applying label '$label'..."
    echo "$MESSAGE_IDS" | gmailcli labels apply "$label" --stdin 2>&1 | grep -v "^$" || true
    echo "  Applied to $MESSAGE_COUNT messages"

    TOTAL_LABELED=$((TOTAL_LABELED + MESSAGE_COUNT))
  fi

  echo ""

done < "$CONFIG_FILE"

echo "Auto-labeling complete!"
echo "======================"
echo "Rules processed: $RULE_COUNT"

if [ "$DRY_RUN" = false ]; then
  echo "Total messages labeled: $TOTAL_LABELED"
fi
