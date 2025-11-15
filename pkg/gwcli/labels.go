package gwcli

import (
	"google.golang.org/api/gmail/v1"
)

// Label represents a Gmail label
type Label struct {
	ID       string
	Label    string
	Response *gmail.Label
}
