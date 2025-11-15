package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// outputWriter handles formatted output (text or JSON)
type outputWriter struct {
	json    bool
	noColor bool
	verbose bool
	writer  io.Writer
}

func newOutputWriter(useJSON, noColor, verbose bool) *outputWriter {
	return &outputWriter{
		json:    useJSON,
		noColor: noColor,
		verbose: verbose,
		writer:  os.Stdout,
	}
}

// writeJSON outputs data as JSON
func (o *outputWriter) writeJSON(data interface{}) error {
	encoder := json.NewEncoder(o.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// writeTable outputs tabular data
func (o *outputWriter) writeTable(headers []string, rows [][]string) error {
	w := tabwriter.NewWriter(o.writer, 0, 0, 2, ' ', 0)

	// Write header
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Write rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	return w.Flush()
}

// writeMessage outputs a simple message
func (o *outputWriter) writeMessage(msg string) {
	fmt.Fprintln(o.writer, msg)
}

// writeError outputs an error message to stderr
func (o *outputWriter) writeError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

// writeVerbose outputs a verbose message to stderr if verbose mode is enabled
func (o *outputWriter) writeVerbose(format string, args ...interface{}) {
	if o.verbose {
		fmt.Fprintf(os.Stderr, "VERBOSE: "+format+"\n", args...)
	}
}

// formatDate formats a timestamp for display
func formatDate(timestamp int64) string {
	t := time.UnixMilli(timestamp)
	return t.Format("2006-01-02 15:04")
}

// formatSize formats bytes as human-readable size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
