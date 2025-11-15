package main

import (
	"fmt"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"gopkg.in/yaml.v3"
)

// OutputFormat represents the desired output format for email display
type OutputFormat int

const (
	FormatMarkdown OutputFormat = iota
	FormatHTML
	FormatPlainText
)

// EmailFrontmatter represents the YAML frontmatter for email output
type EmailFrontmatter struct {
	MessageID string   `yaml:"message_id"`
	ThreadID  string   `yaml:"thread_id"`
	From      string   `yaml:"from"`
	To        string   `yaml:"to"`
	Cc        string   `yaml:"cc,omitempty"`
	Subject   string   `yaml:"subject"`
	Date      string   `yaml:"date"`
	Labels    []string `yaml:"labels,omitempty"`
	Note      string   `yaml:"note,omitempty"` // For fallback messages
}

// AttachmentMeta represents attachment metadata in YAML format
type AttachmentMeta struct {
	Index    int    `yaml:"index"`
	Filename string `yaml:"filename"`
	MimeType string `yaml:"mime_type"`
	Size     int64  `yaml:"size"`
}

// formatSizeBytes converts bytes to human-readable format
func formatSizeBytes(bytes int64) string {
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

// convertHTMLToMarkdown converts HTML email body to markdown
// Uses preserve-more strategy: keeps image URLs, table structure, emphasis
func convertHTMLToMarkdown(htmlBody string) (string, error) {
	// Use v2 simple API for direct conversion
	markdown, err := md.ConvertString(htmlBody)
	if err != nil {
		return "", fmt.Errorf("failed to convert HTML to markdown: %w", err)
	}

	// Clean up excessive newlines that might result from conversion
	markdown = strings.TrimSpace(markdown)

	return markdown, nil
}

// formatEmailAsMarkdown formats email with YAML frontmatter, markdown body, and YAML attachments
func formatEmailAsMarkdown(frontmatter EmailFrontmatter, body string, attachments []AttachmentMeta) (string, error) {
	var output strings.Builder

	// Write frontmatter
	output.WriteString("---\n")
	frontmatterBytes, err := yaml.Marshal(frontmatter)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}
	output.Write(frontmatterBytes)
	output.WriteString("---\n\n")

	// Write body
	output.WriteString(body)
	output.WriteString("\n")

	// Write attachments if present
	if len(attachments) > 0 {
		output.WriteString("\n---\n")
		output.WriteString("attachments:\n")
		for _, att := range attachments {
			output.WriteString(fmt.Sprintf("  - index: %d\n", att.Index))
			output.WriteString(fmt.Sprintf("    filename: %s\n", att.Filename))
			output.WriteString(fmt.Sprintf("    mime_type: %s\n", att.MimeType))
			output.WriteString(fmt.Sprintf("    size: %d\n", att.Size))
		}
	}

	return output.String(), nil
}

// formatEmailAsHTML formats email as HTML fragments (headers, body, attachments)
func formatEmailAsHTML(frontmatter EmailFrontmatter, body string, attachments []AttachmentMeta) string {
	var output strings.Builder

	// Headers section
	output.WriteString("<div class=\"email-headers\">\n")
	output.WriteString("  <dl>\n")
	output.WriteString(fmt.Sprintf("    <dt>Message ID</dt><dd>%s</dd>\n", escapeHTML(frontmatter.MessageID)))
	output.WriteString(fmt.Sprintf("    <dt>Thread ID</dt><dd>%s</dd>\n", escapeHTML(frontmatter.ThreadID)))
	output.WriteString(fmt.Sprintf("    <dt>From</dt><dd>%s</dd>\n", escapeHTML(frontmatter.From)))
	output.WriteString(fmt.Sprintf("    <dt>To</dt><dd>%s</dd>\n", escapeHTML(frontmatter.To)))
	if frontmatter.Cc != "" {
		output.WriteString(fmt.Sprintf("    <dt>Cc</dt><dd>%s</dd>\n", escapeHTML(frontmatter.Cc)))
	}
	output.WriteString(fmt.Sprintf("    <dt>Subject</dt><dd>%s</dd>\n", escapeHTML(frontmatter.Subject)))
	output.WriteString(fmt.Sprintf("    <dt>Date</dt><dd>%s</dd>\n", escapeHTML(frontmatter.Date)))
	if len(frontmatter.Labels) > 0 {
		output.WriteString(fmt.Sprintf("    <dt>Labels</dt><dd>%s</dd>\n", escapeHTML(strings.Join(frontmatter.Labels, ", "))))
	}
	output.WriteString("  </dl>\n")
	output.WriteString("</div>\n\n")

	output.WriteString("<hr class=\"email-separator\">\n\n")

	// Body section (raw HTML)
	output.WriteString(body)
	output.WriteString("\n")

	// Attachments section
	if len(attachments) > 0 {
		output.WriteString("\n<hr class=\"email-separator\">\n\n")
		output.WriteString("<div class=\"email-attachments\">\n")
		output.WriteString("  <h3>Attachments</h3>\n")
		output.WriteString("  <ul>\n")
		for _, att := range attachments {
			output.WriteString(fmt.Sprintf("    <li>[%d] <code>%s</code> (%s, %s)</li>\n",
				att.Index,
				escapeHTML(att.Filename),
				escapeHTML(att.MimeType),
				formatSizeBytes(att.Size)))
		}
		output.WriteString("  </ul>\n")
		output.WriteString("</div>\n")
	}

	return output.String()
}

// escapeHTML escapes special HTML characters
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// formatEmailAsPlainText formats email with YAML frontmatter and plain text body
func formatEmailAsPlainText(frontmatter EmailFrontmatter, body string, attachments []AttachmentMeta) (string, error) {
	// Reuse markdown formatter since structure is identical
	return formatEmailAsMarkdown(frontmatter, body, attachments)
}

// stripHTMLTags removes HTML tags for plain text fallback
func stripHTMLTags(html string) string {
	// Simple tag removal - not perfect but adequate for fallback
	stripped := html
	for {
		start := strings.Index(stripped, "<")
		if start == -1 {
			break
		}
		end := strings.Index(stripped[start:], ">")
		if end == -1 {
			break
		}
		stripped = stripped[:start] + stripped[start+end+1:]
	}
	return strings.TrimSpace(stripped)
}
