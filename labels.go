package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli"
)

// labelListOutput is JSON output for labels
type labelListOutput struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	MessageListView string `json:"messageListVisibility,omitempty"`
	LabelListView   string `json:"labelListVisibility,omitempty"`
	Color           string `json:"color,omitempty"`
}

func runLabelsList(ctx context.Context, conn *gwcli.CmdG, systemOnly, userOnly bool, out *outputWriter) error {
	out.writeVerbose("Loading labels from config...")
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}

	labels := conn.Labels()
	out.writeVerbose("Loaded %d labels", len(labels))

	// Filter
	filtered := []*gwcli.Label{}
	for _, l := range labels {
		if l.Response == nil {
			continue
		}
		isSystem := l.Response.Type == "system"
		if systemOnly && !isSystem {
			continue
		}
		if userOnly && isSystem {
			continue
		}
		filtered = append(filtered, l)
	}

	if out.json {
		output := make([]labelListOutput, len(filtered))
		for i, l := range filtered {
			output[i] = labelListOutput{
				ID:              l.ID,
				Name:            l.Label,
				Type:            l.Response.Type,
				MessageListView: l.Response.MessageListVisibility,
				LabelListView:   l.Response.LabelListVisibility,
			}
			if l.Response.Color != nil {
				output[i].Color = l.Response.Color.BackgroundColor
			}
		}
		return out.writeJSON(output)
	}

	// Text output
	headers := []string{"NAME", "TYPE", "ID"}
	rows := make([][]string, len(filtered))
	for i, l := range filtered {
		rows[i] = []string{
			l.Label,
			l.Response.Type,
			l.ID,
		}
	}

	return out.writeTable(headers, rows)
}

func runLabelsCreate(ctx context.Context, conn *gwcli.CmdG, name, messageListVisibility, labelListVisibility string, out *outputWriter) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("label name is required")
	}

	out.writeVerbose("Loading labels to check for duplicates...")
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}
	for _, l := range conn.Labels() {
		if strings.EqualFold(l.Label, name) {
			return fmt.Errorf("label %q already exists (ID: %s)", l.Label, l.ID)
		}
	}

	out.writeVerbose("Creating label %q...", name)
	created, err := conn.CreateLabel(ctx, name, messageListVisibility, labelListVisibility)
	if err != nil {
		return fmt.Errorf("failed to create label: %w", err)
	}

	if out.json {
		o := labelListOutput{
			ID:              created.Id,
			Name:            created.Name,
			Type:            created.Type,
			MessageListView: created.MessageListVisibility,
			LabelListView:   created.LabelListVisibility,
		}
		if created.Color != nil {
			o.Color = created.Color.BackgroundColor
		}
		return out.writeJSON(o)
	}

	out.writeMessage(fmt.Sprintf("Created label %q (ID: %s)", created.Name, created.Id))
	return nil
}

func runLabelsUpdate(ctx context.Context, conn *gwcli.CmdG, labelRef, newName, messageListVisibility, labelListVisibility string, out *outputWriter) error {
	if newName == "" && messageListVisibility == "" && labelListVisibility == "" {
		return fmt.Errorf("nothing to update: provide --name, --message-list-visibility, or --label-list-visibility")
	}

	out.writeVerbose("Loading labels to resolve %q...", labelRef)
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}

	resolvedID, err := resolveLabelID(conn, labelRef)
	if err != nil {
		return err
	}
	out.writeVerbose("Resolved label %q to ID %q", labelRef, resolvedID)

	updated, err := conn.UpdateLabel(ctx, resolvedID, newName, messageListVisibility, labelListVisibility)
	if err != nil {
		return fmt.Errorf("failed to update label: %w", err)
	}

	if out.json {
		o := labelListOutput{
			ID:              updated.Id,
			Name:            updated.Name,
			Type:            updated.Type,
			MessageListView: updated.MessageListVisibility,
			LabelListView:   updated.LabelListVisibility,
		}
		if updated.Color != nil {
			o.Color = updated.Color.BackgroundColor
		}
		return out.writeJSON(o)
	}

	out.writeMessage(fmt.Sprintf("Updated label %q (ID: %s)", updated.Name, updated.Id))
	return nil
}

func runLabelsDelete(ctx context.Context, conn *gwcli.CmdG, labelRef string, force bool, out *outputWriter) error {
	if !force {
		return fmt.Errorf("refusing to delete without --force")
	}

	out.writeVerbose("Loading labels to resolve %q...", labelRef)
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}

	resolvedID, err := resolveLabelID(conn, labelRef)
	if err != nil {
		return err
	}

	// Guard against deleting Gmail system labels (INBOX, SENT, ...), which the
	// API rejects anyway — give a clearer error up front.
	for _, l := range conn.Labels() {
		if l.ID == resolvedID && l.Response != nil && l.Response.Type == "system" {
			return fmt.Errorf("cannot delete system label %q", l.Label)
		}
	}

	out.writeVerbose("Deleting label %q (ID: %s)...", labelRef, resolvedID)
	if err := conn.DeleteLabel(ctx, resolvedID); err != nil {
		return fmt.Errorf("failed to delete label: %w", err)
	}

	if out.json {
		return out.writeJSON(map[string]string{"status": "deleted", "id": resolvedID})
	}

	out.writeMessage(fmt.Sprintf("Deleted label (ID: %s)", resolvedID))
	return nil
}

func runLabelsApply(ctx context.Context, conn *gwcli.CmdG, labelID, messageID string, stdin bool, verbose bool, out *outputWriter) error {
	var ids []string
	var err error

	if stdin {
		ids, err = readIDsFromStdin()
		if err != nil {
			return err
		}
	} else {
		if messageID == "" {
			return fmt.Errorf("either provide --message or use --stdin")
		}
		ids = []string{messageID}
	}

	out.writeVerbose("Loading labels from config...")
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}

	labels := conn.Labels()
	out.writeVerbose("Loaded %d labels, resolving '%s'...", len(labels), labelID)

	resolvedID := labelID
	found := false
	for _, l := range labels {
		if strings.EqualFold(l.Label, labelID) || l.ID == labelID {
			resolvedID = l.ID
			found = true
			out.writeVerbose("Resolved label '%s' to ID '%s'", labelID, resolvedID)
			break
		}
	}

	if !found {
		out.writeVerbose("Label '%s' not found. Available labels:", labelID)
		for _, l := range labels {
			out.writeVerbose("  - %s (ID: %s)", l.Label, l.ID)
		}
		fmt.Fprintf(os.Stderr, "Warning: label '%s' not found\n", labelID)
	}

	// Batch operation
	bp := newBatchProcessor(len(ids), verbose)
	err = bp.process(ctx, ids, func(ctx context.Context, id string) error {
		return conn.BatchLabel(ctx, []string{id}, resolvedID)
	})

	if out.json {
		return out.writeJSON(map[string]int{
			"applied": bp.processed - len(bp.errors),
			"errors":  len(bp.errors),
		})
	}

	if len(ids) == 1 {
		out.writeMessage("Label applied")
	} else {
		bp.report(os.Stdout)
	}
	return err
}

func runLabelsRemove(ctx context.Context, conn *gwcli.CmdG, labelID, messageID string, stdin bool, verbose bool, out *outputWriter) error {
	var ids []string
	var err error

	if stdin {
		ids, err = readIDsFromStdin()
		if err != nil {
			return err
		}
	} else {
		if messageID == "" {
			return fmt.Errorf("either provide --message or use --stdin")
		}
		ids = []string{messageID}
	}

	out.writeVerbose("Loading labels from config...")
	if err := conn.LoadLabels(ctx, out.verbose); err != nil {
		return fmt.Errorf("failed to load labels: %w", err)
	}

	labels := conn.Labels()
	out.writeVerbose("Loaded %d labels, resolving '%s'...", len(labels), labelID)

	resolvedID := labelID
	found := false
	for _, l := range labels {
		if strings.EqualFold(l.Label, labelID) || l.ID == labelID {
			resolvedID = l.ID
			found = true
			out.writeVerbose("Resolved label '%s' to ID '%s'", labelID, resolvedID)
			break
		}
	}

	if !found {
		out.writeVerbose("Label '%s' not found. Available labels:", labelID)
		for _, l := range labels {
			out.writeVerbose("  - %s (ID: %s)", l.Label, l.ID)
		}
		fmt.Fprintf(os.Stderr, "Warning: label '%s' not found\n", labelID)
	}

	// Batch operation
	bp := newBatchProcessor(len(ids), verbose)
	err = bp.process(ctx, ids, func(ctx context.Context, id string) error {
		return conn.BatchUnlabel(ctx, []string{id}, resolvedID)
	})

	if out.json {
		return out.writeJSON(map[string]int{
			"removed": bp.processed - len(bp.errors),
			"errors":  len(bp.errors),
		})
	}

	if len(ids) == 1 {
		out.writeMessage("Label removed")
	} else {
		bp.report(os.Stdout)
	}
	return err
}
