package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
				Index:    i,
				Filename: att.Part.Filename,
				MimeType: att.Part.MimeType,
				Size:     att.Part.Body.Size,
			}
		}
		return out.writeJSON(output)
	}

	// Text output
	headers := []string{"INDEX", "FILENAME", "TYPE", "SIZE"}
	rows := make([][]string, len(attachments))
	for i, att := range attachments {
		rows[i] = []string{
			fmt.Sprintf("%d", i),
			att.Part.Filename,
			att.Part.MimeType,
			formatSize(att.Part.Body.Size),
		}
	}

	return out.writeTable(headers, rows)
}

// runAttachmentsDownload downloads attachments from a message
func runAttachmentsDownload(ctx context.Context, conn *gwcli.CmdG, messageID string, indices []string, filenamePattern, outputDir, outputFile string, out *outputWriter) error {
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

	// Expand output directory (handle ~ for home directory)
	outputDir = expandPath(outputDir)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	// Filter attachments based on selection criteria
	toDownload, err := filterAttachments(attachments, indices, filenamePattern)
	if err != nil {
		return err
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
			outputPath = findAvailableFilename(outputDir, att.Part.Filename)
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

// expandPath expands ~ to the user's home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// parseIndices parses index strings (supports comma-separated values)
// Examples: "0", "0,1,2", "1,3"
func parseIndices(indexStrs []string) ([]int, error) {
	var indices []int
	seen := make(map[int]bool)

	for _, indexStr := range indexStrs {
		// Split by comma to support "0,1,2" format
		parts := strings.Split(indexStr, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			idx, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid index %q: must be a number", part)
			}
			if idx < 0 {
				return nil, fmt.Errorf("invalid index %d: must be non-negative", idx)
			}
			// Avoid duplicates
			if !seen[idx] {
				indices = append(indices, idx)
				seen[idx] = true
			}
		}
	}

	return indices, nil
}

// filterAttachments filters attachments based on selection criteria
func filterAttachments(attachments []*gwcli.Attachment, indexStrs []string, filenamePattern string) ([]*gwcli.Attachment, error) {
	// Count how many selection criteria are specified
	criteriaCount := 0
	if len(indexStrs) > 0 {
		criteriaCount++
	}
	if filenamePattern != "" {
		criteriaCount++
	}

	// If multiple criteria specified, return error
	if criteriaCount > 1 {
		return nil, fmt.Errorf("cannot specify multiple selection criteria (use only one of: --index, --filename)")
	}

	// If no criteria, download all
	if criteriaCount == 0 {
		return attachments, nil
	}

	// Filter by index
	if len(indexStrs) > 0 {
		indices, err := parseIndices(indexStrs)
		if err != nil {
			return nil, err
		}

		var selected []*gwcli.Attachment
		for _, idx := range indices {
			if idx >= len(attachments) {
				return nil, fmt.Errorf("index %d out of range (message has %d attachments)", idx, len(attachments))
			}
			selected = append(selected, attachments[idx])
		}
		return selected, nil
	}

	// Filter by filename pattern
	if filenamePattern != "" {
		var matched []*gwcli.Attachment
		for _, att := range attachments {
			match, err := filepath.Match(filenamePattern, att.Part.Filename)
			if err != nil {
				return nil, fmt.Errorf("invalid filename pattern %q: %w", filenamePattern, err)
			}
			if match {
				matched = append(matched, att)
			}
		}
		if len(matched) == 0 {
			return nil, fmt.Errorf("no attachments match pattern %q", filenamePattern)
		}
		return matched, nil
	}

	return attachments, nil
}

// findAvailableFilename finds an available filename, adding (n) suffix if needed
// Examples: "file.txt" -> "file (1).txt" -> "file (2).txt"
func findAvailableFilename(dir, filename string) string {
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	// File exists, find available suffix
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	for i := 1; i < 1000; i++ {
		newFilename := fmt.Sprintf("%s (%d)%s", base, i, ext)
		newPath := filepath.Join(dir, newFilename)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}

	// Fallback (extremely unlikely)
	return path
}
