package main

import (
	"context"
	"fmt"

	"github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/tasks/v1"
)

// tasklistOutput represents a task list for JSON output.
type tasklistOutput struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Updated string `json:"updated,omitempty"`
}

// runTasklistsList lists all task lists for the authenticated user.
func runTasklistsList(ctx context.Context, conn *gwcli.CmdG, out *outputWriter) error {
	out.writeVerbose("Fetching task lists...")

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	resp, err := svc.Tasklists.List().Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to list task lists: %w", err)
	}

	if out.json {
		output := make([]tasklistOutput, len(resp.Items))
		for i, tl := range resp.Items {
			output[i] = tasklistOutput{
				ID:      tl.Id,
				Title:   tl.Title,
				Updated: tl.Updated,
			}
		}
		return out.writeJSON(output)
	}

	headers := []string{"TITLE", "ID", "UPDATED"}
	rows := make([][]string, len(resp.Items))
	for i, tl := range resp.Items {
		rows[i] = []string{tl.Title, tl.Id, formatTaskDate(tl.Updated)}
	}
	return out.writeTable(headers, rows)
}

// formatTaskDate formats an RFC3339 date string for display.
func formatTaskDate(rfc3339 string) string {
	if rfc3339 == "" {
		return ""
	}
	// Return just the date part for brevity
	if len(rfc3339) >= 10 {
		return rfc3339[:10]
	}
	return rfc3339
}

// runTasklistsCreate creates a new task list.
func runTasklistsCreate(ctx context.Context, conn *gwcli.CmdG, title string, out *outputWriter) error {
	if title == "" {
		return fmt.Errorf("task list title is required")
	}

	out.writeVerbose("Creating task list %q...", title)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	tl, err := svc.Tasklists.Insert(&tasks.TaskList{Title: title}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create task list: %w", err)
	}

	if out.json {
		return out.writeJSON(tasklistOutput{
			ID:      tl.Id,
			Title:   tl.Title,
			Updated: tl.Updated,
		})
	}

	out.writeMessage(fmt.Sprintf("Created task list %q (ID: %s)", tl.Title, tl.Id))
	return nil
}

// runTasklistsDelete deletes a task list.
func runTasklistsDelete(ctx context.Context, conn *gwcli.CmdG, tasklistID string, force bool, out *outputWriter) error {
	if tasklistID == "" {
		return fmt.Errorf("task list ID is required")
	}

	out.writeVerbose("Deleting task list %s...", tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	if err := svc.Tasklists.Delete(tasklistID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to delete task list: %w", err)
	}

	if out.json {
		return out.writeJSON(map[string]string{"deleted": tasklistID})
	}

	out.writeMessage(fmt.Sprintf("Deleted task list %s", tasklistID))
	return nil
}
