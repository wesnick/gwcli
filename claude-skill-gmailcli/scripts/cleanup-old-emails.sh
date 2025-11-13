#!/bin/bash
# cleanup-old-emails.sh
#
# Periodically clean up old emails from various categories to keep inbox manageable.
# Customize the categories and age thresholds as needed.

set -e

# Configuration
PROMOTIONS_AGE="90d"
SOCIAL_AGE="60d"
UPDATES_AGE="120d"
DRY_RUN=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --promotions-age)
      PROMOTIONS_AGE="$2"
      shift 2
      ;;
    --social-age)
      SOCIAL_AGE="$2"
      shift 2
      ;;
    --updates-age)
      UPDATES_AGE="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --dry-run              Show what would be deleted without deleting"
      echo "  --promotions-age AGE   Age threshold for promotions (default: 90d)"
      echo "  --social-age AGE       Age threshold for social emails (default: 60d)"
      echo "  --updates-age AGE      Age threshold for updates (default: 120d)"
      echo "  --help                 Show this help message"
      echo ""
      echo "Age format: Nd (e.g., 30d, 60d, 90d)"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Run with --help for usage"
      exit 1
      ;;
  esac
done

# Function to delete old emails from a category
delete_category() {
  local category="$1"
  local age="$2"
  local query="category:${category} older_than:${age}"

  echo "Processing: ${category} (older than ${age})"

  # Get message IDs
  local ids=$(gmailcli messages search "$query" --json 2>/dev/null | jq -r '.[].id')

  if [ -z "$ids" ]; then
    echo "  No messages found"
    return
  fi

  local count=$(echo "$ids" | wc -l)
  echo "  Found $count messages"

  if [ "$DRY_RUN" = true ]; then
    echo "  [DRY RUN] Would delete $count messages"
  else
    echo "  Deleting..."
    echo "$ids" | gmailcli messages delete --stdin --force
    echo "  Deleted $count messages"
  fi

  echo ""
}

# Main execution
echo "Gmail Cleanup Script"
echo "===================="
echo ""

if [ "$DRY_RUN" = true ]; then
  echo "** DRY RUN MODE - No messages will be deleted **"
  echo ""
fi

# Delete old promotions
delete_category "promotions" "$PROMOTIONS_AGE"

# Delete old social emails
delete_category "social" "$SOCIAL_AGE"

# Delete old updates
delete_category "updates" "$UPDATES_AGE"

echo "Cleanup complete!"
