package main

import (
	"bufio"
	"context"
	"fmt"
	"net/mail"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/gmail/v1"
)

// messageListOutput is JSON output format for message lists
type messageListOutput struct {
	ID       string   `json:"id"`
	ThreadID string   `json:"threadId"`
	Labels   []string `json:"labels"`
	Date     string   `json:"date"`
	From     string   `json:"from"`
	Subject  string   `json:"subject"`
	Snippet  string   `json:"snippet"`
}

func runMessagesList(ctx context.Context, conn *gwcli.CmdG, label string, limit int, unreadOnly bool, out *outputWriter) error {
	out.writeVerbose("Loading labels from config...")
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}

	labels := conn.Labels()
	out.writeVerbose("Loaded %d labels", len(labels))

	labelID := label
	if label != "" {
		found := false
		for _, l := range labels {
			out.writeVerbose("  Checking label: ID=%s Name=%s", l.ID, l.Label)
			if strings.EqualFold(l.Label, label) || l.ID == label {
				labelID = l.ID
				found = true
				out.writeVerbose("  -> Matched! Using label ID: %s", labelID)
				break
			}
		}
		if !found {
			out.writeVerbose("Available labels:")
			for _, l := range labels {
				out.writeVerbose("  - %s (ID: %s)", l.Label, l.ID)
			}
			return fmt.Errorf("label not found: %s", label)
		}
	}

	// Build query
	query := ""
	if unreadOnly {
		query = "is:unread"
	}

	// List messages - returns a Page
	page, err := conn.ListMessages(ctx, labelID, query, "")
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	if len(page.Messages) == 0 {
		out.writeMessage("No messages found")
		return nil
	}

	// Preload message metadata
	if err := page.PreloadSubjects(ctx); err != nil {
		return fmt.Errorf("failed to preload messages: %w", err)
	}

	// Limit the number of messages if needed
	messages := page.Messages
	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}

	// Output
	if out.json {
		output := make([]messageListOutput, len(messages))
		for i, msg := range messages {
			// Standardized error handling: log errors but continue processing
			threadID, err := msg.ThreadID(ctx)
			if err != nil {
				out.writeVerbose("Failed to get thread ID for message %s: %v", msg.ID, err)
				threadID = "" // Use empty string as fallback
			}
			from, err := msg.GetHeader(ctx, "From")
			if err != nil {
				out.writeVerbose("Failed to get From header for message %s: %v", msg.ID, err)
				from = "" // Use empty string as fallback
			}
			subject, err := msg.GetHeader(ctx, "Subject")
			if err != nil {
				out.writeVerbose("Failed to get Subject header for message %s: %v", msg.ID, err)
				subject = "" // Use empty string as fallback
			}
			date, err := msg.GetTimeFmt(ctx)
			if err != nil {
				out.writeVerbose("Failed to get date for message %s: %v", msg.ID, err)
				date = "" // Use empty string as fallback
			}
			labelIDs := msg.LocalLabels()

			// Get snippet from the Response if available
			var snippet string
			if msg.Response != nil {
				snippet = msg.Response.Snippet
			}

			output[i] = messageListOutput{
				ID:       msg.ID,
				ThreadID: string(threadID),
				Labels:   labelIDs,
				Date:     date,
				From:     from,
				Subject:  subject,
				Snippet:  snippet,
			}
		}
		return out.writeJSON(output)
	}

	// Text output
	headers := []string{"ID", "DATE", "FROM", "SUBJECT", "LABELS"}
	rows := make([][]string, len(messages))
	for i, msg := range messages {
		// Standardized error handling: log errors but continue processing
		from, err := msg.GetHeader(ctx, "From")
		if err != nil {
			out.writeVerbose("Failed to get From header for message %s: %v", msg.ID, err)
			from = "" // Use empty string as fallback
		}
		if len(from) > 30 {
			from = truncateString(from, 30)
		}

		subject, err := msg.GetHeader(ctx, "Subject")
		if err != nil {
			out.writeVerbose("Failed to get Subject header for message %s: %v", msg.ID, err)
			subject = "" // Use empty string as fallback
		}
		if len(subject) > 40 {
			subject = truncateString(subject, 40)
		}

		date, err := msg.GetTimeFmt(ctx)
		if err != nil {
			out.writeVerbose("Failed to get date for message %s: %v", msg.ID, err)
			date = "" // Use empty string as fallback
		}

		// Get label names
		labelIDs := msg.LocalLabels()
		labelNames := []string{}
		for _, labelID := range labelIDs {
			for _, l := range labels {
				if l.ID == labelID {
					labelNames = append(labelNames, l.Label)
					break
				}
			}
		}

		rows[i] = []string{
			msg.ID,
			date,
			from,
			subject,
			strings.Join(labelNames, ", "),
		}
	}

	return out.writeTable(headers, rows)
}

// messageReadOutput is JSON output format for reading a message
type messageReadOutput struct {
	ID           string            `json:"id"`
	ThreadID     string            `json:"threadId"`
	LabelIDs     []string          `json:"labelIds"`
	Snippet      string            `json:"snippet"`
	Headers      map[string]string `json:"headers"`
	Body         string            `json:"body,omitempty"`
	BodyHTML     string            `json:"bodyHtml,omitempty"`
	BodyMarkdown string            `json:"bodyMarkdown,omitempty"`
	Attachments  []attachmentInfo  `json:"attachments,omitempty"`
	Raw          string            `json:"raw,omitempty"`
}

type attachmentInfo struct {
	Index    int    `json:"index"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

// extractRawHTML extracts the raw HTML body from a message without rendering it with lynx
func extractRawHTML(ctx context.Context, conn *gwcli.CmdG, messageID string) (string, error) {
	// Fetch the message directly from Gmail API with full format to get payload
	// but bypass cmdg's body rendering logic
	gmailSvc := conn.GmailService()
	msg, err := gmailSvc.Users.Messages.Get("me", messageID).
		Format("full").
		Context(ctx).
		Do()
	if err != nil {
		return "", err
	}

	// Extract raw HTML from the payload
	return extractHTMLFromPart(msg.Payload), nil
}

// extractPlainText extracts plain text body without rendering HTML
func extractPlainText(ctx context.Context, conn *gwcli.CmdG, messageID string) (string, error) {
	gmailSvc := conn.GmailService()
	msg, err := gmailSvc.Users.Messages.Get("me", messageID).
		Format("full").
		Context(ctx).
		Do()
	if err != nil {
		return "", err
	}

	return extractPlainTextFromPart(msg.Payload), nil
}

// extractPlainTextFromPart recursively searches for plain text parts
func extractPlainTextFromPart(part *gmail.MessagePart) string {
	if part == nil {
		return ""
	}

	// Check if this part is plain text
	if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
		decoded, err := gwcli.MIMEDecode(string(part.Body.Data))
		if err != nil {
			return ""
		}
		return decoded
	}

	// For multipart, search recursively
	if len(part.Parts) > 0 {
		// For multipart/alternative, prefer plain text
		if part.MimeType == "multipart/alternative" {
			for _, p := range part.Parts {
				if p.MimeType == "text/plain" {
					if text := extractPlainTextFromPart(p); text != "" {
						return text
					}
				}
			}
		}

		// Otherwise search all parts
		for _, p := range part.Parts {
			if text := extractPlainTextFromPart(p); text != "" {
				return text
			}
		}
	}

	return ""
}

// extractAttachmentsInfo extracts attachment info without triggering body rendering
func extractAttachmentsInfo(ctx context.Context, conn *gwcli.CmdG, messageID string) ([]attachmentInfo, error) {
	gmailSvc := conn.GmailService()
	msg, err := gmailSvc.Users.Messages.Get("me", messageID).
		Format("full").
		Context(ctx).
		Do()
	if err != nil {
		return nil, err
	}

	var attachments []attachmentInfo
	extractAttachmentsFromPart(msg.Payload, &attachments)

	// Set indices
	for i := range attachments {
		attachments[i].Index = i
	}

	return attachments, nil
}

// extractAttachmentsFromPart recursively searches for attachments
func extractAttachmentsFromPart(part *gmail.MessagePart, attachments *[]attachmentInfo) {
	if part == nil {
		return
	}

	// Check if this part is an attachment
	if part.Filename != "" && part.Body != nil {
		*attachments = append(*attachments, attachmentInfo{
			Filename: part.Filename,
			MimeType: part.MimeType,
			Size:     part.Body.Size,
		})
	}

	// Search in nested parts
	for _, p := range part.Parts {
		extractAttachmentsFromPart(p, attachments)
	}
}

// extractHTMLFromPart recursively searches for HTML parts in the message payload
func extractHTMLFromPart(part *gmail.MessagePart) string {
	if part == nil {
		return ""
	}

	// Check if this part is HTML
	if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
		decoded, err := gwcli.MIMEDecode(string(part.Body.Data))
		if err != nil {
			return ""
		}
		return decoded
	}

	// For multipart, search recursively - prefer HTML from alternative parts
	if len(part.Parts) > 0 {
		// First try to find in a multipart/alternative (prefer HTML over text)
		if part.MimeType == "multipart/alternative" {
			for _, p := range part.Parts {
				if p.MimeType == "text/html" {
					if html := extractHTMLFromPart(p); html != "" {
						return html
					}
				}
			}
		}

		// Otherwise search all parts recursively
		for _, p := range part.Parts {
			if html := extractHTMLFromPart(p); html != "" {
				return html
			}
		}
	}

	return ""
}

// runMessagesRead reads and displays a single message
func runMessagesRead(ctx context.Context, conn *gwcli.CmdG, messageID string, raw, headersOnly, rawHTML, preferPlain bool, out *outputWriter) error {
	// Validate flags
	if rawHTML && preferPlain {
		return fmt.Errorf("--raw-html and --prefer-plain are mutually exclusive")
	}

	// Create message and fetch at appropriate level
	msg := gwcli.NewMessage(conn, messageID)

	if raw {
		// Fetch raw message
		rawData, err := msg.Raw(ctx)
		if err != nil {
			return fmt.Errorf("failed to get raw message: %w", err)
		}

		if out.json {
			output := messageReadOutput{
				ID:  messageID,
				Raw: rawData,
			}
			return out.writeJSON(output)
		}

		// Text output - just print raw
		fmt.Println(rawData)
		return nil
	}

	// Determine level and fetch message
	// Always just load metadata - we extract bodies directly from API to avoid lynx
	level := gwcli.LevelMetadata
	if err := msg.Preload(ctx, level); err != nil {
		return fmt.Errorf("failed to get message metadata: %w", err)
	}

	// JSON output
	if out.json {
		threadID, _ := msg.ThreadID(ctx)

		output := messageReadOutput{
			ID:       messageID,
			ThreadID: string(threadID),
			LabelIDs: msg.Response.LabelIds,
			Snippet:  msg.Response.Snippet,
			Headers:  make(map[string]string),
		}

		// Extract common headers
		commonHeaders := []string{"From", "To", "Cc", "Subject", "Date"}
		for _, h := range commonHeaders {
			if val, err := msg.GetHeader(ctx, h); err == nil && val != "" {
				output.Headers[h] = val
			}
		}

		if !headersOnly {
			// Get all body formats for JSON
			bodyHTML, err := extractRawHTML(ctx, conn, messageID)
			if err == nil {
				output.BodyHTML = bodyHTML

				// Convert HTML to markdown
				if bodyHTML != "" {
					bodyMarkdown, err := convertHTMLToMarkdown(bodyHTML)
					if err == nil {
						output.BodyMarkdown = bodyMarkdown
					}
				}
			}

			// Get plain text body
			body, err := extractPlainText(ctx, conn, messageID)
			if err == nil {
				output.Body = body
			}

			// Attachments
			attachmentsInfo, err := extractAttachmentsInfo(ctx, conn, messageID)
			if err == nil && len(attachmentsInfo) > 0 {
				output.Attachments = attachmentsInfo
			}
		}

		return out.writeJSON(output)
	}

	// Text output
	// Get thread ID
	threadID, _ := msg.ThreadID(ctx)

	// Headers
	from, _ := msg.GetHeader(ctx, "From")
	to, _ := msg.GetHeader(ctx, "To")
	cc, _ := msg.GetHeader(ctx, "Cc")
	subject, _ := msg.GetHeader(ctx, "Subject")
	date, _ := msg.GetHeader(ctx, "Date")

	if headersOnly {
		// Build frontmatter for headers-only mode
		frontmatter := EmailFrontmatter{
			MessageID: messageID,
			ThreadID:  string(threadID),
			From:      from,
			To:        to,
			Cc:        cc,
			Subject:   subject,
			Date:      date,
			Labels:    msg.Response.LabelIds,
		}

		// Output just frontmatter
		output, err := formatEmailAsMarkdown(frontmatter, "", nil)
		if err != nil {
			return fmt.Errorf("failed to format headers: %w", err)
		}
		fmt.Print(output)
		return nil
	}

	// Determine output format based on flags
	var format OutputFormat
	if rawHTML {
		format = FormatHTML
	} else if preferPlain {
		format = FormatPlainText
	} else {
		format = FormatMarkdown // Default
	}

	// Body selection with fallback logic
	var bodyContent string
	var fallbackNote string

	switch format {
	case FormatMarkdown:
		// Try HTML -> markdown first
		htmlBody, err := extractRawHTML(ctx, conn, messageID)
		if err == nil && htmlBody != "" {
			bodyContent, err = convertHTMLToMarkdown(htmlBody)
			if err != nil {
				return fmt.Errorf("failed to convert HTML to markdown: %w", err)
			}
		} else {
			// Fallback to plain text
			plainText, err := extractPlainText(ctx, conn, messageID)
			if err == nil && plainText != "" {
				bodyContent = plainText
				fallbackNote = "HTML body not available, showing plain text"
			} else {
				bodyContent = "<!-- No body found in this message -->"
				fallbackNote = "Neither HTML nor plain text body available"
			}
		}

	case FormatHTML:
		// Try HTML first
		htmlBody, err := extractRawHTML(ctx, conn, messageID)
		if err == nil && htmlBody != "" {
			bodyContent = htmlBody
		} else {
			// Fallback to plain text wrapped in <pre>
			plainText, err := extractPlainText(ctx, conn, messageID)
			if err == nil && plainText != "" {
				bodyContent = fmt.Sprintf("<pre>%s</pre>", escapeHTML(plainText))
				fallbackNote = "HTML body not available, showing plain text"
			} else {
				bodyContent = "<!-- No body found in this message -->"
				fallbackNote = "Neither HTML nor plain text body available"
			}
		}

	case FormatPlainText:
		// Try plain text first
		plainText, err := extractPlainText(ctx, conn, messageID)
		if err == nil && plainText != "" {
			bodyContent = plainText
		} else {
			// Fallback to HTML -> plain text conversion or snippet
			htmlBody, err := extractRawHTML(ctx, conn, messageID)
			if err == nil && htmlBody != "" {
				// Best effort: strip HTML tags for plain text
				bodyContent = stripHTMLTags(htmlBody)
				fallbackNote = "Plain text body not available, converted from HTML"
			} else {
				bodyContent = msg.Response.Snippet
				fallbackNote = "Neither plain text nor HTML body available, showing snippet"
			}
		}
	}

	// Build frontmatter
	frontmatter := EmailFrontmatter{
		MessageID: messageID,
		ThreadID:  string(threadID),
		From:      from,
		To:        to,
		Cc:        cc,
		Subject:   subject,
		Date:      date,
		Labels:    msg.Response.LabelIds,
		Note:      fallbackNote,
	}

	// Get attachments
	var attachmentsMeta []AttachmentMeta
	attachmentsInfo, err := extractAttachmentsInfo(ctx, conn, messageID)
	if err == nil && len(attachmentsInfo) > 0 {
		for _, att := range attachmentsInfo {
			attachmentsMeta = append(attachmentsMeta, AttachmentMeta{
				Index:    att.Index,
				Filename: att.Filename,
				MimeType: att.MimeType,
				Size:     att.Size,
			})
		}
	}

	// Format output based on selected format
	var formattedOutput string
	switch format {
	case FormatMarkdown:
		formattedOutput, err = formatEmailAsMarkdown(frontmatter, bodyContent, attachmentsMeta)
		if err != nil {
			return fmt.Errorf("failed to format as markdown: %w", err)
		}
	case FormatHTML:
		formattedOutput = formatEmailAsHTML(frontmatter, bodyContent, attachmentsMeta)
	case FormatPlainText:
		formattedOutput, err = formatEmailAsPlainText(frontmatter, bodyContent, attachmentsMeta)
		if err != nil {
			return fmt.Errorf("failed to format as plain text: %w", err)
		}
	}

	fmt.Print(formattedOutput)
	return nil
}

func runMessagesSearch(ctx context.Context, conn *gwcli.CmdG, query string, limit int, out *outputWriter) error {
	out.writeVerbose("Searching with query: %s", query)

	page, err := conn.ListMessages(ctx, "", query, "")
	if err != nil {
		return fmt.Errorf("failed to search messages: %w", err)
	}

	out.writeVerbose("Found %d messages", len(page.Messages))

	if len(page.Messages) == 0 {
		out.writeMessage("No messages found")
		return nil
	}

	if err := page.PreloadSubjects(ctx); err != nil {
		return fmt.Errorf("failed to preload messages: %w", err)
	}

	messages := page.Messages
	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
		out.writeVerbose("Limited to %d messages", limit)
	}

	out.writeVerbose("Loading labels from config...")
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}

	labels := conn.Labels()
	out.writeVerbose("Loaded %d labels for display", len(labels))

	// Reuse list output logic
	if out.json {
		output := make([]messageListOutput, len(messages))
		for i, msg := range messages {
			// Standardized error handling: log errors but continue processing
			threadID, err := msg.ThreadID(ctx)
			if err != nil {
				out.writeVerbose("Failed to get thread ID for message %s: %v", msg.ID, err)
				threadID = "" // Use empty string as fallback
			}
			from, err := msg.GetHeader(ctx, "From")
			if err != nil {
				out.writeVerbose("Failed to get From header for message %s: %v", msg.ID, err)
				from = "" // Use empty string as fallback
			}
			subject, err := msg.GetHeader(ctx, "Subject")
			if err != nil {
				out.writeVerbose("Failed to get Subject header for message %s: %v", msg.ID, err)
				subject = "" // Use empty string as fallback
			}
			date, err := msg.GetTimeFmt(ctx)
			if err != nil {
				out.writeVerbose("Failed to get date for message %s: %v", msg.ID, err)
				date = "" // Use empty string as fallback
			}
			labelIDs := msg.LocalLabels()

			var snippet string
			if msg.Response != nil {
				snippet = msg.Response.Snippet
			}

			output[i] = messageListOutput{
				ID:       msg.ID,
				ThreadID: string(threadID),
				Labels:   labelIDs,
				Date:     date,
				From:     from,
				Subject:  subject,
				Snippet:  snippet,
			}
		}
		return out.writeJSON(output)
	}

	// Text output
	headers := []string{"ID", "DATE", "FROM", "SUBJECT", "LABELS"}
	rows := make([][]string, len(messages))
	for i, msg := range messages {
		// Standardized error handling: log errors but continue processing
		from, err := msg.GetHeader(ctx, "From")
		if err != nil {
			out.writeVerbose("Failed to get From header for message %s: %v", msg.ID, err)
			from = "" // Use empty string as fallback
		}
		from = truncateString(from, 30)

		subject, err := msg.GetHeader(ctx, "Subject")
		if err != nil {
			out.writeVerbose("Failed to get Subject header for message %s: %v", msg.ID, err)
			subject = "" // Use empty string as fallback
		}
		subject = truncateString(subject, 40)

		date, err := msg.GetTimeFmt(ctx)
		if err != nil {
			out.writeVerbose("Failed to get date for message %s: %v", msg.ID, err)
			date = "" // Use empty string as fallback
		}

		labelIDs := msg.LocalLabels()
		labelNames := []string{}
		for _, labelID := range labelIDs {
			for _, l := range labels {
				if l.ID == labelID {
					labelNames = append(labelNames, l.Label)
					break
				}
			}
		}

		rows[i] = []string{
			msg.ID,
			date,
			from,
			subject,
			strings.Join(labelNames, ", "),
		}
	}

	return out.writeTable(headers, rows)
}

// runMessagesSend sends an email message
func runMessagesSend(ctx context.Context, conn *gwcli.CmdG, to, cc, bcc []string, subject, body string, attachments []string, html bool, threadID string, out *outputWriter) error {
	// Read body from stdin if not provided
	if body == "" {
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading body from stdin: %w", err)
		}
		body = strings.Join(lines, "\n")
	}

	// Build message parts
	parts := []*gwcli.Part{}

	// Add body
	contentType := `text/plain; charset="UTF-8"`
	if html {
		contentType = `text/html; charset="UTF-8"`
	}
	parts = append(parts, &gwcli.Part{
		Header: textproto.MIMEHeader{
			"Content-Type":        {contentType},
			"Content-Disposition": {"inline"},
		},
		Contents: body,
	})

	// Add attachments
	for _, path := range attachments {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read attachment %s: %w", path, err)
		}

		filename := filepath.Base(path)
		mimeType := "application/octet-stream"
		// Basic MIME type detection
		if strings.HasSuffix(filename, ".pdf") {
			mimeType = "application/pdf"
		} else if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
			mimeType = "image/jpeg"
		} else if strings.HasSuffix(filename, ".png") {
			mimeType = "image/png"
		} else if strings.HasSuffix(filename, ".txt") {
			mimeType = "text/plain"
		}

		parts = append(parts, &gwcli.Part{
			Header: textproto.MIMEHeader{
				"Content-Type":        {fmt.Sprintf(`%s; name=%q`, mimeType, filename)},
				"Content-Disposition": {fmt.Sprintf(`attachment; filename=%q`, filename)},
			},
			Contents: string(data),
		})
	}

	// Build headers
	headers := mail.Header{
		"To":           {strings.Join(to, ", ")},
		"Subject":      {subject},
		"MIME-Version": {"1.0"},
	}
	if len(cc) > 0 {
		headers["Cc"] = []string{strings.Join(cc, ", ")}
	}
	if len(bcc) > 0 {
		headers["Bcc"] = []string{strings.Join(bcc, ", ")}
	}

	// Determine multipart type
	multipartType := "mixed"

	// Send the message
	// Note: SendParts API does not return the message ID of the sent message.
	// This is a limitation of the Gmail API's messages.send endpoint when using
	// raw RFC822 format. The API only confirms successful submission.
	err := conn.SendParts(ctx, gwcli.ThreadID(threadID), multipartType, headers, parts)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	if out.json {
		// Message ID is not available from SendParts API
		return out.writeJSON(map[string]string{"status": "sent"})
	}

	out.writeMessage("Message sent successfully")
	return nil
}

// runMessagesDelete deletes messages (moves to trash)
func runMessagesDelete(ctx context.Context, conn *gwcli.CmdG, messageID string, stdin bool, verbose bool, out *outputWriter) error {
	var ids []string
	var err error

	if stdin {
		ids, err = readIDsFromStdin()
		if err != nil {
			return err
		}
	} else {
		if messageID == "" {
			return fmt.Errorf("either provide message ID or use --stdin")
		}
		ids = []string{messageID}
	}

	// Batch delete
	if len(ids) == 1 {
		// Single message
		if err := conn.BatchTrash(ctx, ids); err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}
		if out.json {
			return out.writeJSON(map[string]int{"deleted": 1})
		}
		out.writeMessage("Message deleted")
		return nil
	}

	// Multiple messages
	bp := newBatchProcessor(len(ids), verbose)
	err = bp.process(ctx, ids, func(ctx context.Context, id string) error {
		return conn.BatchTrash(ctx, []string{id})
	})

	if out.json {
		return out.writeJSON(map[string]int{
			"deleted": bp.processed - len(bp.errors),
			"errors":  len(bp.errors),
		})
	}

	bp.report(os.Stdout)
	return err
}

// runMessagesMarkRead marks messages as read
func runMessagesMarkRead(ctx context.Context, conn *gwcli.CmdG, messageID string, stdin bool, verbose bool, out *outputWriter) error {
	var ids []string
	var err error

	if stdin {
		ids, err = readIDsFromStdin()
		if err != nil {
			return err
		}
	} else {
		if messageID == "" {
			return fmt.Errorf("either provide message ID or use --stdin")
		}
		ids = []string{messageID}
	}

	// Remove UNREAD label
	unreadLabelID := gwcli.Unread

	// Batch operation
	bp := newBatchProcessor(len(ids), verbose)
	err = bp.process(ctx, ids, func(ctx context.Context, id string) error {
		return conn.BatchUnlabel(ctx, []string{id}, unreadLabelID)
	})

	if out.json {
		return out.writeJSON(map[string]int{
			"marked": bp.processed - len(bp.errors),
			"errors": len(bp.errors),
		})
	}

	if len(ids) == 1 {
		out.writeMessage("Message marked as read")
	} else {
		bp.report(os.Stdout)
	}
	return err
}

// runMessagesMarkUnread marks messages as unread
func runMessagesMarkUnread(ctx context.Context, conn *gwcli.CmdG, messageID string, stdin bool, verbose bool, out *outputWriter) error {
	var ids []string
	var err error

	if stdin {
		ids, err = readIDsFromStdin()
		if err != nil {
			return err
		}
	} else {
		if messageID == "" {
			return fmt.Errorf("either provide message ID or use --stdin")
		}
		ids = []string{messageID}
	}

	// Add UNREAD label
	unreadLabelID := gwcli.Unread

	// Batch operation
	bp := newBatchProcessor(len(ids), verbose)
	err = bp.process(ctx, ids, func(ctx context.Context, id string) error {
		return conn.BatchLabel(ctx, []string{id}, unreadLabelID)
	})

	if out.json {
		return out.writeJSON(map[string]int{
			"marked": bp.processed - len(bp.errors),
			"errors": len(bp.errors),
		})
	}

	if len(ids) == 1 {
		out.writeMessage("Message marked as unread")
	} else {
		bp.report(os.Stdout)
	}
	return err
}

func runMessagesMove(ctx context.Context, conn *gwcli.CmdG, messageID, toLabelName string, stdin bool, verbose bool, out *outputWriter) error {
	var ids []string
	var err error

	if stdin {
		ids, err = readIDsFromStdin()
		if err != nil {
			return err
		}
	} else {
		if messageID == "" {
			return fmt.Errorf("either provide message ID or use --stdin")
		}
		ids = []string{messageID}
	}

	out.writeVerbose("Loading labels from config...")
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}

	labels := conn.Labels()
	out.writeVerbose("Loaded %d labels, resolving '%s'...", len(labels), toLabelName)

	toLabelID := ""
	for _, l := range labels {
		if strings.EqualFold(l.Label, toLabelName) || l.ID == toLabelName {
			toLabelID = l.ID
			out.writeVerbose("Resolved label '%s' to ID '%s'", toLabelName, toLabelID)
			break
		}
	}

	if toLabelID == "" {
		out.writeVerbose("Label '%s' not found. Available labels:", toLabelName)
		for _, l := range labels {
			out.writeVerbose("  - %s (ID: %s)", l.Label, l.ID)
		}
		return fmt.Errorf("label not found: %s", toLabelName)
	}

	// Batch operation
	// Note: "Move" in Gmail means adding the destination label and removing INBOX.
	// This implements Gmail's label-based filing system where messages can have
	// multiple labels. The move operation adds the target label and removes INBOX
	// to achieve the traditional "move" behavior users expect.
	bp := newBatchProcessor(len(ids), verbose)
	err = bp.process(ctx, ids, func(ctx context.Context, id string) error {
		// Get current labels
		msg := gwcli.NewMessage(conn, id)
		if err := msg.Preload(ctx, gwcli.LevelMinimal); err != nil {
			return err
		}

		// Remove INBOX if present
		removeLabels := []string{}
		if msg.Response != nil {
			for _, labelID := range msg.Response.LabelIds {
				if labelID == gwcli.Inbox {
					removeLabels = append(removeLabels, labelID)
				}
			}
		}

		// Add new label
		if err := conn.BatchLabel(ctx, []string{id}, toLabelID); err != nil {
			return err
		}

		// Remove old labels
		if len(removeLabels) > 0 {
			if err := conn.BatchUnlabel(ctx, []string{id}, removeLabels[0]); err != nil {
				return err
			}
		}

		return nil
	})

	if out.json {
		return out.writeJSON(map[string]int{
			"moved":  bp.processed - len(bp.errors),
			"errors": len(bp.errors),
		})
	}

	if len(ids) == 1 {
		out.writeMessage(fmt.Sprintf("Message moved to %s", toLabelName))
	} else {
		bp.report(os.Stdout)
	}
	return err
}
