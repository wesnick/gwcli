#!/bin/bash
# extract-invoices.sh
#
# Download all attachments from messages with a specific label.
# Useful for extracting invoices, receipts, or any other categorized attachments.

set -e

# Configuration
LABEL="${INVOICE_LABEL:-Invoices}"
OUTPUT_DIR="${OUTPUT_DIR:-./invoices/$(date +%Y-%m)}"
APPLY_PROCESSED_LABEL=true
PROCESSED_LABEL="Processed/Invoices"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --label)
      LABEL="$2"
      shift 2
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --no-label)
      APPLY_PROCESSED_LABEL=false
      shift
      ;;
    --processed-label)
      PROCESSED_LABEL="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [OPTIONS]"
      echo ""
      echo "Download attachments from messages with a specific label."
      echo ""
      echo "Options:"
      echo "  --label LABEL          Label to process (default: Invoices)"
      echo "  --output-dir DIR       Output directory (default: ./invoices/YYYY-MM)"
      echo "  --no-label             Don't apply 'Processed' label after extraction"
      echo "  --processed-label NAME Processed label name (default: Processed/Invoices)"
      echo "  --help                 Show this help message"
      echo ""
      echo "Environment variables:"
      echo "  INVOICE_LABEL          Alternative to --label"
      echo "  OUTPUT_DIR             Alternative to --output-dir"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Run with --help for usage"
      exit 1
      ;;
  esac
done

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo "Invoice Extraction Script"
echo "========================="
echo ""
echo "Label: $LABEL"
echo "Output directory: $OUTPUT_DIR"
echo ""

# Create processed label if needed
if [ "$APPLY_PROCESSED_LABEL" = true ]; then
  gmailcli labels create "$PROCESSED_LABEL" 2>/dev/null || true
fi

# Get message IDs
echo "Fetching messages with label '$LABEL'..."
MESSAGE_IDS=$(gmailcli messages list --label "$LABEL" --json 2>/dev/null | jq -r '.[].id')

if [ -z "$MESSAGE_IDS" ]; then
  echo "No messages found with label '$LABEL'"
  exit 0
fi

MESSAGE_COUNT=$(echo "$MESSAGE_IDS" | wc -l)
echo "Found $MESSAGE_COUNT messages"
echo ""

# Process each message
CURRENT=0
SUCCESSFUL=0
FAILED=0

while IFS= read -r message_id; do
  CURRENT=$((CURRENT + 1))
  echo "[$CURRENT/$MESSAGE_COUNT] Processing message: $message_id"

  # Check if message has attachments
  ATTACHMENT_COUNT=$(gmailcli attachments list "$message_id" --json 2>/dev/null | jq 'length')

  if [ "$ATTACHMENT_COUNT" -eq 0 ]; then
    echo "  No attachments found"
    continue
  fi

  echo "  Found $ATTACHMENT_COUNT attachment(s)"

  # Download attachments
  if gmailcli attachments download "$message_id" --output-dir "$OUTPUT_DIR" 2>&1; then
    echo "  Downloaded successfully"
    SUCCESSFUL=$((SUCCESSFUL + 1))

    # Apply processed label
    if [ "$APPLY_PROCESSED_LABEL" = true ]; then
      echo "$message_id" | gmailcli labels apply "$PROCESSED_LABEL" --stdin 2>/dev/null || true
    fi
  else
    echo "  Failed to download"
    FAILED=$((FAILED + 1))
  fi

  echo ""
done <<< "$MESSAGE_IDS"

echo "Extraction complete!"
echo "===================="
echo "Total messages: $MESSAGE_COUNT"
echo "Successfully processed: $SUCCESSFUL"
echo "Failed: $FAILED"
echo ""
echo "Downloaded to: $OUTPUT_DIR"
