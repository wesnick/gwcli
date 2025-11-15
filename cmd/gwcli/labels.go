package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/wesnick/cmdg/pkg/cmdg"
	"google.golang.org/api/gmail/v1"
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

// runLabelsList lists all labels
func runLabelsList(ctx context.Context, conn *cmdg.CmdG, systemOnly, userOnly bool, out *outputWriter) error {
	labels := conn.Labels()

	// Filter
	filtered := []*cmdg.Label{}
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

// runLabelsCreate creates a new label
func runLabelsCreate(ctx context.Context, conn *cmdg.CmdG, name, color string, out *outputWriter) error {
	label := &gmail.Label{
		Name:                  name,
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
	}

	if color != "" {
		label.Color = &gmail.LabelColor{
			BackgroundColor: color,
		}
	}

	created, err := conn.GmailService().Users.Labels.Create("me", label).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create label: %w", err)
	}

	if out.json {
		return out.writeJSON(map[string]string{
			"id":   created.Id,
			"name": created.Name,
		})
	}

	out.writeMessage(fmt.Sprintf("Label created: %s (ID: %s)", created.Name, created.Id))
	return nil
}

// runLabelsDelete deletes a label
func runLabelsDelete(ctx context.Context, conn *cmdg.CmdG, labelID string, force bool, out *outputWriter) error {
	if !force {
		return fmt.Errorf("--force flag must be set to true to delete a label")
	}
	// Resolve label name to ID
	labels := conn.Labels()

	resolvedID := labelID
	for _, l := range labels {
		if strings.EqualFold(l.Label, labelID) || l.ID == labelID {
			resolvedID = l.ID
			break
		}
	}

	if err := conn.GmailService().Users.Labels.Delete("me", resolvedID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to delete label: %w", err)
	}

	if out.json {
		return out.writeJSON(map[string]string{"deleted": resolvedID})
	}

	out.writeMessage(fmt.Sprintf("Label deleted: %s", resolvedID))
	return nil
}

// runLabelsApply applies a label to messages
func runLabelsApply(ctx context.Context, conn *cmdg.CmdG, labelID, messageID string, stdin bool, verbose bool, out *outputWriter) error {
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

	// Resolve label
	labels := conn.Labels()

	resolvedID := labelID
	for _, l := range labels {
		if strings.EqualFold(l.Label, labelID) || l.ID == labelID {
			resolvedID = l.ID
			break
		}
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

// runLabelsRemove removes a label from messages
func runLabelsRemove(ctx context.Context, conn *cmdg.CmdG, labelID, messageID string, stdin bool, verbose bool, out *outputWriter) error {
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

	// Resolve label
	labels := conn.Labels()

	resolvedID := labelID
	for _, l := range labels {
		if strings.EqualFold(l.Label, labelID) || l.ID == labelID {
			resolvedID = l.ID
			break
		}
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
