package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

const (
	driveDocMime      = "application/vnd.google-apps.document"
	driveMarkdownMime = "text/markdown"
)

// driveToMdName is the name of the Markdown file migrated from a Doc: the
// Doc's name with a .md extension (not doubled when it already has one).
func driveToMdName(docName string) string {
	if strings.HasSuffix(strings.ToLower(docName), ".md") {
		return docName
	}
	return docName + ".md"
}

// runDriveToMd migrates a Google Doc to a native Markdown file: it exports the
// Doc as text/markdown and creates <name>.md in the Doc's parent folder (or
// folderRef). Doc comments do not carry over, so the open comment count is
// reported as droppedComments. With upsert an existing same-name Markdown file
// in the destination is replaced; with trashOriginal the Doc is trashed
// (reversible) after the new file is written.
func runDriveToMd(ctx context.Context, conn *gwcli.CmdG, ref, folderRef string, upsert, trashOriginal bool, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}

	doc, err := svc.Files.Get(art.ID).
		Fields("id,name,mimeType,parents").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	if doc.MimeType != driveDocMime {
		return fmt.Errorf("%q (%s) is %s, not a Google Doc; only Docs can be migrated to Markdown",
			doc.Name, doc.Id, doc.MimeType)
	}

	parent := ""
	if folderRef != "" {
		dest, err := resolveDriveRef(folderRef)
		if err != nil {
			return err
		}
		parent = dest.ID
	} else if len(doc.Parents) > 0 {
		parent = doc.Parents[0]
	}

	comments, err := listDriveComments(ctx, svc, doc.Id, false)
	if err != nil {
		return err
	}
	dropped := countOpenDriveComments(comments)

	data, _, err := fetchDriveArtifactFormat(ctx, conn, driveArtifact{ID: doc.Id, Title: doc.Name}, "md")
	if err != nil {
		return fmt.Errorf("failed to export %q as Markdown: %w", doc.Name, err)
	}

	name := driveToMdName(doc.Name)
	written, err := writeDriveMarkdown(ctx, svc, name, parent, data, upsert)
	if err != nil {
		return err
	}

	if dropped > 0 {
		out.writeWarning(fmt.Sprintf("%d unresolved comment(s) on %q were not carried over to %q "+
			"(Doc comments do not migrate); review them with `gwcli drive comments list %s`",
			dropped, doc.Name, written.Name, doc.Id))
	}

	trashed := false
	if trashOriginal {
		if _, err := svc.Files.Update(doc.Id, &drive.File{Trashed: true}).
			SupportsAllDrives(true).
			Fields("id").
			Context(ctx).
			Do(); err != nil {
			return fmt.Errorf("created %q (id %s) but failed to trash the original Doc %s: %w",
				written.Name, written.Id, doc.Id, wrapDriveErr(err))
		}
		trashed = true
	}

	if out.json {
		result := driveFileJSON(written)
		result["sourceId"] = doc.Id
		result["sourceName"] = doc.Name
		result["droppedComments"] = dropped
		result["sourceTrashed"] = trashed
		return out.writeJSON(result)
	}
	if err := writeDriveFile("Migrated", written, out); err != nil {
		return err
	}
	if trashed {
		out.writeMessage(fmt.Sprintf("Trashed original Doc: %s (id %s)", doc.Name, doc.Id))
	}
	return nil
}

// writeDriveMarkdown creates a native text/markdown file named name in parent,
// or with upsert replaces an existing same-name Markdown file there. The
// lookup is restricted to text/markdown so a Doc that is itself named "x.md"
// is never overwritten.
func writeDriveMarkdown(ctx context.Context, svc *drive.Service, name, parent string, data []byte, upsert bool) (*drive.File, error) {
	media := googleapi.ContentType(driveMarkdownMime)
	if upsert {
		existing, err := findDriveChildByName(ctx, svc, name, parent, driveMarkdownMime)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			updated, err := svc.Files.Update(existing.Id, &drive.File{}).
				Media(bytes.NewReader(data), media).
				SupportsAllDrives(true).
				Fields(driveWriteFields).
				Context(ctx).
				Do()
			if err != nil {
				return nil, wrapDriveErr(err)
			}
			return updated, nil
		}
	}
	meta := &drive.File{Name: name, MimeType: driveMarkdownMime}
	if parent != "" {
		meta.Parents = []string{parent}
	}
	created, err := svc.Files.Create(meta).
		Media(bytes.NewReader(data), media).
		SupportsAllDrives(true).
		Fields(driveWriteFields).
		Context(ctx).
		Do()
	if err != nil {
		return nil, wrapDriveErr(err)
	}
	return created, nil
}
