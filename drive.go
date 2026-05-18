package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// resolveDriveRef accepts either a raw Drive file ID or a Drive/Docs URL and
// returns a driveArtifact suitable for fetchDriveArtifact / metadata lookups.
// The type is only a hint for callers; fetchDriveArtifact re-derives the real
// export/download behavior from the file's actual mimeType.
func resolveDriveRef(ref string) (driveArtifact, error) {
	if ref == "" {
		return driveArtifact{}, fmt.Errorf("a Drive file ID or URL is required")
	}
	if id, typ, ok := parseDriveURL(ref); ok {
		return driveArtifact{ID: id, Type: typ, URL: canonicalDriveURL(id, typ)}, nil
	}
	// Not a URL: treat the whole argument as a raw file ID. Type is unknown
	// until Files.Get; "drive-file" yields a sensible canonical URL.
	return driveArtifact{ID: ref, Type: "drive-file", URL: canonicalDriveURL(ref, "drive-file")}, nil
}

// runDriveGet fetches Drive file metadata only (no content download).
func runDriveGet(ctx context.Context, conn *gwcli.CmdG, ref string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}

	svc := conn.DriveService()
	if svc == nil {
		return fmt.Errorf("Drive service is not available for this connection. %s", driveScopeHelp)
	}

	f, err := svc.Files.Get(art.ID).
		Fields("id,name,mimeType,size,modifiedTime,owners(displayName,emailAddress)").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}

	if out.json {
		owners := make([]map[string]string, 0, len(f.Owners))
		for _, o := range f.Owners {
			owners = append(owners, map[string]string{
				"displayName":  o.DisplayName,
				"emailAddress": o.EmailAddress,
			})
		}
		return out.writeJSON(map[string]interface{}{
			"id":           f.Id,
			"name":         f.Name,
			"mimeType":     f.MimeType,
			"size":         f.Size,
			"modifiedTime": f.ModifiedTime,
			"owners":       owners,
			"url":          canonicalDriveURL(f.Id, art.Type),
		})
	}

	sizeStr := "-"
	if f.Size > 0 {
		sizeStr = formatSize(f.Size)
	}
	owner := ""
	if len(f.Owners) > 0 {
		owner = f.Owners[0].EmailAddress
		if owner == "" {
			owner = f.Owners[0].DisplayName
		}
	}
	headers := []string{"ID", "NAME", "MIME TYPE", "SIZE", "MODIFIED", "OWNER"}
	rows := [][]string{{f.Id, f.Name, f.MimeType, sizeStr, f.ModifiedTime, owner}}
	return out.writeTable(headers, rows)
}

// runDriveExport exports (native Google-apps docs) or downloads (binary files)
// a Drive file by ID or URL. Mirrors `artifacts download` output conventions.
func runDriveExport(ctx context.Context, conn *gwcli.CmdG, ref, exportFormat, outputDir, outputFile string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}

	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	meta, err := svc.Files.Get(art.ID).
		Fields("id,name,mimeType").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	if meta.MimeType == driveFolderMime {
		dest := expandPath(outputDir)
		if outputFile != "" {
			dest = expandPath(outputFile)
		}
		root := filepath.Join(dest, sanitizeFilename(meta.Name))
		count, err := exportDriveFolder(ctx, conn, svc, art.ID, root, exportFormat)
		if err != nil {
			return err
		}
		if out.json {
			return out.writeJSON(map[string]interface{}{
				"id":    art.ID,
				"dir":   root,
				"files": count,
			})
		}
		out.writeMessage(fmt.Sprintf("Exported %d file(s) to %s", count, root))
		return nil
	}

	data, filename, err := fetchDriveArtifactFormat(ctx, conn, art, exportFormat)
	if err != nil {
		return fmt.Errorf("failed to export Drive file %q: %w", art.ID, err)
	}

	var outputPath string
	if outputFile != "" {
		outputPath = outputFile
	} else {
		outputDir = expandPath(outputDir)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
		}
		outputPath = findAvailableFilename(outputDir, filename)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputPath, err)
	}

	if out.json {
		return out.writeJSON(map[string]interface{}{
			"id":    art.ID,
			"file":  outputPath,
			"bytes": len(data),
		})
	}
	out.writeMessage(fmt.Sprintf("Exported: %s (%s)", outputPath, formatSize(int64(len(data)))))
	return nil
}

// exportDriveFolder recursively exports every file under folderID into the
// local directory dir (created mirroring the Drive folder tree). Native docs
// use exportFormat (or their per-type default); binary files are downloaded
// as-is. Returns the number of files written.
func exportDriveFolder(ctx context.Context, conn *gwcli.CmdG, svc *drive.Service, folderID, dir, exportFormat string) (int, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create %s: %w", dir, err)
	}
	count := 0
	pageToken := ""
	for {
		call := svc.Files.List().
			Q(fmt.Sprintf("'%s' in parents and trashed = false", driveQuoteValue(folderID))).
			Fields("nextPageToken, files(id,name,mimeType)").
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true).
			Corpora("allDrives").
			PageSize(100).
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return count, wrapDriveErr(err)
		}
		for _, f := range resp.Files {
			if f.MimeType == driveFolderMime {
				sub := filepath.Join(dir, sanitizeFilename(f.Name))
				n, err := exportDriveFolder(ctx, conn, svc, f.Id, sub, exportFormat)
				if err != nil {
					return count, err
				}
				count += n
				continue
			}
			art := driveArtifact{ID: f.Id, Title: f.Name}
			data, filename, err := fetchDriveArtifactFormat(ctx, conn, art, exportFormat)
			if err != nil {
				return count, fmt.Errorf("failed to export %q: %w", f.Name, err)
			}
			outPath := findAvailableFilename(dir, filename)
			if err := os.WriteFile(outPath, data, 0644); err != nil {
				return count, fmt.Errorf("failed to write %s: %w", outPath, err)
			}
			count++
		}
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return count, nil
}

// driveListFile is the trimmed metadata shape emitted by list/search.
type driveListFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	Size         int64  `json:"size,omitempty"`
	ModifiedTime string `json:"modifiedTime,omitempty"`
}

// driveList runs a paginated Files.List with the given raw Drive query,
// stopping once limit results are collected (limit <= 0 means no cap).
func driveList(ctx context.Context, conn *gwcli.CmdG, query string, limit int) ([]driveListFile, error) {
	svc := conn.DriveService()
	if svc == nil {
		return nil, fmt.Errorf("Drive service is not available for this connection. %s", driveScopeHelp)
	}

	var files []driveListFile
	pageToken := ""
	for {
		call := svc.Files.List().
			Fields("nextPageToken, files(id,name,mimeType,size,modifiedTime)").
			SupportsAllDrives(true).
			IncludeItemsFromAllDrives(true).
			Corpora("allDrives").
			OrderBy("modifiedTime desc").
			PageSize(100).
			Context(ctx)
		if query != "" {
			call = call.Q(query)
		}
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, wrapDriveErr(err)
		}
		for _, f := range resp.Files {
			files = append(files, driveListFile{
				ID:           f.Id,
				Name:         f.Name,
				MimeType:     f.MimeType,
				Size:         f.Size,
				ModifiedTime: f.ModifiedTime,
			})
			if limit > 0 && len(files) >= limit {
				return files, nil
			}
		}
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return files, nil
}

func writeDriveList(files []driveListFile, out *outputWriter) error {
	if len(files) == 0 {
		return out.WriteEmptyList("No Drive files found")
	}
	if out.json {
		return out.writeJSON(files)
	}
	headers := []string{"ID", "NAME", "MIME TYPE", "SIZE", "MODIFIED"}
	rows := make([][]string, len(files))
	for i, f := range files {
		sizeStr := "-"
		if f.Size > 0 {
			sizeStr = formatSize(f.Size)
		}
		rows[i] = []string{f.ID, f.Name, f.MimeType, sizeStr, f.ModifiedTime}
	}
	return out.writeTable(headers, rows)
}

// runDriveList lists Drive files. query is a raw Drive `q` expression; folder,
// when set, restricts results to direct children of that folder ID.
func runDriveList(ctx context.Context, conn *gwcli.CmdG, query, folder string, limit int, out *outputWriter) error {
	clauses := []string{}
	if query != "" {
		clauses = append(clauses, "("+query+")")
	}
	if folder != "" {
		clauses = append(clauses, fmt.Sprintf("%q in parents", folder))
	}
	// Hide trashed items by default (they otherwise leak into folder/listing
	// results). A caller who explicitly filters on `trashed` in --query keeps
	// full control.
	if !strings.Contains(query, "trashed") {
		clauses = append(clauses, "trashed = false")
	}
	files, err := driveList(ctx, conn, strings.Join(clauses, " and "), limit)
	if err != nil {
		return err
	}
	return writeDriveList(files, out)
}

// runDriveSearch is a convenience wrapper turning a plain term into a
// name/fullText Drive query.
func runDriveSearch(ctx context.Context, conn *gwcli.CmdG, term string, limit int, out *outputWriter) error {
	if strings.TrimSpace(term) == "" {
		return fmt.Errorf("a search term is required")
	}
	esc := strings.ReplaceAll(term, "'", `\'`)
	q := fmt.Sprintf("(name contains '%s' or fullText contains '%s') and trashed = false", esc, esc)
	files, err := driveList(ctx, conn, q, limit)
	if err != nil {
		return err
	}
	return writeDriveList(files, out)
}

// runDriveUpdate replaces the content of an existing Drive file with a local
// file. name, when set, also renames the file.
func runDriveUpdate(ctx context.Context, conn *gwcli.CmdG, ref, path, name string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}

	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}

	f, err := os.Open(expandPath(path))
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close()

	meta := &drive.File{}
	if name != "" {
		meta.Name = name
	}

	var mediaOpts []googleapi.MediaOption
	if opt := driveMediaOption(path); opt != nil {
		mediaOpts = append(mediaOpts, opt)
	}

	updated, err := svc.Files.Update(art.ID, meta).
		Media(f, mediaOpts...).
		SupportsAllDrives(true).
		Fields(driveWriteFields).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	return writeDriveFile("Updated", updated, out)
}
