# Output Writer: Consistent Empty List Handling

## Problem

When `--json` flag is set but no results are found, commands output text instead of JSON:
```bash
gwcli messages list --json  # outputs "No messages found" instead of []
```

## Solution

Add `WriteEmptyList(textMessage string)` method to `outputWriter`:

```go
func (o *outputWriter) WriteEmptyList(textMessage string) error {
    if o.json {
        return o.writeJSON([]interface{}{})
    }
    o.writeMessage(textMessage)
    return nil
}
```

## Output Format

**JSON mode:** `[]`

**Text mode:** The provided message (e.g., "No messages found")

## Files to Update

- `output.go` - Add WriteEmptyList method
- `messages.go:71,609` - List and search empty results
- `attachments.go:26` - No attachments
- `events.go:133,590,800,875,948` - Various empty event results
