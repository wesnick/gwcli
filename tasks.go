package main

import (
	"context"
	"fmt"

	"github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/tasks/v1"
)

// taskOutput represents a task for JSON output.
type taskOutput struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Notes     string `json:"notes,omitempty"`
	Status    string `json:"status"`
	Due       string `json:"due,omitempty"`
	Completed string `json:"completed,omitempty"`
	Parent    string `json:"parent,omitempty"`
	Position  string `json:"position,omitempty"`
}

// taskOutputFromTask converts a tasks.Task to taskOutput.
func taskOutputFromTask(t *tasks.Task) taskOutput {
	completed := ""
	if t.Completed != nil {
		completed = *t.Completed
	}
	return taskOutput{
		ID:        t.Id,
		Title:     t.Title,
		Notes:     t.Notes,
		Status:    t.Status,
		Due:       t.Due,
		Completed: completed,
		Parent:    t.Parent,
		Position:  t.Position,
	}
}

// runTasksList lists tasks in a task list.
func runTasksList(ctx context.Context, conn *gwcli.CmdG, tasklistID string, includeCompleted bool, out *outputWriter) error {
	if tasklistID == "" {
		return fmt.Errorf("task list ID is required")
	}

	out.writeVerbose("Fetching tasks from list %s...", tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	call := svc.Tasks.List(tasklistID).Context(ctx)
	if includeCompleted {
		call = call.ShowCompleted(true).ShowHidden(true)
	}

	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}

	if out.json {
		output := make([]taskOutput, len(resp.Items))
		for i, t := range resp.Items {
			output[i] = taskOutputFromTask(t)
		}
		return out.writeJSON(output)
	}

	headers := []string{"STATUS", "TITLE", "DUE", "ID"}
	rows := make([][]string, len(resp.Items))
	for i, t := range resp.Items {
		status := "[ ]"
		if t.Status == "completed" {
			status = "[x]"
		}
		rows[i] = []string{status, truncateString(t.Title, 50), formatTaskDate(t.Due), t.Id}
	}
	return out.writeTable(headers, rows)
}

// runTasksCreate creates a new task in a task list.
func runTasksCreate(ctx context.Context, conn *gwcli.CmdG, tasklistID, title, notes, due string, out *outputWriter) error {
	if tasklistID == "" {
		return fmt.Errorf("task list ID is required")
	}
	if title == "" {
		return fmt.Errorf("task title is required")
	}

	out.writeVerbose("Creating task %q in list %s...", title, tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	task := &tasks.Task{
		Title: title,
		Notes: notes,
	}

	if due != "" {
		task.Due = due
	}

	created, err := svc.Tasks.Insert(tasklistID, task).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	if out.json {
		return out.writeJSON(taskOutputFromTask(created))
	}

	out.writeMessage(fmt.Sprintf("Created task %q (ID: %s)", created.Title, created.Id))
	return nil
}

// runTasksRead gets details of a single task.
func runTasksRead(ctx context.Context, conn *gwcli.CmdG, tasklistID, taskID string, out *outputWriter) error {
	if tasklistID == "" {
		return fmt.Errorf("task list ID is required")
	}
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}

	out.writeVerbose("Fetching task %s from list %s...", taskID, tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	task, err := svc.Tasks.Get(tasklistID, taskID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	if out.json {
		return out.writeJSON(taskOutputFromTask(task))
	}

	// Text output with details
	status := "Pending"
	if task.Status == "completed" {
		status = "Completed"
	}

	out.writeMessage(fmt.Sprintf("Title: %s", task.Title))
	out.writeMessage(fmt.Sprintf("Status: %s", status))
	if task.Due != "" {
		out.writeMessage(fmt.Sprintf("Due: %s", formatTaskDate(task.Due)))
	}
	if task.Notes != "" {
		out.writeMessage(fmt.Sprintf("Notes: %s", task.Notes))
	}
	out.writeMessage(fmt.Sprintf("ID: %s", task.Id))

	return nil
}

// runTasksComplete marks a task as completed.
func runTasksComplete(ctx context.Context, conn *gwcli.CmdG, tasklistID, taskID string, out *outputWriter) error {
	if tasklistID == "" {
		return fmt.Errorf("task list ID is required")
	}
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}

	out.writeVerbose("Completing task %s in list %s...", taskID, tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	// Update task status to completed
	updated, err := svc.Tasks.Patch(tasklistID, taskID, &tasks.Task{
		Status: "completed",
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	if out.json {
		return out.writeJSON(taskOutputFromTask(updated))
	}

	out.writeMessage(fmt.Sprintf("Completed task %q", updated.Title))
	return nil
}

// runTasksDelete deletes a task.
func runTasksDelete(ctx context.Context, conn *gwcli.CmdG, tasklistID, taskID string, force bool, out *outputWriter) error {
	if tasklistID == "" {
		return fmt.Errorf("task list ID is required")
	}
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}

	out.writeVerbose("Deleting task %s from list %s...", taskID, tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	if err := svc.Tasks.Delete(tasklistID, taskID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	if out.json {
		return out.writeJSON(map[string]string{"deleted": taskID})
	}

	out.writeMessage(fmt.Sprintf("Deleted task %s", taskID))
	return nil
}
