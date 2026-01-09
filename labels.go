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
		fmt.Fprintf(os.Stderr, "Warning: label '%s' not found in config\n", labelID)
		fmt.Fprintf(os.Stderr, "If you need to create labels, use gmailctl\n")
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
		fmt.Fprintf(os.Stderr, "Warning: label '%s' not found in config\n", labelID)
		fmt.Fprintf(os.Stderr, "If you need to create labels, use gmailctl\n")
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
