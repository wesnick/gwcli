package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wesnick/gwcli/pkg/gwcli"
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
func runDriveExport(ctx context.Context, conn *gwcli.CmdG, ref, outputDir, outputFile string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}

	data, filename, err := fetchDriveArtifact(ctx, conn, art)
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
