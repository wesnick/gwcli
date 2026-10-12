package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// driveFolderMime is the well-known mimeType Google Drive uses for folders.
const driveFolderMime = "application/vnd.google-apps.folder"

// driveWriteFields is the field mask requested on every write so the JSON
// result is composable (the next step can chain on id/webViewLink/parents
// without a search round-trip, and pass headRevisionId to the next guarded
// write).
const driveWriteFields = "id,name,mimeType,size,webViewLink,parents,headRevisionId,md5Checksum"

// driveGuardFields is the metadata read by checkDriveWriteGuard.
const driveGuardFields = "id,name,mimeType,modifiedTime,headRevisionId,md5Checksum"

// errDriveConflict marks a guarded write refused because the Drive file
// changed since the caller read it. Commands exit 4 on it (driveExitCode).
var errDriveConflict = errors.New("drive file changed since it was read")

// driveConflictError is the concrete error for a failed write guard.
type driveConflictError struct {
	msg string
}

func (e *driveConflictError) Error() string        { return e.msg }
func (e *driveConflictError) Is(target error) bool { return target == errDriveConflict }

// driveExitCode maps a Drive write error to the CLI exit code: 4 for a write
// guard conflict, 2 otherwise.
func driveExitCode(err error) int {
	if errors.Is(err, errDriveConflict) {
		return 4
	}
	return 2
}

// driveWriteGuard is the optimistic-concurrency precondition for a write:
// the file's current headRevisionId and/or md5Checksum must match. Drive v3
// has no conditional update, so this is check-then-write: racy, but it
// catches a person editing the file in Docs between an agent's pull and push.
type driveWriteGuard struct {
	Revision string
	MD5      string
}

func (g driveWriteGuard) empty() bool {
	return g.Revision == "" && g.MD5 == ""
}

// checkDriveWriteGuard fetches fileID's current revision/checksum and returns
// a *driveConflictError if they no longer match g. An empty guard is a no-op
// (no API call).
func checkDriveWriteGuard(ctx context.Context, svc *drive.Service, fileID string, g driveWriteGuard) error {
	if g.empty() {
		return nil
	}
	cur, err := svc.Files.Get(fileID).
		Fields(driveGuardFields).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	return compareDriveWriteGuard(cur, g)
}

// compareDriveWriteGuard checks the guard against already-fetched metadata.
func compareDriveWriteGuard(cur *drive.File, g driveWriteGuard) error {
	if strings.HasPrefix(cur.MimeType, "application/vnd.google-apps.") || cur.HeadRevisionId == "" {
		return fmt.Errorf("%q (%s) is a native Google file (%s), which has no "+
			"headRevisionId/md5Checksum; --if-revision/--if-md5 only apply to "+
			"uploaded files such as native Markdown", cur.Name, cur.Id, cur.MimeType)
	}
	repull := fmt.Sprintf("Re-pull it (gwcli drive export %s --output - --json), "+
		"merge your changes, then push again with --if-revision %s.", cur.Id, cur.HeadRevisionId)
	if g.Revision != "" && g.Revision != cur.HeadRevisionId {
		return &driveConflictError{msg: fmt.Sprintf(
			"%q (%s) changed since it was read: head revision is %s (expected %s), modified %s. %s",
			cur.Name, cur.Id, cur.HeadRevisionId, g.Revision, cur.ModifiedTime, repull)}
	}
	if g.MD5 != "" && !strings.EqualFold(g.MD5, cur.Md5Checksum) {
		return &driveConflictError{msg: fmt.Sprintf(
			"%q (%s) changed since it was read: md5 is %s (expected %s), head revision %s, modified %s. %s",
			cur.Name, cur.Id, cur.Md5Checksum, g.MD5, cur.HeadRevisionId, cur.ModifiedTime, repull)}
	}
	return nil
}

// driveConvertByExt maps a local file extension to the native Google-apps
// mimeType to convert into on upload (--convert). Anything not listed is
// uploaded as-is (no conversion). Markdown (.md/.markdown) is deliberately
// absent: Drive and Docs handle native Markdown files directly, and converting
// to a Doc and exporting back is lossy (it mangles YAML frontmatter and table
// alignment rows). Use --as doc to force conversion anyway.
var driveConvertByExt = map[string]string{
	".csv":  "application/vnd.google-apps.spreadsheet",
	".tsv":  "application/vnd.google-apps.spreadsheet",
	".xls":  "application/vnd.google-apps.spreadsheet",
	".xlsx": "application/vnd.google-apps.spreadsheet",
	".ods":  "application/vnd.google-apps.spreadsheet",
	".txt":  "application/vnd.google-apps.document",
	".rtf":  "application/vnd.google-apps.document",
	".doc":  "application/vnd.google-apps.document",
	".docx": "application/vnd.google-apps.document",
	".odt":  "application/vnd.google-apps.document",
	".html": "application/vnd.google-apps.document",
	".htm":  "application/vnd.google-apps.document",
	".ppt":  "application/vnd.google-apps.presentation",
	".pptx": "application/vnd.google-apps.presentation",
	".odp":  "application/vnd.google-apps.presentation",
}

// driveSourceMimeByExt maps a local file extension to the *source* content
// type to declare on the media upload. Drive's import converter keys the
// conversion off the uploaded media's MIME type, not the target metadata
// mimeType — so without this a .csv is content-sniffed as text/plain and
// imported as a Doc even when the target is google-apps.spreadsheet.
var driveSourceMimeByExt = map[string]string{
	".csv":      "text/csv",
	".tsv":      "text/tab-separated-values",
	".xls":      "application/vnd.ms-excel",
	".xlsx":     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ods":      "application/vnd.oasis.opendocument.spreadsheet",
	".txt":      "text/plain",
	".md":       "text/markdown",
	".markdown": "text/markdown",
	".rtf":      "application/rtf",
	".doc":      "application/msword",
	".docx":     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".odt":      "application/vnd.oasis.opendocument.text",
	".html":     "text/html",
	".htm":      "text/html",
	".ppt":      "application/vnd.ms-powerpoint",
	".pptx":     "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".odp":      "application/vnd.oasis.opendocument.presentation",
	".json":     "application/json",
}

// driveAsAliases maps the friendly --as values to native Google-apps
// mimeTypes. A raw "application/vnd.google-apps.*" value is also accepted
// verbatim by resolveDriveTargetMime.
var driveAsAliases = map[string]string{
	"doc":          "application/vnd.google-apps.document",
	"document":     "application/vnd.google-apps.document",
	"sheet":        "application/vnd.google-apps.spreadsheet",
	"spreadsheet":  "application/vnd.google-apps.spreadsheet",
	"slides":       "application/vnd.google-apps.presentation",
	"presentation": "application/vnd.google-apps.presentation",
	"drawing":      "application/vnd.google-apps.drawing",
	"form":         "application/vnd.google-apps.form",
}

// resolveDriveTargetMime turns an --as value into a native Google-apps
// mimeType. It accepts a friendly alias (doc/sheet/slides/...) or a raw
// "application/vnd.google-apps.*" type. Empty input yields "" (no override).
func resolveDriveTargetMime(as string) (string, error) {
	as = strings.TrimSpace(as)
	if as == "" {
		return "", nil
	}
	if m, ok := driveAsAliases[strings.ToLower(as)]; ok {
		return m, nil
	}
	if strings.HasPrefix(as, "application/vnd.google-apps.") {
		return as, nil
	}
	return "", fmt.Errorf("unknown --as value %q (use doc, sheet, slides, drawing, form, or a raw application/vnd.google-apps.* type)", as)
}

// driveUploadTargetMime returns the metadata mimeType to request for an
// upload of localPath: the --as override when set, else the --convert mapping
// by extension, else "" (keep the file as uploaded).
func driveUploadTargetMime(localPath string, convert bool, targetMimeOverride string) string {
	if targetMimeOverride != "" {
		return targetMimeOverride
	}
	if convert {
		return driveConvertByExt[strings.ToLower(filepath.Ext(localPath))]
	}
	return ""
}

// driveMediaOption returns the explicit source-content-type media option for
// localPath, or nil when the extension is unknown (let the library sniff).
func driveMediaOption(localPath string) googleapi.MediaOption {
	if m, ok := driveSourceMimeByExt[strings.ToLower(filepath.Ext(localPath))]; ok {
		return googleapi.ContentType(m)
	}
	return nil
}

// driveQuoteValue escapes a value for embedding inside a single-quoted Drive
// query string literal.
func driveQuoteValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `'`, `\'`)
	return v
}

// writeDriveFile emits a Drive file as the canonical write result: JSON with
// id + webViewLink (so callers can chain share/move/link without a search), or
// a one-line prose confirmation.
func writeDriveFile(verb string, f *drive.File, out *outputWriter) error {
	if out.json {
		return out.writeJSON(driveFileJSON(f))
	}
	out.writeMessage(fmt.Sprintf("%s: %s (id %s)", verb, f.Name, f.Id))
	return nil
}

// driveFileJSON is the composable JSON shape of a written Drive file.
// headRevisionId/md5Checksum are only set for uploaded (non-Google) files.
func driveFileJSON(f *drive.File) map[string]interface{} {
	m := map[string]interface{}{
		"id":          f.Id,
		"name":        f.Name,
		"mimeType":    f.MimeType,
		"size":        f.Size,
		"webViewLink": f.WebViewLink,
		"parents":     f.Parents,
	}
	if f.HeadRevisionId != "" {
		m["headRevisionId"] = f.HeadRevisionId
	}
	if f.Md5Checksum != "" {
		m["md5Checksum"] = f.Md5Checksum
	}
	return m
}

// driveSvc returns the connection's Drive client or an actionable error.
func driveSvc(conn *gwcli.CmdG) (*drive.Service, error) {
	svc := conn.DriveService()
	if svc == nil {
		return nil, fmt.Errorf("Drive service is not available for this connection. %s", driveScopeHelp)
	}
	return svc, nil
}

// findDriveChildByName looks up a single non-trashed file by exact name within
// a parent (parent "" means "root"). If mimeFilter is non-empty it is added as
// a `mimeType =` clause. Returns nil (no error) when nothing matches.
func findDriveChildByName(ctx context.Context, svc *drive.Service, name, parent, mimeFilter string) (*drive.File, error) {
	if parent == "" {
		parent = "root"
	}
	clauses := []string{
		fmt.Sprintf("name = '%s'", driveQuoteValue(name)),
		fmt.Sprintf("'%s' in parents", driveQuoteValue(parent)),
		"trashed = false",
	}
	if mimeFilter != "" {
		clauses = append(clauses, fmt.Sprintf("mimeType = '%s'", driveQuoteValue(mimeFilter)))
	}
	resp, err := svc.Files.List().
		Q(strings.Join(clauses, " and ")).
		Fields("files(" + driveWriteFields + ")").
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Corpora("allDrives").
		PageSize(1).
		Context(ctx).
		Do()
	if err != nil {
		return nil, wrapDriveErr(err)
	}
	if len(resp.Files) == 0 {
		return nil, nil
	}
	return resp.Files[0], nil
}

// ensureDriveFolder creates a folder named name under parent, or returns the
// existing one when dedupe is true and a same-name folder already exists in
// that parent. Idempotent by default so agent reruns don't fan out duplicates.
func ensureDriveFolder(ctx context.Context, svc *drive.Service, name, parent string, dedupe bool) (*drive.File, error) {
	if dedupe {
		existing, err := findDriveChildByName(ctx, svc, name, parent, driveFolderMime)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}
	meta := &drive.File{Name: name, MimeType: driveFolderMime}
	if parent != "" {
		meta.Parents = []string{parent}
	}
	created, err := svc.Files.Create(meta).
		SupportsAllDrives(true).
		Fields(driveWriteFields).
		Context(ctx).
		Do()
	if err != nil {
		return nil, wrapDriveErr(err)
	}
	return created, nil
}

// runDriveMkdir creates (or, by default, reuses) a Drive folder.
func runDriveMkdir(ctx context.Context, conn *gwcli.CmdG, name, parentRef string, noDedupe bool, out *outputWriter) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("a folder name is required")
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	parent := ""
	if parentRef != "" {
		art, err := resolveDriveRef(parentRef)
		if err != nil {
			return err
		}
		parent = art.ID
	}
	f, err := ensureDriveFolder(ctx, svc, name, parent, !noDedupe)
	if err != nil {
		return err
	}
	return writeDriveFile("Folder", f, out)
}

// runDriveMove reparents a Drive file: it is removed from all its current
// parents and added to the destination folder.
func runDriveMove(ctx context.Context, conn *gwcli.CmdG, ref, destRef string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	if destRef == "" {
		return fmt.Errorf("a destination folder (--folder) is required")
	}
	dest, err := resolveDriveRef(destRef)
	if err != nil {
		return err
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}

	cur, err := svc.Files.Get(art.ID).
		Fields("id,parents").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}

	updated, err := svc.Files.Update(art.ID, &drive.File{}).
		AddParents(dest.ID).
		RemoveParents(strings.Join(cur.Parents, ",")).
		SupportsAllDrives(true).
		Fields(driveWriteFields).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	return writeDriveFile("Moved", updated, out)
}

// runDriveRename changes only a file's name (content untouched).
func runDriveRename(ctx context.Context, conn *gwcli.CmdG, ref, name string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("a new name is required")
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	updated, err := svc.Files.Update(art.ID, &drive.File{Name: name}).
		SupportsAllDrives(true).
		Fields(driveWriteFields).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	return writeDriveFile("Renamed", updated, out)
}

// runDriveCopy duplicates a Drive file (template instantiation). name and
// folder override the copy's title and destination parent.
func runDriveCopy(ctx context.Context, conn *gwcli.CmdG, ref, name, folderRef string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	meta := &drive.File{}
	if name != "" {
		meta.Name = name
	}
	if folderRef != "" {
		dest, err := resolveDriveRef(folderRef)
		if err != nil {
			return err
		}
		meta.Parents = []string{dest.ID}
	}
	copied, err := svc.Files.Copy(art.ID, meta).
		SupportsAllDrives(true).
		Fields(driveWriteFields).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	return writeDriveFile("Copied", copied, out)
}

// runDriveRm trashes (default) or permanently deletes a Drive file. --force is
// required; --permanent skips the trash and is irreversible.
func runDriveRm(ctx context.Context, conn *gwcli.CmdG, ref string, permanent, force bool, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	if !force {
		return fmt.Errorf("refusing to delete without --force")
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}

	if permanent {
		if err := svc.Files.Delete(art.ID).
			SupportsAllDrives(true).
			Context(ctx).
			Do(); err != nil {
			return wrapDriveErr(err)
		}
		if out.json {
			return out.writeJSON(map[string]interface{}{"id": art.ID, "deleted": true, "permanent": true})
		}
		out.writeMessage(fmt.Sprintf("Permanently deleted: %s", art.ID))
		return nil
	}

	updated, err := svc.Files.Update(art.ID, &drive.File{Trashed: true}).
		SupportsAllDrives(true).
		Fields(driveWriteFields).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	if out.json {
		return out.writeJSON(map[string]interface{}{"id": updated.Id, "name": updated.Name, "trashed": true})
	}
	out.writeMessage(fmt.Sprintf("Trashed: %s (id %s)", updated.Name, updated.Id))
	return nil
}

// runDriveShare grants a permission on a Drive file. emailOrDomain meaning
// depends on permType: user/group -> email address, domain -> domain name,
// anyone -> ignored.
func runDriveShare(ctx context.Context, conn *gwcli.CmdG, ref, permType, role, emailOrDomain, message string, notify bool, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}

	if role == "" {
		role = "reader"
	}
	if permType == "" {
		permType = "user"
	}
	perm := &drive.Permission{Type: permType, Role: role}
	switch permType {
	case "user", "group":
		if emailOrDomain == "" {
			return fmt.Errorf("--email is required for type %q", permType)
		}
		perm.EmailAddress = emailOrDomain
	case "domain":
		if emailOrDomain == "" {
			return fmt.Errorf("--domain is required for type \"domain\"")
		}
		perm.Domain = emailOrDomain
	case "anyone":
		// no principal needed
	default:
		return fmt.Errorf("unknown permission type %q (use user, group, domain, or anyone)", permType)
	}

	call := svc.Permissions.Create(art.ID, perm).
		SupportsAllDrives(true).
		SendNotificationEmail(notify).
		Fields("id,type,role,emailAddress,domain").
		Context(ctx)
	if role == "owner" && permType == "user" {
		call = call.TransferOwnership(true)
	}
	if notify && message != "" {
		call = call.EmailMessage(message)
	}
	created, err := call.Do()
	if err != nil {
		return wrapDriveErr(err)
	}

	if out.json {
		return out.writeJSON(map[string]interface{}{
			"id":           created.Id,
			"type":         created.Type,
			"role":         created.Role,
			"emailAddress": created.EmailAddress,
			"domain":       created.Domain,
			"fileId":       art.ID,
		})
	}
	who := emailOrDomain
	if permType == "anyone" {
		who = "anyone"
	}
	out.writeMessage(fmt.Sprintf("Shared %s with %s as %s", art.ID, who, role))
	return nil
}

// runDriveLink ensures an anyone-with-the-link permission exists (unless
// --no-anyone) and prints the shareable webViewLink so it can be pasted into
// an email/message.
func runDriveLink(ctx context.Context, conn *gwcli.CmdG, ref, role string, noAnyone bool, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	if role == "" {
		role = "reader"
	}
	if !noAnyone {
		_, err := svc.Permissions.Create(art.ID, &drive.Permission{Type: "anyone", Role: role}).
			SupportsAllDrives(true).
			Context(ctx).
			Do()
		if err != nil {
			return wrapDriveErr(err)
		}
	}
	f, err := svc.Files.Get(art.ID).
		Fields("id,name,webViewLink").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return wrapDriveErr(err)
	}
	if out.json {
		return out.writeJSON(map[string]interface{}{
			"id":          f.Id,
			"name":        f.Name,
			"webViewLink": f.WebViewLink,
		})
	}
	out.writeMessage(f.WebViewLink)
	return nil
}

// runDrivePermissions lists who can access a Drive file (audit before sharing
// internal notes).
func runDrivePermissions(ctx context.Context, conn *gwcli.CmdG, ref string, out *outputWriter) error {
	art, err := resolveDriveRef(ref)
	if err != nil {
		return err
	}
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	var perms []*drive.Permission
	pageToken := ""
	for {
		call := svc.Permissions.List(art.ID).
			Fields("nextPageToken, permissions(id,type,role,emailAddress,domain,displayName)").
			SupportsAllDrives(true).
			PageSize(100).
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return wrapDriveErr(err)
		}
		perms = append(perms, resp.Permissions...)
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	if len(perms) == 0 {
		return out.WriteEmptyList("No permissions found")
	}
	if out.json {
		return out.writeJSON(perms)
	}
	headers := []string{"ID", "TYPE", "ROLE", "PRINCIPAL", "NAME"}
	rows := make([][]string, len(perms))
	for i, p := range perms {
		principal := p.EmailAddress
		if principal == "" {
			principal = p.Domain
		}
		if principal == "" && p.Type == "anyone" {
			principal = "anyone-with-link"
		}
		rows[i] = []string{p.Id, p.Type, p.Role, principal, p.DisplayName}
	}
	return out.writeTable(headers, rows)
}

// uploadOneFile uploads a single local file into parent. With convert it is
// converted to the native Google-apps type for its extension; with upsert an
// existing same-name file in the parent is replaced instead of duplicated,
// after checking guard against it (a non-empty guard requires that file).
func uploadOneFile(ctx context.Context, svc *drive.Service, localPath, parent, name string, convert, upsert bool, targetMimeOverride string, guard driveWriteGuard) (*drive.File, error) {
	fh, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", localPath, err)
	}
	defer fh.Close()

	if name == "" {
		name = filepath.Base(localPath)
	}
	targetMime := driveUploadTargetMime(localPath, convert, targetMimeOverride)

	var mediaOpts []googleapi.MediaOption
	if opt := driveMediaOption(localPath); opt != nil {
		mediaOpts = append(mediaOpts, opt)
	}

	if upsert {
		existing, err := findDriveChildByName(ctx, svc, name, parent, "")
		if err != nil {
			return nil, err
		}
		if existing == nil && !guard.empty() {
			return nil, &driveConflictError{msg: fmt.Sprintf(
				"no file named %q exists in the destination to check --if-revision/--if-md5 "+
					"against (it was moved, renamed or deleted since it was read)", name)}
		}
		if existing != nil {
			if err := checkDriveWriteGuard(ctx, svc, existing.Id, guard); err != nil {
				return nil, err
			}
			meta := &drive.File{}
			if targetMime != "" {
				meta.MimeType = targetMime
			}
			updated, err := svc.Files.Update(existing.Id, meta).
				Media(fh, mediaOpts...).
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

	meta := &drive.File{Name: name}
	if parent != "" {
		meta.Parents = []string{parent}
	}
	if targetMime != "" {
		meta.MimeType = targetMime
	}
	created, err := svc.Files.Create(meta).
		Media(fh, mediaOpts...).
		SupportsAllDrives(true).
		Fields(driveWriteFields).
		Context(ctx).
		Do()
	if err != nil {
		return nil, wrapDriveErr(err)
	}
	return created, nil
}

// uploadTree walks a local directory, mirroring its structure into Drive
// (folders created idempotently) and uploading every file. Returns the
// uploaded/updated files.
func uploadTree(ctx context.Context, svc *drive.Service, root, parent string, convert, upsert bool, targetMimeOverride string) ([]*drive.File, error) {
	// dirFolderID caches the Drive folder ID for each local directory so a
	// folder is only created/looked-up once.
	dirFolderID := map[string]string{}

	base := filepath.Base(filepath.Clean(root))
	topFolder, err := ensureDriveFolder(ctx, svc, base, parent, true)
	if err != nil {
		return nil, err
	}
	dirFolderID[filepath.Clean(root)] = topFolder.Id

	var results []*drive.File
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if _, ok := dirFolderID[filepath.Clean(path)]; ok {
				return nil
			}
			parentID := dirFolderID[filepath.Clean(filepath.Dir(path))]
			folder, err := ensureDriveFolder(ctx, svc, info.Name(), parentID, true)
			if err != nil {
				return err
			}
			dirFolderID[filepath.Clean(path)] = folder.Id
			return nil
		}
		parentID := dirFolderID[filepath.Clean(filepath.Dir(path))]
		f, err := uploadOneFile(ctx, svc, path, parentID, "", convert, upsert, targetMimeOverride, driveWriteGuard{})
		if err != nil {
			return err
		}
		results = append(results, f)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return results, nil
}

// runDriveUpload uploads one or more local files/directories to Drive.
// Directories are mirrored recursively; --convert converts by extension;
// --upsert replaces an existing same-name file in the destination instead of
// creating a duplicate (idempotent reruns); guard (--if-revision/--if-md5)
// makes that replacement conditional and needs --upsert and a single file.
func runDriveUpload(ctx context.Context, conn *gwcli.CmdG, paths []string, folderRef, name string, convert, upsert bool, as string, guard driveWriteGuard, out *outputWriter) error {
	svc, err := driveSvc(conn)
	if err != nil {
		return err
	}
	targetMimeOverride, err := resolveDriveTargetMime(as)
	if err != nil {
		return err
	}
	parent := ""
	if folderRef != "" {
		dest, err := resolveDriveRef(folderRef)
		if err != nil {
			return err
		}
		parent = dest.ID
	}
	if name != "" && len(paths) != 1 {
		return fmt.Errorf("--name can only be used with a single file")
	}
	if !guard.empty() {
		if !upsert {
			return fmt.Errorf("--if-revision/--if-md5 require --upsert (a new upload has nothing to compare against)")
		}
		if len(paths) != 1 {
			return fmt.Errorf("--if-revision/--if-md5 can only be used with a single file")
		}
	}

	var results []*drive.File
	for _, p := range paths {
		ep := expandPath(p)
		fi, err := os.Stat(ep)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", p, err)
		}
		if fi.IsDir() {
			if name != "" {
				return fmt.Errorf("--name cannot be used when uploading a directory")
			}
			if !guard.empty() {
				return fmt.Errorf("--if-revision/--if-md5 cannot be used when uploading a directory")
			}
			tree, err := uploadTree(ctx, svc, ep, parent, convert, upsert, targetMimeOverride)
			if err != nil {
				return err
			}
			results = append(results, tree...)
			continue
		}
		f, err := uploadOneFile(ctx, svc, ep, parent, name, convert, upsert, targetMimeOverride, guard)
		if err != nil {
			return err
		}
		results = append(results, f)
	}

	if out.json {
		if len(results) == 1 {
			return writeDriveFile("Uploaded", results[0], out)
		}
		items := make([]map[string]interface{}, len(results))
		for i, f := range results {
			items[i] = driveFileJSON(f)
		}
		return out.writeJSON(items)
	}
	for _, f := range results {
		out.writeMessage(fmt.Sprintf("Uploaded: %s (id %s)", f.Name, f.Id))
	}
	return nil
}
