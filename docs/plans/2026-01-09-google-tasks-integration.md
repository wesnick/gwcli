# Google Tasks Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Google Tasks API support to gwcli with full command coverage for task lists and tasks, supporting both OAuth2 desktop flow and service accounts.

**Architecture:** Extend the existing authentication infrastructure in `pkg/gwcli/gmailctl/` to include Tasks API scope. Add a `*tasks.Service` field to the `CmdG` struct, initialized alongside Gmail service. Create `tasks.go` and `tasklists.go` command handlers following the existing patterns in `messages.go` and `labels.go`.

**Tech Stack:** Kong CLI, `google.golang.org/api/tasks/v1`, existing gwcli authentication infrastructure, outputWriter pattern for JSON/text output.

---

## Prerequisites

- gwcli codebase at `/home/wes/Devel/gwcli`
- Reference implementation at `/home/wes/Devel/gwcli/to_import/gtasks`
- Go 1.24+

---

## Task 1: Add Tasks API Scope to Authentication

**Files:**
- Modify: `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth.go:72-82` (OAuth2 scopes)
- Modify: `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth.go:127-133` (Service account scopes)
- Test: `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth_test.go` (new file)

**Step 1: Write the failing test**

Create `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth_test.go`:

```go
package gmailctl

import (
	"strings"
	"testing"
)

func TestClientFromCredentialsIncludesTasksScope(t *testing.T) {
	// Sample OAuth2 credentials JSON (fake client ID/secret)
	credJSON := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token"
		}
	}`

	cfg, err := clientFromCredentials(strings.NewReader(credJSON))
	if err != nil {
		t.Fatalf("clientFromCredentials() error = %v", err)
	}

	// Check that Tasks scope is included
	hasTasksScope := false
	for _, scope := range cfg.Scopes {
		if scope == "https://www.googleapis.com/auth/tasks" {
			hasTasksScope = true
			break
		}
	}

	if !hasTasksScope {
		t.Errorf("clientFromCredentials() missing tasks scope, got scopes: %v", cfg.Scopes)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/gwcli/gmailctl/ -run TestClientFromCredentialsIncludesTasksScope -v`
Expected: FAIL with "missing tasks scope"

**Step 3: Add Tasks import and scope to OAuth2 config**

In `/home/wes/Devel/gwcli/pkg/gwcli/gmailctl/auth.go`, add import:

```go
import (
	// ... existing imports ...
	"google.golang.org/api/tasks/v1"
)
```

Update `clientFromCredentials` function (lines 72-82):

```go
func clientFromCredentials(credentials io.Reader) (*oauth2.Config, error) {
	credBytes, err := io.ReadAll(credentials)
	if err != nil {
		return nil, fmt.Errorf("reading credentials: %w", err)
	}
	return google.ConfigFromJSON(credBytes,
		gmail.GmailModifyScope,
		gmail.GmailSettingsBasicScope,
		gmail.GmailLabelsScope,
		tasks.TasksScope,
	)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/gwcli/gmailctl/ -run TestClientFromCredentialsIncludesTasksScope -v`
Expected: PASS

**Step 5: Update service account scopes**

Update `ServiceAccountAuthenticator.Service()` method (lines 127-143):

```go
func (a *ServiceAccountAuthenticator) Service(ctx context.Context) (*gmail.Service, error) {
	config, err := google.JWTConfigFromJSON(
		a.credBytes,
		gmail.GmailModifyScope,
		gmail.GmailSettingsBasicScope,
		gmail.GmailLabelsScope,
		tasks.TasksScope,
	)
	if err != nil {
		return nil, fmt.Errorf("parsing service account credentials: %w", err)
	}

	config.Subject = a.userEmail
	return gmail.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx)))
}
```

**Step 6: Run all auth tests**

Run: `go test ./pkg/gwcli/gmailctl/ -v`
Expected: All PASS

**Step 7: Commit**

```bash
git add pkg/gwcli/gmailctl/auth.go pkg/gwcli/gmailctl/auth_test.go
git commit -m "$(cat <<'EOF'
feat: add Tasks API scope to authentication

Add tasks.TasksScope to OAuth2 and service account configurations
to enable Google Tasks API access alongside Gmail.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add Tasks Service to CmdG Struct

**Files:**
- Modify: `/home/wes/Devel/gwcli/pkg/gwcli/connection.go`
- Modify: `/home/wes/Devel/gwcli/pkg/gwcli/auth.go`
- Test: `/home/wes/Devel/gwcli/pkg/gwcli/connection_test.go` (add test)

**Step 1: Write the failing test**

Add to `/home/wes/Devel/gwcli/pkg/gwcli/connection_test.go`:

```go
func TestCmdGHasTasksService(t *testing.T) {
	// Create a mock HTTP client
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	if conn.TasksService() == nil {
		t.Error("TasksService() returned nil, expected *tasks.Service")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/gwcli/ -run TestCmdGHasTasksService -v`
Expected: FAIL (TasksService method doesn't exist)

**Step 3: Add tasks import and field to CmdG struct**

In `/home/wes/Devel/gwcli/pkg/gwcli/connection.go`, add import:

```go
import (
	// ... existing imports ...
	"google.golang.org/api/tasks/v1"
)
```

Add field to CmdG struct (find the struct definition):

```go
type CmdG struct {
	// ... existing fields ...
	tasksService *tasks.Service
}
```

Add getter method:

```go
// TasksService returns the Google Tasks API service client.
func (c *CmdG) TasksService() *tasks.Service {
	return c.tasksService
}
```

**Step 4: Update setupClients to initialize Tasks service**

Find `setupClients()` method and add Tasks service initialization:

```go
func (c *CmdG) setupClients() error {
	// ... existing Gmail service setup ...

	// Initialize Tasks service
	tasksSvc, err := tasks.NewService(context.Background(), option.WithHTTPClient(c.authedClient))
	if err != nil {
		return fmt.Errorf("creating tasks service: %w", err)
	}
	c.tasksService = tasksSvc

	return nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./pkg/gwcli/ -run TestCmdGHasTasksService -v`
Expected: PASS

**Step 6: Run all connection tests**

Run: `go test ./pkg/gwcli/ -v`
Expected: All PASS

**Step 7: Commit**

```bash
git add pkg/gwcli/connection.go pkg/gwcli/connection_test.go
git commit -m "$(cat <<'EOF'
feat: add Tasks service to CmdG struct

Initialize Google Tasks API service alongside Gmail service
in the connection setup, enabling task operations.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Create Task List Commands

**Files:**
- Create: `/home/wes/Devel/gwcli/tasklists.go`
- Test: `/home/wes/Devel/gwcli/tasklists_test.go`

**Step 1: Write the failing test for list task lists**

Create `/home/wes/Devel/gwcli/tasklists_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
)

func TestRunTasklistsList(t *testing.T) {
	// Mock response for tasklists.list
	tasklistsJSON := `{
		"kind": "tasks#taskLists",
		"items": [
			{"id": "tl1", "title": "My Tasks", "updated": "2024-01-01T00:00:00.000Z"},
			{"id": "tl2", "title": "Work", "updated": "2024-01-02T00:00:00.000Z"}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/tasks/v1/users/@me/lists") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tasklistsJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runTasklistsList(context.Background(), conn, out)
	if err != nil {
		t.Fatalf("runTasklistsList() error = %v", err)
	}

	var result []tasklistOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 tasklists, got %d", len(result))
	}

	if result[0].Title != "My Tasks" {
		t.Errorf("expected first tasklist title 'My Tasks', got %q", result[0].Title)
	}
}

// roundTripFunc helper for mocking HTTP
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunTasklistsList -v`
Expected: FAIL (runTasklistsList doesn't exist)

**Step 3: Create tasklists.go with list command**

Create `/home/wes/Devel/gwcli/tasklists.go`:

```go
package main

import (
	"context"
	"fmt"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
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
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunTasklistsList -v`
Expected: PASS

**Step 5: Add create task list command**

Add test to `tasklists_test.go`:

```go
func TestRunTasklistsCreate(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/tasks/v1/users/@me/lists") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id": "new-tl", "title": "New List"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runTasklistsCreate(context.Background(), conn, "New List", out)
	if err != nil {
		t.Fatalf("runTasklistsCreate() error = %v", err)
	}

	var result tasklistOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Title != "New List" {
		t.Errorf("expected title 'New List', got %q", result.Title)
	}
}
```

Add to `tasklists.go`:

```go
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
```

Add import for tasks:

```go
import (
	"context"
	"fmt"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/tasks/v1"
)
```

**Step 6: Add delete task list command**

Add test to `tasklists_test.go`:

```go
func TestRunTasklistsDelete(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "DELETE" && strings.Contains(req.URL.Path, "/tasks/v1/users/@me/lists/tl-to-delete") {
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(``)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runTasklistsDelete(ctx context.Background(), conn, "tl-to-delete", false, out)
	if err != nil {
		t.Fatalf("runTasklistsDelete() error = %v", err)
	}
}
```

Add to `tasklists.go`:

```go
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
```

**Step 7: Run all tasklist tests**

Run: `go test . -run TestRunTasklists -v`
Expected: All PASS

**Step 8: Commit**

```bash
git add tasklists.go tasklists_test.go
git commit -m "$(cat <<'EOF'
feat: add task list commands (list, create, delete)

Implement tasklists subcommands for managing Google Tasks lists:
- tasklists list: List all task lists
- tasklists create: Create a new task list
- tasklists delete: Delete a task list

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Create Task Commands

**Files:**
- Create: `/home/wes/Devel/gwcli/tasks.go`
- Test: `/home/wes/Devel/gwcli/tasks_test.go`

**Step 1: Write failing test for list tasks**

Create `/home/wes/Devel/gwcli/tasks_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
)

func TestRunTasksList(t *testing.T) {
	tasksJSON := `{
		"kind": "tasks#tasks",
		"items": [
			{"id": "t1", "title": "Task 1", "status": "needsAction"},
			{"id": "t2", "title": "Task 2", "status": "completed", "completed": "2024-01-01T00:00:00.000Z"}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/tasks/v1/lists/tl1/tasks") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tasksJSON)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runTasksList(context.Background(), conn, "tl1", true, out)
	if err != nil {
		t.Fatalf("runTasksList() error = %v", err)
	}

	var result []taskOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(result))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test . -run TestRunTasksList -v`
Expected: FAIL (runTasksList doesn't exist)

**Step 3: Create tasks.go with list command**

Create `/home/wes/Devel/gwcli/tasks.go`:

```go
package main

import (
	"context"
	"fmt"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
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

// taskOutputFromTask converts a tasks.Task to taskOutput.
func taskOutputFromTask(t *tasks.Task) taskOutput {
	return taskOutput{
		ID:        t.Id,
		Title:     t.Title,
		Notes:     t.Notes,
		Status:    t.Status,
		Due:       t.Due,
		Completed: t.Completed,
		Parent:    t.Parent,
		Position:  t.Position,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test . -run TestRunTasksList -v`
Expected: PASS

**Step 5: Add create task command**

Add test to `tasks_test.go`:

```go
func TestRunTasksCreate(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/tasks/v1/lists/tl1/tasks") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id": "new-task", "title": "New Task", "status": "needsAction"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runTasksCreate(context.Background(), conn, "tl1", "New Task", "", "", out)
	if err != nil {
		t.Fatalf("runTasksCreate() error = %v", err)
	}

	var result taskOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Title != "New Task" {
		t.Errorf("expected title 'New Task', got %q", result.Title)
	}
}
```

Add to `tasks.go`:

```go
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
```

**Step 6: Add complete task command**

Add test to `tasks_test.go`:

```go
func TestRunTasksComplete(t *testing.T) {
	// First GET to fetch the task, then PATCH to update it
	callCount := 0
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			callCount++
			if req.Method == "GET" && strings.Contains(req.URL.Path, "/tasks/v1/lists/tl1/tasks/t1") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id": "t1", "title": "Task 1", "status": "needsAction"}`)),
				}, nil
			}
			if req.Method == "PATCH" && strings.Contains(req.URL.Path, "/tasks/v1/lists/tl1/tasks/t1") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id": "t1", "title": "Task 1", "status": "completed"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runTasksComplete(context.Background(), conn, "tl1", "t1", out)
	if err != nil {
		t.Fatalf("runTasksComplete() error = %v", err)
	}

	var result taskOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", result.Status)
	}
}
```

Add to `tasks.go`:

```go
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
```

**Step 7: Add read task command**

Add test to `tasks_test.go`:

```go
func TestRunTasksRead(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/tasks/v1/lists/tl1/tasks/t1") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"id": "t1", "title": "Task 1", "notes": "Some notes", "status": "needsAction", "due": "2024-12-31T00:00:00.000Z"}`)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runTasksRead(context.Background(), conn, "tl1", "t1", out)
	if err != nil {
		t.Fatalf("runTasksRead() error = %v", err)
	}

	var result taskOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.Title != "Task 1" {
		t.Errorf("expected title 'Task 1', got %q", result.Title)
	}

	if result.Notes != "Some notes" {
		t.Errorf("expected notes 'Some notes', got %q", result.Notes)
	}
}
```

Add to `tasks.go`:

```go
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
```

**Step 8: Add delete task command**

Add test to `tasks_test.go`:

```go
func TestRunTasksDelete(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "DELETE" && strings.Contains(req.URL.Path, "/tasks/v1/lists/tl1/tasks/t1") {
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(``)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runTasksDelete(context.Background(), conn, "tl1", "t1", false, out)
	if err != nil {
		t.Fatalf("runTasksDelete() error = %v", err)
	}
}
```

Add to `tasks.go`:

```go
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
```

**Step 9: Run all task tests**

Run: `go test . -run TestRunTasks -v`
Expected: All PASS

**Step 10: Commit**

```bash
git add tasks.go tasks_test.go
git commit -m "$(cat <<'EOF'
feat: add task commands (list, create, read, complete, delete)

Implement tasks subcommands for managing Google Tasks:
- tasks list: List tasks in a task list
- tasks create: Create a new task
- tasks read: Get task details
- tasks complete: Mark a task as completed
- tasks delete: Delete a task

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Wire Commands into Kong CLI

**Files:**
- Modify: `/home/wes/Devel/gwcli/main.go`

**Step 1: Add Tasklists and Tasks command structs to CLI**

Find the CLI struct in `main.go` and add:

```go
type CLI struct {
	// ... existing fields (Config, User, JSON, Verbose, NoColor) ...

	// ... existing commands ...

	Tasklists struct {
		List struct{} `cmd:"" help:"List all task lists"`

		Create struct {
			Title string `arg:"" help:"Title for the new task list"`
		} `cmd:"" help:"Create a new task list"`

		Delete struct {
			TasklistID string `arg:"" help:"ID of the task list to delete"`
			Force      bool   `name:"force" short:"f" help:"Skip confirmation"`
		} `cmd:"" help:"Delete a task list"`
	} `cmd:"" help:"Manage Google Tasks lists"`

	Tasks struct {
		List struct {
			TasklistID       string `arg:"" help:"Task list ID"`
			IncludeCompleted bool   `name:"include-completed" short:"a" help:"Include completed tasks"`
		} `cmd:"" help:"List tasks in a task list"`

		Create struct {
			TasklistID string `arg:"" help:"Task list ID"`
			Title      string `name:"title" short:"t" required:"" help:"Task title"`
			Notes      string `name:"notes" short:"n" help:"Task notes"`
			Due        string `name:"due" short:"d" help:"Due date (RFC3339 format)"`
		} `cmd:"" help:"Create a new task"`

		Read struct {
			TasklistID string `arg:"" help:"Task list ID"`
			TaskID     string `arg:"" help:"Task ID"`
		} `cmd:"" help:"Get task details"`

		Complete struct {
			TasklistID string `arg:"" help:"Task list ID"`
			TaskID     string `arg:"" help:"Task ID"`
		} `cmd:"" help:"Mark a task as completed"`

		Delete struct {
			TasklistID string `arg:"" help:"Task list ID"`
			TaskID     string `arg:"" help:"Task ID"`
			Force      bool   `name:"force" short:"f" help:"Skip confirmation"`
		} `cmd:"" help:"Delete a task"`
	} `cmd:"" help:"Manage Google Tasks"`
}
```

**Step 2: Add case handlers in the switch statement**

Find the `switch ctx.Command()` block in `main()` and add cases:

```go
	case "tasklists list":
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasklistsList(context.Background(), conn, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasklists create <title>":
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasklistsCreate(context.Background(), conn, cli.Tasklists.Create.Title, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasklists delete <tasklist-id>":
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasklistsDelete(context.Background(), conn, cli.Tasklists.Delete.TasklistID, cli.Tasklists.Delete.Force, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks list <tasklist-id>":
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksList(context.Background(), conn, cli.Tasks.List.TasklistID, cli.Tasks.List.IncludeCompleted, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks create <tasklist-id>":
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksCreate(context.Background(), conn, cli.Tasks.Create.TasklistID, cli.Tasks.Create.Title, cli.Tasks.Create.Notes, cli.Tasks.Create.Due, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks read <tasklist-id> <task-id>":
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksRead(context.Background(), conn, cli.Tasks.Read.TasklistID, cli.Tasks.Read.TaskID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks complete <tasklist-id> <task-id>":
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksComplete(context.Background(), conn, cli.Tasks.Complete.TasklistID, cli.Tasks.Complete.TaskID, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}

	case "tasks delete <tasklist-id> <task-id>":
		conn, err := getConnection(cli.Config, cli.User, cli.Verbose)
		if err != nil {
			out.writeError(err)
			os.Exit(3)
		}
		if err := runTasksDelete(context.Background(), conn, cli.Tasks.Delete.TasklistID, cli.Tasks.Delete.TaskID, cli.Tasks.Delete.Force, out); err != nil {
			out.writeError(err)
			os.Exit(2)
		}
```

**Step 3: Build and verify help output**

Run: `go build -o gwcli . && ./gwcli --help`
Expected: Shows `tasklists` and `tasks` commands in help

Run: `./gwcli tasklists --help`
Expected: Shows tasklists subcommands

Run: `./gwcli tasks --help`
Expected: Shows tasks subcommands

**Step 4: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add main.go
git commit -m "$(cat <<'EOF'
feat: wire tasks and tasklists commands into Kong CLI

Add command routing for all tasklists and tasks subcommands:
- tasklists: list, create, delete
- tasks: list, create, read, complete, delete

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Update Documentation

**Files:**
- Modify: `/home/wes/Devel/gwcli/CLAUDE.md`

**Step 1: Add Tasks section to CLAUDE.md**

Add to the documentation after the Gmail sections:

```markdown
## Google Tasks Commands

gwcli supports Google Tasks API for managing task lists and tasks.

### Task Lists

```bash
# List all task lists
gwcli tasklists list
gwcli tasklists list --json

# Create a new task list
gwcli tasklists create "Work Projects"

# Delete a task list
gwcli tasklists delete <tasklist-id>
gwcli tasklists delete <tasklist-id> --force
```

### Tasks

```bash
# List tasks in a task list
gwcli tasks list <tasklist-id>
gwcli tasks list <tasklist-id> --include-completed
gwcli tasks list <tasklist-id> --json

# Create a new task
gwcli tasks create <tasklist-id> --title "Review PR"
gwcli tasks create <tasklist-id> --title "Review PR" --notes "Check tests" --due "2024-12-31T00:00:00Z"

# Read task details
gwcli tasks read <tasklist-id> <task-id>
gwcli tasks read <tasklist-id> <task-id> --json

# Mark task as completed
gwcli tasks complete <tasklist-id> <task-id>

# Delete a task
gwcli tasks delete <tasklist-id> <task-id>
gwcli tasks delete <tasklist-id> <task-id> --force
```

### Service Account Usage

For Google Workspace accounts using service accounts:

```bash
# List task lists for a specific user
gwcli --user user@example.com tasklists list

# Create task for a user
gwcli --user user@example.com tasks create <tasklist-id> --title "New Task"
```

Note: Service accounts require domain-wide delegation with the `https://www.googleapis.com/auth/tasks` scope authorized in Google Workspace Admin Console.
```

**Step 2: Update OAuth scopes documentation**

Find the "Required OAuth scopes" section and add:

```markdown
Required OAuth scopes:
- `https://www.googleapis.com/auth/gmail.modify`
- `https://www.googleapis.com/auth/gmail.settings.basic`
- `https://www.googleapis.com/auth/gmail.labels`
- `https://www.googleapis.com/auth/tasks`
```

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: add Google Tasks documentation

Document tasklists and tasks commands including:
- Command usage examples
- JSON output examples
- Service account usage
- OAuth scope requirements

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Run Full Test Suite and Final Verification

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All PASS

**Step 2: Build the binary**

Run: `go build -o gwcli .`
Expected: Builds successfully with no errors

**Step 3: Verify help output**

Run: `./gwcli --help`
Expected: Shows all commands including tasklists and tasks

Run: `./gwcli tasklists --help`
Run: `./gwcli tasks --help`
Expected: Shows all subcommands with proper help text

**Step 4: Run linter**

Run: `go vet ./...`
Expected: No warnings

**Step 5: Test with real credentials (manual)**

If you have OAuth credentials configured:
```bash
./gwcli tasklists list
./gwcli tasklists create "Test List"
./gwcli tasks create <new-list-id> --title "Test Task"
./gwcli tasks list <list-id>
./gwcli tasks complete <list-id> <task-id>
./gwcli tasklists delete <list-id> --force
```

**Step 6: Final commit (if any fixes needed)**

```bash
git status
# If changes needed, commit them
```

---

## Summary

This plan implements Google Tasks integration with:

1. **Authentication**: Adds `tasks.TasksScope` to both OAuth2 and service account configurations
2. **Service Layer**: Adds `*tasks.Service` to CmdG struct with proper initialization
3. **Task Lists Commands**: list, create, delete
4. **Tasks Commands**: list, create, read, complete, delete
5. **Testing**: Mock HTTP transport tests for all commands
6. **Documentation**: Updated CLAUDE.md with usage examples

**Total Files:**
- Modified: 4 (auth.go, connection.go, main.go, CLAUDE.md)
- Created: 4 (tasklists.go, tasklists_test.go, tasks.go, tasks_test.go)
- Tests: 1 existing modified (connection_test.go), 1 new (auth_test.go)

**Estimated Commits:** 7
