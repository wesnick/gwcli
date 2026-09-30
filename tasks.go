package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/tasks/v1"
)

// defaultTasklistID is the Tasks API alias for the user's primary task list.
const defaultTasklistID = "@default"

// Task status values used by the Tasks API.
const (
	taskStatusNeedsAction = "needsAction"
	taskStatusCompleted   = "completed"
)

// tasksPageSize is the Tasks API maximum for tasks.list / tasklists.list.
const tasksPageSize = 100

// taskDueLayout is how due dates are sent to the Tasks API. The API only
// records the date; the time portion is discarded, so it is always midnight UTC.
const taskDueLayout = "2006-01-02T15:04:05.000Z"

// taskOutput represents a task for JSON output.
type taskOutput struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Notes       string `json:"notes,omitempty"`
	Status      string `json:"status"`
	Due         string `json:"due,omitempty"`
	Completed   string `json:"completed,omitempty"`
	Parent      string `json:"parent,omitempty"`
	Position    string `json:"position,omitempty"`
	Updated     string `json:"updated,omitempty"`
	WebViewLink string `json:"webViewLink,omitempty"`
}

// taskDetailOutput is the `tasks get` JSON shape: the task plus its subtasks.
type taskDetailOutput struct {
	taskOutput
	Subtasks []taskOutput `json:"subtasks,omitempty"`
}

// taskOutputFromTask converts a tasks.Task to taskOutput.
func taskOutputFromTask(t *tasks.Task) taskOutput {
	completed := ""
	if t.Completed != nil {
		completed = *t.Completed
	}
	return taskOutput{
		ID:          t.Id,
		Title:       t.Title,
		Notes:       t.Notes,
		Status:      t.Status,
		Due:         t.Due,
		Completed:   completed,
		Parent:      t.Parent,
		Position:    t.Position,
		Updated:     t.Updated,
		WebViewLink: t.WebViewLink,
	}
}

// resolveTasklistID returns the task list to operate on, defaulting to the
// user's primary list (@default) when none was given.
func resolveTasklistID(id string) string {
	if id = strings.TrimSpace(id); id != "" {
		return id
	}
	return defaultTasklistID
}

// resolveTasklistArg merges an optional positional task list ID with the
// --list-id flag. Both may be given only if they agree.
func resolveTasklistArg(positional, flag string) (string, error) {
	if positional != "" && flag != "" && positional != flag {
		return "", fmt.Errorf("task list given twice (%q and --list-id %q); use --list-id", positional, flag)
	}
	if positional != "" {
		return resolveTasklistID(positional), nil
	}
	return resolveTasklistID(flag), nil
}

// resolveTaskRef resolves the positional arguments of a single-task command
// into (tasklistID, taskID). The current form is `<task-id> [--list-id <id>]`;
// the legacy form `<tasklist-id> <task-id>` is still accepted.
func resolveTaskRef(args []string, listFlag string) (string, string, error) {
	switch len(args) {
	case 1:
		if args[0] == "" {
			return "", "", fmt.Errorf("task ID is required")
		}
		return resolveTasklistID(listFlag), args[0], nil
	case 2:
		listID, err := resolveTasklistArg(args[0], listFlag)
		if err != nil {
			return "", "", err
		}
		if args[1] == "" {
			return "", "", fmt.Errorf("task ID is required")
		}
		return listID, args[1], nil
	case 0:
		return "", "", fmt.Errorf("task ID is required")
	default:
		return "", "", fmt.Errorf("expected <task-id> (or legacy <tasklist-id> <task-id>), got %d arguments", len(args))
	}
}

// parseTaskDate parses a due date given as YYYY-MM-DD or RFC3339 and returns
// midnight UTC of that calendar date. For RFC3339 input the date is taken as
// written (in its own offset), since the Tasks API only stores the date and a
// UTC conversion could shift it by a day.
func parseTaskDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: expected YYYY-MM-DD or RFC3339 (e.g. 2026-10-01T00:00:00Z)", s)
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
}

// normalizeTaskDue converts a user-supplied due date into the RFC3339
// timestamp the Tasks API expects (e.g. 2026-10-01T00:00:00.000Z).
func normalizeTaskDue(s string) (string, error) {
	t, err := parseTaskDate(s)
	if err != nil {
		return "", err
	}
	return t.Format(taskDueLayout), nil
}

// taskDueRange turns the list filters (--due for a single day, or
// --due-min/--due-max as an inclusive range) into dueMin/dueMax API values.
// The upper bound is the last second of the day so it is inclusive
// regardless of how the API treats the boundary.
func taskDueRange(due, dueMin, dueMax string) (string, string, error) {
	if due != "" && (dueMin != "" || dueMax != "") {
		return "", "", fmt.Errorf("--due cannot be combined with --due-min/--due-max")
	}
	if due != "" {
		dueMin, dueMax = due, due
	}
	var lo, hi string
	if dueMin != "" {
		t, err := parseTaskDate(dueMin)
		if err != nil {
			return "", "", err
		}
		lo = t.Format(time.RFC3339)
	}
	if dueMax != "" {
		t, err := parseTaskDate(dueMax)
		if err != nil {
			return "", "", err
		}
		hi = t.Add(24*time.Hour - time.Second).Format(time.RFC3339)
	}
	if lo != "" && hi != "" && lo > hi {
		return "", "", fmt.Errorf("--due-min %s is after --due-max %s", dueMin, dueMax)
	}
	return lo, hi, nil
}

// orderTasks sorts tasks by position with each subtask placed directly after
// its parent. It returns the ordered tasks and, for each, whether it is a
// subtask of a task in the slice. Subtasks whose parent is not present (e.g.
// the parent is completed and hidden) are listed as top-level tasks.
func orderTasks(items []*tasks.Task) ([]*tasks.Task, []bool) {
	present := make(map[string]bool, len(items))
	for _, t := range items {
		present[t.Id] = true
	}
	children := make(map[string][]*tasks.Task)
	var roots []*tasks.Task
	for _, t := range items {
		if t.Parent != "" && present[t.Parent] {
			children[t.Parent] = append(children[t.Parent], t)
		} else {
			roots = append(roots, t)
		}
	}
	byPosition := func(s []*tasks.Task) {
		sort.SliceStable(s, func(i, j int) bool { return s[i].Position < s[j].Position })
	}
	byPosition(roots)

	ordered := make([]*tasks.Task, 0, len(items))
	isChild := make([]bool, 0, len(items))
	for _, r := range roots {
		ordered = append(ordered, r)
		isChild = append(isChild, false)
		kids := children[r.Id]
		byPosition(kids)
		for _, k := range kids {
			ordered = append(ordered, k)
			isChild = append(isChild, true)
		}
	}
	return ordered, isChild
}

// taskStatusBox renders the table status indicator.
func taskStatusBox(status string) string {
	if status == taskStatusCompleted {
		return "[x]"
	}
	return "[ ]"
}

// tasksListOptions holds the `tasks list` flags.
type tasksListOptions struct {
	tasklistID    string
	showCompleted bool
	due           string
	dueMin        string
	dueMax        string
	limit         int
}

// fetchTasks pages through tasks.list, stopping once limit tasks are
// collected (limit <= 0 means no cap).
func fetchTasks(ctx context.Context, call *tasks.TasksListCall, limit int) ([]*tasks.Task, error) {
	var items []*tasks.Task
	errDone := fmt.Errorf("limit reached")
	err := call.MaxResults(tasksPageSize).Pages(ctx, func(page *tasks.Tasks) error {
		for _, t := range page.Items {
			items = append(items, t)
			if limit > 0 && len(items) >= limit {
				return errDone
			}
		}
		return nil
	})
	if err != nil && err != errDone {
		return nil, err
	}
	return items, nil
}

// runTasksList lists tasks in a task list.
func runTasksList(ctx context.Context, conn *gwcli.CmdG, opts tasksListOptions, out *outputWriter) error {
	tasklistID := resolveTasklistID(opts.tasklistID)
	dueMin, dueMax, err := taskDueRange(opts.due, opts.dueMin, opts.dueMax)
	if err != nil {
		return err
	}

	out.writeVerbose("Fetching tasks from list %s...", tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	// showCompleted defaults to true in the API, so it must be turned off
	// explicitly. Tasks completed in the Google apps are also hidden, so
	// showing completed tasks needs showHidden too.
	call := svc.Tasks.List(tasklistID).ShowCompleted(opts.showCompleted)
	if opts.showCompleted {
		call = call.ShowHidden(true)
	}
	if dueMin != "" {
		call = call.DueMin(dueMin)
	}
	if dueMax != "" {
		call = call.DueMax(dueMax)
	}

	items, err := fetchTasks(ctx, call, opts.limit)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %w", err)
	}
	ordered, isChild := orderTasks(items)

	if out.json {
		output := make([]taskOutput, len(ordered))
		for i, t := range ordered {
			output[i] = taskOutputFromTask(t)
		}
		return out.writeJSON(output)
	}

	headers := []string{"STATUS", "TITLE", "DUE", "ID"}
	rows := make([][]string, len(ordered))
	for i, t := range ordered {
		title := truncateString(t.Title, 50)
		if isChild[i] {
			title = "  ↳ " + truncateString(t.Title, 46)
		}
		rows[i] = []string{taskStatusBox(t.Status), title, formatTaskDate(t.Due), t.Id}
	}
	return out.writeTable(headers, rows)
}

// taskCreateOptions holds the `tasks create` flags.
type taskCreateOptions struct {
	title  string
	notes  string
	due    string
	parent string
}

// runTasksCreate creates a new task in a task list.
func runTasksCreate(ctx context.Context, conn *gwcli.CmdG, tasklistID string, opts taskCreateOptions, out *outputWriter) error {
	tasklistID = resolveTasklistID(tasklistID)
	if strings.TrimSpace(opts.title) == "" {
		return fmt.Errorf("task title is required")
	}

	task := &tasks.Task{
		Title: opts.title,
		Notes: opts.notes,
	}
	if opts.due != "" {
		due, err := normalizeTaskDue(opts.due)
		if err != nil {
			return err
		}
		task.Due = due
	}

	out.writeVerbose("Creating task %q in list %s...", opts.title, tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	call := svc.Tasks.Insert(tasklistID, task).Context(ctx)
	if opts.parent != "" {
		call = call.Parent(opts.parent)
	}
	created, err := call.Do()
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	if out.json {
		return out.writeJSON(taskOutputFromTask(created))
	}

	out.writeMessage(fmt.Sprintf("Created task %q (ID: %s)", created.Title, created.Id))
	return nil
}

// runTasksRead gets details of a single task, including its subtasks.
func runTasksRead(ctx context.Context, conn *gwcli.CmdG, tasklistID, taskID string, out *outputWriter) error {
	tasklistID = resolveTasklistID(tasklistID)
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

	// The API has no "children of" query: subtasks are the tasks in the same
	// list whose parent is this task.
	all, err := fetchTasks(ctx, svc.Tasks.List(tasklistID).ShowCompleted(true).ShowHidden(true), 0)
	if err != nil {
		return fmt.Errorf("failed to list subtasks: %w", err)
	}
	var subtasks []*tasks.Task
	for _, t := range all {
		if t.Parent == task.Id {
			subtasks = append(subtasks, t)
		}
	}
	sort.SliceStable(subtasks, func(i, j int) bool { return subtasks[i].Position < subtasks[j].Position })

	if out.json {
		detail := taskDetailOutput{taskOutput: taskOutputFromTask(task)}
		for _, s := range subtasks {
			detail.Subtasks = append(detail.Subtasks, taskOutputFromTask(s))
		}
		return out.writeJSON(detail)
	}

	status := "Pending"
	if task.Status == taskStatusCompleted {
		status = "Completed"
	}

	out.writeMessage(fmt.Sprintf("Title: %s", task.Title))
	out.writeMessage(fmt.Sprintf("Status: %s", status))
	if task.Due != "" {
		out.writeMessage(fmt.Sprintf("Due: %s", formatTaskDate(task.Due)))
	}
	if task.Completed != nil && *task.Completed != "" {
		out.writeMessage(fmt.Sprintf("Completed: %s", *task.Completed))
	}
	if task.Parent != "" {
		out.writeMessage(fmt.Sprintf("Parent: %s", task.Parent))
	}
	if task.Notes != "" {
		out.writeMessage(fmt.Sprintf("Notes: %s", task.Notes))
	}
	if len(subtasks) > 0 {
		out.writeMessage("Subtasks:")
		for _, s := range subtasks {
			out.writeMessage(fmt.Sprintf("  %s %s (ID: %s)", taskStatusBox(s.Status), s.Title, s.Id))
		}
	}
	if task.WebViewLink != "" {
		out.writeMessage(fmt.Sprintf("Link: %s", task.WebViewLink))
	}
	out.writeMessage(fmt.Sprintf("ID: %s", task.Id))

	return nil
}

// runTasksComplete marks a task as completed.
func runTasksComplete(ctx context.Context, conn *gwcli.CmdG, tasklistID, taskID string, out *outputWriter) error {
	tasklistID = resolveTasklistID(tasklistID)
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
		Status: taskStatusCompleted,
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

// taskUpdateOptions holds the `tasks update` flags. Empty strings mean
// "leave unchanged"; the clear flags remove a field.
type taskUpdateOptions struct {
	title      string
	notes      string
	due        string
	status     string
	clearNotes bool
	clearDue   bool
}

// normalizeTaskStatus maps a user-supplied status to the API value.
func normalizeTaskStatus(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "completed", "complete", "done":
		return taskStatusCompleted, nil
	case "needsaction", "needs-action", "pending", "open":
		return taskStatusNeedsAction, nil
	default:
		return "", fmt.Errorf("invalid status %q: expected completed or needsAction", s)
	}
}

// buildTaskPatch validates update options and builds the PATCH body.
func buildTaskPatch(opts taskUpdateOptions) (*tasks.Task, error) {
	if opts.notes != "" && opts.clearNotes {
		return nil, fmt.Errorf("--notes and --clear-notes are mutually exclusive")
	}
	if opts.due != "" && opts.clearDue {
		return nil, fmt.Errorf("--due and --clear-due are mutually exclusive")
	}

	patch := &tasks.Task{}
	changed := false
	if opts.title != "" {
		patch.Title = opts.title
		changed = true
	}
	if opts.notes != "" {
		patch.Notes = opts.notes
		changed = true
	}
	if opts.clearNotes {
		patch.NullFields = append(patch.NullFields, "Notes")
		changed = true
	}
	if opts.due != "" {
		due, err := normalizeTaskDue(opts.due)
		if err != nil {
			return nil, err
		}
		patch.Due = due
		changed = true
	}
	if opts.clearDue {
		patch.NullFields = append(patch.NullFields, "Due")
		changed = true
	}
	if opts.status != "" {
		status, err := normalizeTaskStatus(opts.status)
		if err != nil {
			return nil, err
		}
		patch.Status = status
		if status == taskStatusNeedsAction {
			// Reopening a task must also drop its completion timestamp.
			patch.NullFields = append(patch.NullFields, "Completed")
		}
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("nothing to update: pass at least one of --title, --notes, --due, --status, --clear-notes, --clear-due")
	}
	return patch, nil
}

// runTasksUpdate patches a task's title, notes, due date, or status.
func runTasksUpdate(ctx context.Context, conn *gwcli.CmdG, tasklistID, taskID string, opts taskUpdateOptions, out *outputWriter) error {
	tasklistID = resolveTasklistID(tasklistID)
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	patch, err := buildTaskPatch(opts)
	if err != nil {
		return err
	}

	out.writeVerbose("Updating task %s in list %s...", taskID, tasklistID)

	svc := conn.TasksService()
	if svc == nil {
		return fmt.Errorf("tasks service not initialized")
	}

	updated, err := svc.Tasks.Patch(tasklistID, taskID, patch).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	if out.json {
		return out.writeJSON(taskOutputFromTask(updated))
	}

	out.writeMessage(fmt.Sprintf("Updated task %q (ID: %s)", updated.Title, updated.Id))
	return nil
}

// runTasksDelete deletes a task.
func runTasksDelete(ctx context.Context, conn *gwcli.CmdG, tasklistID, taskID string, force bool, out *outputWriter) error {
	tasklistID = resolveTasklistID(tasklistID)
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}
	if !force {
		return fmt.Errorf("refusing to delete task %s without --force", taskID)
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
