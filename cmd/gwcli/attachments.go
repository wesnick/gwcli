package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wesnick/gwcli/pkg/gwcli"
)

// runAttachmentsList lists attachments in a message
func runAttachmentsList(ctx context.Context, conn *gwcli.CmdG, messageID string, out *outputWriter) error {
	msg := gwcli.NewMessage(conn, messageID)
	if err := msg.Preload(ctx, gwcli.LevelFull); err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	attachments, err := msg.Attachments(ctx)
	if err != nil {
		return fmt.Errorf("failed to get attachments: %w", err)
	}
	if len(attachments) == 0 {
		out.writeMessage("No attachments found")
		return nil
	}

	if out.json {
		output := make([]attachmentInfo, len(attachments))
		for i, att := range attachments {
			output[i] = attachmentInfo{
				Filename:     att.Part.Filename,
				MimeType:     att.Part.MimeType,
				Size:         att.Part.Body.Size,
				AttachmentID: att.ID,
			}
		}
		return out.writeJSON(output)
	}

	// Text output
	headers := []string{"FILENAME", "TYPE", "SIZE", "ID"}
	rows := make([][]string, len(attachments))
	for i, att := range attachments {
		rows[i] = []string{
			att.Part.Filename,
			att.Part.MimeType,
			formatSize(att.Part.Body.Size),
			att.ID,
		}
	}

	return out.writeTable(headers, rows)
}

// runAttachmentsDownload downloads attachments from a message
func runAttachmentsDownload(ctx context.Context, conn *gwcli.CmdG, messageID, attachmentID, outputDir, outputFile string, out *outputWriter) error {
	msg := gwcli.NewMessage(conn, messageID)
	if err := msg.Preload(ctx, gwcli.LevelFull); err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	attachments, err := msg.Attachments(ctx)
	if err != nil {
		return fmt.Errorf("failed to get attachments: %w", err)
	}
	if len(attachments) == 0 {
		return fmt.Errorf("no attachments found in message")
	}

	// Filter by attachment ID if specified
	toDownload := attachments
	if attachmentID != "" {
		toDownload = []*gwcli.Attachment{}
		for _, att := range attachments {
			if att.ID == attachmentID {
				toDownload = append(toDownload, att)
				break
			}
		}
		if len(toDownload) == 0 {
			return fmt.Errorf("attachment not found: %s", attachmentID)
		}
	}

	// Download attachments
	downloaded := []string{}
	for _, att := range toDownload {
		data, err := att.Download(ctx)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", att.Part.Filename, err)
		}

		// Determine output path
		var outputPath string
		if outputFile != "" && len(toDownload) == 1 {
			outputPath = outputFile
		} else {
			outputPath = filepath.Join(outputDir, att.Part.Filename)
		}

		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outputPath, err)
		}

		downloaded = append(downloaded, outputPath)

		if !out.json {
			out.writeMessage(fmt.Sprintf("Downloaded: %s (%s)", outputPath, formatSize(att.Part.Body.Size)))
		}
	}

	if out.json {
		return out.writeJSON(map[string]interface{}{
			"downloaded": len(downloaded),
			"files":      downloaded,
		})
	}

	return nil
}
