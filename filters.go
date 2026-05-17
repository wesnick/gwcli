package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
	gmail "google.golang.org/api/gmail/v1"
)

// filtersCreateCmd holds the flags for `gwcli filters create`.
type filtersCreateCmd struct {
	From          string   `help:"Match sender (display name or email)"`
	To            string   `help:"Match recipient (to/cc/bcc)"`
	Subject       string   `help:"Match subject phrase"`
	Query         string   `help:"Match raw Gmail search query"`
	HasAttachment bool     `help:"Match only messages with attachments" name:"has-attachment"`
	AddLabel      []string `help:"Label name or ID to add (repeatable)" name:"add-label"`
	RemoveLabel   []string `help:"Label name or ID to remove (repeatable)" name:"remove-label"`
	Archive       bool     `help:"Skip the inbox (remove INBOX label)"`
	MarkRead      bool     `help:"Mark as read (remove UNREAD label)" name:"mark-read"`
	Star          bool     `help:"Star the message (add STARRED label)"`
	Important     bool     `help:"Mark important (add IMPORTANT label)"`
	Trash         bool     `help:"Move to trash (add TRASH label)"`
	Forward       string   `help:"Forward to this email address"`
}

// filterOutput represents a Gmail filter for JSON output, with label IDs
// resolved to their human-readable names.
type filterOutput struct {
	ID             string   `json:"id"`
	From           string   `json:"from,omitempty"`
	To             string   `json:"to,omitempty"`
	Subject        string   `json:"subject,omitempty"`
	Query          string   `json:"query,omitempty"`
	NegatedQuery   string   `json:"negatedQuery,omitempty"`
	HasAttachment  bool     `json:"hasAttachment,omitempty"`
	AddLabels      []string `json:"addLabels,omitempty"`
	RemoveLabels   []string `json:"removeLabels,omitempty"`
	AddLabelIDs    []string `json:"addLabelIds,omitempty"`
	RemoveLabelIDs []string `json:"removeLabelIds,omitempty"`
	Forward        string   `json:"forward,omitempty"`
}

// labelIDToName builds a map from label ID to label name using the
// connection's loaded label cache.
func labelIDToName(ctx context.Context, conn *gwcli.CmdG, out *outputWriter) (map[string]string, error) {
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return nil, fmt.Errorf("failed to load labels: %w", err)
	}
	m := make(map[string]string)
	for _, l := range conn.Labels() {
		m[l.ID] = l.Label
	}
	return m, nil
}

// resolveLabelID resolves a label name or ID to a Gmail label ID.
func resolveLabelID(conn *gwcli.CmdG, nameOrID string) (string, error) {
	for _, l := range conn.Labels() {
		if strings.EqualFold(l.Label, nameOrID) || l.ID == nameOrID {
			return l.ID, nil
		}
	}
	return "", fmt.Errorf("label %q not found (use a label name or ID; see 'gwcli labels list')", nameOrID)
}

func namesFor(ids []string, idToName map[string]string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		if n, ok := idToName[id]; ok {
			out[i] = n
		} else {
			out[i] = id
		}
	}
	return out
}

func toFilterOutput(f *gmail.Filter, idToName map[string]string) filterOutput {
	fo := filterOutput{ID: f.Id}
	if f.Criteria != nil {
		fo.From = f.Criteria.From
		fo.To = f.Criteria.To
		fo.Subject = f.Criteria.Subject
		fo.Query = f.Criteria.Query
		fo.NegatedQuery = f.Criteria.NegatedQuery
		fo.HasAttachment = f.Criteria.HasAttachment
	}
	if f.Action != nil {
		fo.AddLabelIDs = f.Action.AddLabelIds
		fo.RemoveLabelIDs = f.Action.RemoveLabelIds
		fo.AddLabels = namesFor(f.Action.AddLabelIds, idToName)
		fo.RemoveLabels = namesFor(f.Action.RemoveLabelIds, idToName)
		fo.Forward = f.Action.Forward
	}
	return fo
}

// runFiltersList lists all Gmail filters.
func runFiltersList(ctx context.Context, conn *gwcli.CmdG, out *outputWriter) error {
	out.writeVerbose("Fetching filters...")

	svc := conn.GmailService()
	if svc == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	idToName, err := labelIDToName(ctx, conn, out)
	if err != nil {
		return err
	}

	resp, err := svc.Users.Settings.Filters.List("me").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to list filters: %w", err)
	}

	if out.json {
		output := make([]filterOutput, len(resp.Filter))
		for i, f := range resp.Filter {
			output[i] = toFilterOutput(f, idToName)
		}
		return out.writeJSON(output)
	}

	if len(resp.Filter) == 0 {
		return out.WriteEmptyList("No filters found")
	}

	headers := []string{"ID", "FROM", "TO", "SUBJECT", "QUERY", "ADD", "REMOVE"}
	rows := make([][]string, len(resp.Filter))
	for i, f := range resp.Filter {
		fo := toFilterOutput(f, idToName)
		rows[i] = []string{
			fo.ID,
			fo.From,
			fo.To,
			fo.Subject,
			fo.Query,
			strings.Join(fo.AddLabels, ","),
			strings.Join(fo.RemoveLabels, ","),
		}
	}
	return out.writeTable(headers, rows)
}

// runFiltersGet shows a single filter's details.
func runFiltersGet(ctx context.Context, conn *gwcli.CmdG, filterID string, out *outputWriter) error {
	if filterID == "" {
		return fmt.Errorf("filter ID is required")
	}

	svc := conn.GmailService()
	if svc == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	idToName, err := labelIDToName(ctx, conn, out)
	if err != nil {
		return err
	}

	f, err := svc.Users.Settings.Filters.Get("me", filterID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get filter: %w", err)
	}

	fo := toFilterOutput(f, idToName)

	if out.json {
		return out.writeJSON(fo)
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", fo.ID},
		{"From", fo.From},
		{"To", fo.To},
		{"Subject", fo.Subject},
		{"Query", fo.Query},
		{"NegatedQuery", fo.NegatedQuery},
		{"HasAttachment", fmt.Sprintf("%t", fo.HasAttachment)},
		{"AddLabels", strings.Join(fo.AddLabels, ", ")},
		{"RemoveLabels", strings.Join(fo.RemoveLabels, ", ")},
		{"Forward", fo.Forward},
	}
	return out.writeTable(headers, rows)
}

// runFiltersCreate creates a new Gmail filter.
func runFiltersCreate(ctx context.Context, conn *gwcli.CmdG, c filtersCreateCmd, out *outputWriter) error {
	svc := conn.GmailService()
	if svc == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	criteria := &gmail.FilterCriteria{
		From:          c.From,
		To:            c.To,
		Subject:       c.Subject,
		Query:         c.Query,
		HasAttachment: c.HasAttachment,
	}
	if criteria.From == "" && criteria.To == "" && criteria.Subject == "" &&
		criteria.Query == "" && !criteria.HasAttachment {
		return fmt.Errorf("at least one match criterion is required (--from, --to, --subject, --query, --has-attachment)")
	}

	idToName, err := labelIDToName(ctx, conn, out)
	if err != nil {
		return err
	}

	addIDs := []string{}
	removeIDs := []string{}

	for _, name := range c.AddLabel {
		id, err := resolveLabelID(conn, name)
		if err != nil {
			return err
		}
		addIDs = append(addIDs, id)
	}
	for _, name := range c.RemoveLabel {
		id, err := resolveLabelID(conn, name)
		if err != nil {
			return err
		}
		removeIDs = append(removeIDs, id)
	}
	if c.Star {
		addIDs = append(addIDs, "STARRED")
	}
	if c.Important {
		addIDs = append(addIDs, "IMPORTANT")
	}
	if c.Trash {
		addIDs = append(addIDs, "TRASH")
	}
	if c.Archive {
		removeIDs = append(removeIDs, "INBOX")
	}
	if c.MarkRead {
		removeIDs = append(removeIDs, "UNREAD")
	}

	action := &gmail.FilterAction{
		AddLabelIds:    dedupe(addIDs),
		RemoveLabelIds: dedupe(removeIDs),
		Forward:        c.Forward,
	}
	if len(action.AddLabelIds) == 0 && len(action.RemoveLabelIds) == 0 && action.Forward == "" {
		return fmt.Errorf("at least one action is required (--add-label, --remove-label, --archive, --mark-read, --star, --important, --trash, --forward)")
	}

	out.writeVerbose("Creating filter...")
	created, err := svc.Users.Settings.Filters.Create("me", &gmail.Filter{
		Criteria: criteria,
		Action:   action,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create filter: %w", err)
	}

	if out.json {
		return out.writeJSON(toFilterOutput(created, idToName))
	}

	out.writeMessage(fmt.Sprintf("Created filter %s", created.Id))
	return nil
}

// runFiltersDelete deletes a Gmail filter.
func runFiltersDelete(ctx context.Context, conn *gwcli.CmdG, filterID string, force bool, out *outputWriter) error {
	if filterID == "" {
		return fmt.Errorf("filter ID is required")
	}
	if !force {
		return fmt.Errorf("refusing to delete filter %s without --force", filterID)
	}

	svc := conn.GmailService()
	if svc == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	out.writeVerbose("Deleting filter %s...", filterID)
	if err := svc.Users.Settings.Filters.Delete("me", filterID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to delete filter: %w", err)
	}

	if out.json {
		return out.writeJSON(map[string]string{"deleted": filterID})
	}

	out.writeMessage(fmt.Sprintf("Deleted filter %s", filterID))
	return nil
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
