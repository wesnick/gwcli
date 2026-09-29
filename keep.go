package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/googleapi"
	keep "google.golang.org/api/keep/v1"
)

// keepNotePrefix is the resource-name prefix of every Keep note
// ("notes/<id>"). Commands accept either the bare ID or the full name.
const keepNotePrefix = "notes/"

// keepClient is the subset of the Keep API gwcli uses. It exists so command
// handlers can be tested against a fake without Workspace credentials.
type keepClient interface {
	ListNotes(ctx context.Context, filter string, pageSize int64, pageToken string) (*keep.ListNotesResponse, error)
	GetNote(ctx context.Context, name string) (*keep.Note, error)
	CreateNote(ctx context.Context, note *keep.Note) (*keep.Note, error)
	DeleteNote(ctx context.Context, name string) error
}

// keepServiceClient adapts *keep.Service to keepClient.
type keepServiceClient struct {
	svc *keep.Service
}

func (c *keepServiceClient) ListNotes(ctx context.Context, filter string, pageSize int64, pageToken string) (*keep.ListNotesResponse, error) {
	call := c.svc.Notes.List().Context(ctx)
	if filter != "" {
		call = call.Filter(filter)
	}
	if pageSize > 0 {
		call = call.PageSize(pageSize)
	}
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	return call.Do()
}

func (c *keepServiceClient) GetNote(ctx context.Context, name string) (*keep.Note, error) {
	return c.svc.Notes.Get(name).Context(ctx).Do()
}

func (c *keepServiceClient) CreateNote(ctx context.Context, note *keep.Note) (*keep.Note, error) {
	return c.svc.Notes.Create(note).Context(ctx).Do()
}

func (c *keepServiceClient) DeleteNote(ctx context.Context, name string) error {
	_, err := c.svc.Notes.Delete(name).Context(ctx).Do()
	return err
}

// getKeepClient builds a Keep client. Keep is service-account-only (domain-
// wide delegation), so this deliberately bypasses getConnection: it never
// initializes Gmail or reads token.json, and rejects OAuth credentials with
// an actionable error. readOnly lets read commands fall back to the
// keep.readonly scope when that is all the admin authorized.
func getKeepClient(configDir, userEmail string, readOnly bool) (keepClient, error) {
	svc, err := gwcli.NewKeepService(context.Background(), configDir, userEmail, readOnly)
	if err != nil {
		return nil, err
	}
	return &keepServiceClient{svc: svc}, nil
}

// keepListItemOutput is one checklist item (with at most one level of
// nesting, per the Keep API).
type keepListItemOutput struct {
	Text     string               `json:"text"`
	Checked  bool                 `json:"checked"`
	Children []keepListItemOutput `json:"children,omitempty"`
}

// keepAttachmentOutput describes a note attachment (metadata only).
type keepAttachmentOutput struct {
	Name      string   `json:"name"`
	MimeTypes []string `json:"mimeTypes,omitempty"`
}

// keepNoteOutput is the JSON shape of a Keep note. `id` is the bare note ID
// accepted by `keep get`/`keep delete`; `name` is the API resource name.
type keepNoteOutput struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Title       string                 `json:"title"`
	Type        string                 `json:"type"` // "text" or "checklist"
	Text        string                 `json:"text,omitempty"`
	Items       []keepListItemOutput   `json:"items,omitempty"`
	CreateTime  string                 `json:"createTime,omitempty"`
	UpdateTime  string                 `json:"updateTime,omitempty"`
	Trashed     bool                   `json:"trashed"`
	TrashTime   string                 `json:"trashTime,omitempty"`
	Attachments []keepAttachmentOutput `json:"attachments,omitempty"`
}

// keepNoteName normalizes a note reference (bare ID or "notes/<id>") to the
// API resource name.
func keepNoteName(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	id := strings.TrimPrefix(ref, keepNotePrefix)
	if id == "" || strings.Contains(id, "/") {
		return "", fmt.Errorf("invalid note ID %q (expected <id> or notes/<id>)", ref)
	}
	return keepNotePrefix + id, nil
}

func keepNoteType(n *keep.Note) string {
	if n.Body != nil && n.Body.List != nil {
		return "checklist"
	}
	return "text"
}

func convertKeepItems(items []*keep.ListItem) []keepListItemOutput {
	if len(items) == 0 {
		return nil
	}
	out := make([]keepListItemOutput, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		o := keepListItemOutput{Checked: it.Checked, Children: convertKeepItems(it.ChildListItems)}
		if it.Text != nil {
			o.Text = it.Text.Text
		}
		out = append(out, o)
	}
	return out
}

func convertKeepNote(n *keep.Note) keepNoteOutput {
	o := keepNoteOutput{
		ID:         strings.TrimPrefix(n.Name, keepNotePrefix),
		Name:       n.Name,
		Title:      n.Title,
		Type:       keepNoteType(n),
		CreateTime: n.CreateTime,
		UpdateTime: n.UpdateTime,
		Trashed:    n.Trashed,
		TrashTime:  n.TrashTime,
	}
	if n.Body != nil {
		if n.Body.Text != nil {
			o.Text = n.Body.Text.Text
		}
		if n.Body.List != nil {
			o.Items = convertKeepItems(n.Body.List.ListItems)
		}
	}
	for _, a := range n.Attachments {
		if a != nil {
			o.Attachments = append(o.Attachments, keepAttachmentOutput{Name: a.Name, MimeTypes: a.MimeType})
		}
	}
	return o
}

// keepScopeHelp explains how to fix a domain-wide-delegation scope failure.
const keepScopeHelp = "Google Keep access denied: the service account is not " +
	"authorized for the Keep scope. In the Workspace Admin console " +
	"(Security > API controls > Domain-wide delegation), authorize the " +
	"service account's numeric Client ID (client_id in credentials.json) for " +
	"https://www.googleapis.com/auth/keep (read-only commands also accept " +
	"https://www.googleapis.com/auth/keep.readonly; create/delete need the " +
	"full keep scope), and make sure --user is a user in that Workspace domain"

// wrapKeepErr turns Keep auth/enablement failures into actionable messages.
func wrapKeepErr(err error) error {
	if err == nil {
		return nil
	}
	if gwcli.IsUnauthorizedClient(err) {
		return fmt.Errorf("%s (underlying: %w)", keepScopeHelp, err)
	}
	msg := err.Error()
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) && apiErr.Code == 403 {
		for _, sig := range []string{"SERVICE_DISABLED", "has not been used", "is disabled"} {
			if strings.Contains(msg, sig) {
				return fmt.Errorf("the Google Keep API is not enabled for the "+
					"service account's Cloud project; enable keep.googleapis.com "+
					"in the Google Cloud console (underlying: %w)", err)
			}
		}
		for _, sig := range []string{"ACCESS_TOKEN_SCOPE_INSUFFICIENT", "insufficient authentication scopes", "insufficientPermissions"} {
			if strings.Contains(msg, sig) {
				return fmt.Errorf("%s (underlying: %w)", keepScopeHelp, err)
			}
		}
	}
	if strings.Contains(msg, "unauthorized_client") {
		return fmt.Errorf("%s (underlying: %w)", keepScopeHelp, err)
	}
	return err
}

// keepMaxPageSize bounds a single List page; the server may return fewer.
const keepMaxPageSize = 100

// runKeepList lists notes, paginating until limit (0 = no limit) is reached.
func runKeepList(ctx context.Context, client keepClient, filter string, limit int, out *outputWriter) error {
	if limit < 0 {
		return fmt.Errorf("--limit must be >= 0")
	}
	out.writeVerbose("Listing Keep notes (filter=%q, limit=%d)...", filter, limit)

	var notes []*keep.Note
	pageToken := ""
	for {
		pageSize := int64(keepMaxPageSize)
		if limit > 0 && limit-len(notes) < keepMaxPageSize {
			pageSize = int64(limit - len(notes))
		}
		resp, err := client.ListNotes(ctx, filter, pageSize, pageToken)
		if err != nil {
			return fmt.Errorf("failed to list notes: %w", wrapKeepErr(err))
		}
		notes = append(notes, resp.Notes...)
		if limit > 0 && len(notes) >= limit {
			notes = notes[:limit]
			break
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	if len(notes) == 0 {
		return out.WriteEmptyList("No notes found")
	}

	if out.json {
		result := make([]keepNoteOutput, len(notes))
		for i, n := range notes {
			result[i] = convertKeepNote(n)
		}
		return out.writeJSON(result)
	}

	headers := []string{"ID", "TYPE", "TITLE", "UPDATED"}
	rows := make([][]string, len(notes))
	for i, n := range notes {
		o := convertKeepNote(n)
		title := o.Title
		if title == "" {
			title = keepPreview(o)
		}
		rows[i] = []string{o.ID, o.Type, truncateString(title, 60), formatTaskDate(o.UpdateTime)}
	}
	return out.writeTable(headers, rows)
}

// keepPreview returns a one-line preview for an untitled note.
func keepPreview(o keepNoteOutput) string {
	s := o.Text
	if o.Type == "checklist" && len(o.Items) > 0 {
		s = o.Items[0].Text
	}
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "(untitled)"
	}
	return "(" + truncateString(s, 58) + ")"
}

// runKeepGet shows one note, including its text body or checklist items.
func runKeepGet(ctx context.Context, client keepClient, ref string, out *outputWriter) error {
	name, err := keepNoteName(ref)
	if err != nil {
		return err
	}
	out.writeVerbose("Fetching Keep note %s...", name)

	n, err := client.GetNote(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get note: %w", wrapKeepErr(err))
	}
	o := convertKeepNote(n)

	if out.json {
		return out.writeJSON(o)
	}

	out.writeMessage(formatKeepNote(o))
	return nil
}

// formatKeepNote renders a note as human-readable text.
func formatKeepNote(o keepNoteOutput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title:   %s\n", o.Title)
	fmt.Fprintf(&b, "ID:      %s\n", o.ID)
	fmt.Fprintf(&b, "Type:    %s\n", o.Type)
	if o.CreateTime != "" {
		fmt.Fprintf(&b, "Created: %s\n", o.CreateTime)
	}
	if o.UpdateTime != "" {
		fmt.Fprintf(&b, "Updated: %s\n", o.UpdateTime)
	}
	if o.Trashed {
		fmt.Fprintf(&b, "Trashed: %s\n", o.TrashTime)
	}
	for _, a := range o.Attachments {
		fmt.Fprintf(&b, "Attachment: %s %s\n", a.Name, strings.Join(a.MimeTypes, ","))
	}
	b.WriteString("\n")
	if o.Type == "checklist" {
		writeKeepItems(&b, o.Items, "")
	} else {
		b.WriteString(o.Text)
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeKeepItems(b *strings.Builder, items []keepListItemOutput, indent string) {
	for _, it := range items {
		mark := " "
		if it.Checked {
			mark = "x"
		}
		fmt.Fprintf(b, "%s[%s] %s\n", indent, mark, it.Text)
		writeKeepItems(b, it.Children, indent+"    ")
	}
}

// keepCreateOptions holds the flags for `keep create`.
type keepCreateOptions struct {
	title     string
	text      string
	checklist []string
}

// buildKeepNote validates create options and assembles the API payload.
// Exactly one of text or checklist must be set; text "-" reads stdin.
func buildKeepNote(opts keepCreateOptions, stdin io.Reader) (*keep.Note, error) {
	hasText := opts.text != ""
	hasList := len(opts.checklist) > 0
	if hasText == hasList {
		return nil, fmt.Errorf("specify exactly one of --text or --checklist")
	}

	note := &keep.Note{Title: opts.title}
	if hasText {
		text := opts.text
		if text == "-" {
			data, err := io.ReadAll(stdin)
			if err != nil {
				return nil, fmt.Errorf("reading note text from stdin: %w", err)
			}
			text = strings.TrimRight(string(data), "\n")
			if strings.TrimSpace(text) == "" {
				return nil, fmt.Errorf("note text from stdin is empty")
			}
		}
		note.Body = &keep.Section{Text: &keep.TextContent{Text: text}}
		return note, nil
	}

	items := make([]*keep.ListItem, 0, len(opts.checklist))
	for _, raw := range opts.checklist {
		if strings.TrimSpace(raw) == "" {
			return nil, fmt.Errorf("checklist items must not be empty")
		}
		items = append(items, &keep.ListItem{Text: &keep.TextContent{Text: raw}})
	}
	note.Body = &keep.Section{List: &keep.ListContent{ListItems: items}}
	return note, nil
}

// runKeepCreate creates a text note or a checklist note.
func runKeepCreate(ctx context.Context, client keepClient, opts keepCreateOptions, stdin io.Reader, out *outputWriter) error {
	note, err := buildKeepNote(opts, stdin)
	if err != nil {
		return err
	}
	out.writeVerbose("Creating Keep note %q...", opts.title)

	created, err := client.CreateNote(ctx, note)
	if err != nil {
		return fmt.Errorf("failed to create note: %w", wrapKeepErr(err))
	}
	o := convertKeepNote(created)

	if out.json {
		return out.writeJSON(o)
	}
	out.writeMessage(fmt.Sprintf("Created %s note %q (ID: %s)", o.Type, o.Title, o.ID))
	return nil
}

// runKeepDelete permanently deletes a note. The Keep API has no trash for
// API deletes, so --force is required (non-interactive rule #3).
func runKeepDelete(ctx context.Context, client keepClient, ref string, force bool, out *outputWriter) error {
	name, err := keepNoteName(ref)
	if err != nil {
		return err
	}
	if !force {
		return fmt.Errorf("refusing to delete note %s without --force (deletion is permanent)", name)
	}
	out.writeVerbose("Deleting Keep note %s...", name)

	if err := client.DeleteNote(ctx, name); err != nil {
		return fmt.Errorf("failed to delete note: %w", wrapKeepErr(err))
	}

	id := strings.TrimPrefix(name, keepNotePrefix)
	if out.json {
		return out.writeJSON(map[string]string{"status": "deleted", "id": id})
	}
	out.writeMessage(fmt.Sprintf("Deleted note %s", id))
	return nil
}
