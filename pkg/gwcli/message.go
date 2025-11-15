package gwcli

import (
	"google.golang.org/api/gmail/v1"
)

// Message represents a Gmail message with cached data
type Message struct {
	ID       string
	ThreadID string
	Response *gmail.Message
}
