package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/googleapi"
)

// driveScopeHelp is the actionable message shown when Drive access is denied
// because the granted credentials lack the drive.readonly scope.
const driveScopeHelp = "Drive access is not authorized. " +
	"For an OAuth account, re-run `gwcli configure` to grant the " +
	"drive.readonly scope. For a service account, authorize " +
	"https://www.googleapis.com/auth/drive.readonly via domain-wide " +
	"delegation in the Google Workspace Admin console."

// wrapDriveErr turns the opaque Google API auth errors into an actionable
// message pointing at the scope/consent fix, and passes everything else
// through unchanged.
func wrapDriveErr(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		if apiErr.Code == 401 || apiErr.Code == 403 {
			return fmt.Errorf("%s (underlying: %w)", driveScopeHelp, err)
		}
	}
	msg := err.Error()
	for _, sig := range []string{
		"ACCESS_TOKEN_SCOPE_INSUFFICIENT",
		"insufficient authentication scopes",
		"insufficientPermissions",
		// Service-account DWD missing the drive.readonly scope: token
		// exchange fails before any API call.
		"unauthorized_client",
		"not authorized for any of the scopes",
		"cannot fetch token",
	} {
		if strings.Contains(msg, sig) {
			return fmt.Errorf("%s (underlying: %w)", driveScopeHelp, err)
		}
	}
	return err
}

// driveExportSpec returns the export MIME type and file extension to use for a
// native Google-apps document. Defaults to PDF for unknown native types.
func driveExportSpec(googleMime string) (exportMIME, ext string) {
	switch googleMime {
	case "application/vnd.google-apps.document":
		return "text/markdown", ".md"
	case "application/vnd.google-apps.spreadsheet":
		return "text/csv", ".csv"
	case "application/vnd.google-apps.presentation":
		return "application/pdf", ".pdf"
	case "application/vnd.google-apps.drawing":
		return "image/png", ".png"
	case "application/vnd.google-apps.script":
		return "application/vnd.google-apps.script+json", ".json"
	default:
		return "application/pdf", ".pdf"
	}
}

// sanitizeFilename strips path separators and trims a name so it is safe to
// write into the output directory.
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "artifact"
	}
	return name
}

// fetchDriveArtifact resolves the artifact's Drive file, then exports (native
// docs) or downloads (uploaded files) its content. It returns the bytes and a
// suggested filename. Folders are rejected; auth failures are made actionable.
func fetchDriveArtifact(ctx context.Context, conn *gwcli.CmdG, art driveArtifact) ([]byte, string, error) {
	svc := conn.DriveService()
	if svc == nil {
		return nil, "", fmt.Errorf("Drive service is not available for this connection. %s", driveScopeHelp)
	}

	meta, err := svc.Files.Get(art.ID).
		Fields("id,name,mimeType").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, "", wrapDriveErr(err)
	}

	if meta.MimeType == "application/vnd.google-apps.folder" {
		return nil, "", fmt.Errorf("artifact %q is a folder and cannot be downloaded", meta.Name)
	}

	baseName := meta.Name
	if baseName == "" {
		baseName = art.Title
	}
	if baseName == "" {
		baseName = art.ID
	}

	var resp *http.Response
	if strings.HasPrefix(meta.MimeType, "application/vnd.google-apps.") {
		exportMIME, ext := driveExportSpec(meta.MimeType)
		resp, err = svc.Files.Export(art.ID, exportMIME).Context(ctx).Download()
		if err != nil {
			return nil, "", wrapDriveErr(err)
		}
		if !strings.HasSuffix(strings.ToLower(baseName), ext) {
			baseName += ext
		}
	} else {
		resp, err = svc.Files.Get(art.ID).
			SupportsAllDrives(true).
			Context(ctx).
			Download()
		if err != nil {
			return nil, "", wrapDriveErr(err)
		}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading Drive content: %w", err)
	}
	return data, sanitizeFilename(baseName), nil
}

// runArtifactsList lists the Drive artifacts linked from a message. Mirrors
// `attachments list`.
func runArtifactsList(ctx context.Context, conn *gwcli.CmdG, messageID string, out *outputWriter) error {
	artifacts, err := extractDriveArtifacts(ctx, conn, messageID)
	if err != nil {
		return fmt.Errorf("failed to get drive artifacts: %w", err)
	}
	if len(artifacts) == 0 {
		return out.WriteEmptyList("No drive artifacts found")
	}

	if out.json {
		return out.writeJSON(artifacts)
	}

	headers := []string{"INDEX", "TITLE", "TYPE", "ID", "URL"}
	rows := make([][]string, len(artifacts))
	for i, a := range artifacts {
		title := a.Title
		if title == "" {
			title = a.Type
		}
		rows[i] = []string{
			fmt.Sprintf("%d", a.Index),
			title,
			a.Type,
			a.ID,
			a.URL,
		}
	}
	return out.writeTable(headers, rows)
}

// runArtifactsDownload exports/downloads selected Drive artifacts from a
// message. Mirrors `attachments download` (same flags & selection rules).
func runArtifactsDownload(ctx context.Context, conn *gwcli.CmdG, messageID string, indices []string, titlePattern, outputDir, outputFile string, out *outputWriter) error {
	artifacts, err := extractDriveArtifacts(ctx, conn, messageID)
	if err != nil {
		return fmt.Errorf("failed to get drive artifacts: %w", err)
	}
	if len(artifacts) == 0 {
		return fmt.Errorf("no drive artifacts found in message")
	}

	outputDir = expandPath(outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
	}

	toDownload, err := filterArtifacts(artifacts, indices, titlePattern)
	if err != nil {
		return err
	}

	downloaded := []string{}
	for _, art := range toDownload {
		data, filename, err := fetchDriveArtifact(ctx, conn, art)
		if err != nil {
			return fmt.Errorf("failed to download artifact %q: %w", art.Title, err)
		}

		var outputPath string
		if outputFile != "" && len(toDownload) == 1 {
			outputPath = outputFile
		} else {
			outputPath = findAvailableFilename(outputDir, filename)
		}

		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outputPath, err)
		}
		downloaded = append(downloaded, outputPath)

		if !out.json {
			out.writeMessage(fmt.Sprintf("Downloaded: %s (%s, %s)", outputPath, art.Type, formatSize(int64(len(data)))))
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

// filterArtifacts mirrors filterAttachments: select by index or by a glob on
// the artifact title; with no criteria, all artifacts are returned.
func filterArtifacts(artifacts []driveArtifact, indexStrs []string, titlePattern string) ([]driveArtifact, error) {
	criteriaCount := 0
	if len(indexStrs) > 0 {
		criteriaCount++
	}
	if titlePattern != "" {
		criteriaCount++
	}
	if criteriaCount > 1 {
		return nil, fmt.Errorf("cannot specify multiple selection criteria (use only one of: --index, --filename)")
	}
	if criteriaCount == 0 {
		return artifacts, nil
	}

	if len(indexStrs) > 0 {
		idxs, err := parseIndices(indexStrs)
		if err != nil {
			return nil, err
		}
		var selected []driveArtifact
		for _, idx := range idxs {
			if idx >= len(artifacts) {
				return nil, fmt.Errorf("index %d out of range (message has %d artifacts)", idx, len(artifacts))
			}
			selected = append(selected, artifacts[idx])
		}
		return selected, nil
	}

	var matched []driveArtifact
	for _, a := range artifacts {
		ok, err := filepath.Match(titlePattern, a.Title)
		if err != nil {
			return nil, fmt.Errorf("invalid title pattern %q: %w", titlePattern, err)
		}
		if ok {
			matched = append(matched, a)
		}
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("no artifacts match pattern %q", titlePattern)
	}
	return matched, nil
}
