package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/wesnick/gwcli/pkg/gwcli"
	"google.golang.org/api/tasks/v1"
)

// jsonResponse builds a 200 response with the given JSON body.
func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// withTaskListRoute answers tasks.list requests (GET .../lists/<id>/tasks)
// with listJSON and passes everything else to next. `tasks get` issues such a
// request to find subtasks.
func withTaskListRoute(listJSON string, next func(*http.Request) (*http.Response, error)) roundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/tasks") {
			return jsonResponse(listJSON), nil
		}
		return next(req)
	}
}

// newTasksFake returns a fake connection whose HTTP traffic goes to rt.
func newTasksFake(t *testing.T, rt roundTripFunc) *gwcli.CmdG {
	t.Helper()
	conn, err := gwcli.NewFake(&http.Client{Transport: rt})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	return conn
}

// noHTTP fails the test if any request is made.
func noHTTP(t *testing.T) roundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL)
		return nil, nil
	}
}

func TestResolveTaskRef(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		listFlag string
		wantList string
		wantTask string
		wantErr  string
	}{
		{name: "task only defaults list", args: []string{"T1"}, wantList: "@default", wantTask: "T1"},
		{name: "task with --list-id", args: []string{"T1"}, listFlag: "L1", wantList: "L1", wantTask: "T1"},
		{name: "legacy two-arg form", args: []string{"L1", "T1"}, wantList: "L1", wantTask: "T1"},
		{name: "legacy form agreeing flag", args: []string{"L1", "T1"}, listFlag: "L1", wantList: "L1", wantTask: "T1"},
		{name: "legacy form conflicting flag", args: []string{"L1", "T1"}, listFlag: "L2", wantErr: "given twice"},
		{name: "no args", args: nil, wantErr: "task ID is required"},
		{name: "empty task", args: []string{""}, wantErr: "task ID is required"},
		{name: "too many", args: []string{"a", "b", "c"}, wantErr: "got 3 arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, task, err := resolveTaskRef(tt.args, tt.listFlag)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveTaskRef() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveTaskRef() error = %v", err)
			}
			if list != tt.wantList || task != tt.wantTask {
				t.Errorf("resolveTaskRef() = (%q, %q), want (%q, %q)", list, task, tt.wantList, tt.wantTask)
			}
		})
	}
}

func TestResolveTasklistArg(t *testing.T) {
	tests := []struct {
		positional, flag, want string
		wantErr                bool
	}{
		{"", "", "@default", false},
		{"L1", "", "L1", false},
		{"", "L2", "L2", false},
		{"L1", "L1", "L1", false},
		{"L1", "L2", "", true},
	}
	for _, tt := range tests {
		got, err := resolveTasklistArg(tt.positional, tt.flag)
		if (err != nil) != tt.wantErr {
			t.Errorf("resolveTasklistArg(%q, %q) error = %v, wantErr %v", tt.positional, tt.flag, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("resolveTasklistArg(%q, %q) = %q, want %q", tt.positional, tt.flag, got, tt.want)
		}
	}
}

func TestNormalizeTaskDue(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "2026-10-01", want: "2026-10-01T00:00:00.000Z"},
		{in: " 2026-10-01 ", want: "2026-10-01T00:00:00.000Z"},
		{in: "2026-10-01T00:00:00Z", want: "2026-10-01T00:00:00.000Z"},
		{in: "2026-10-01T00:00:00.000Z", want: "2026-10-01T00:00:00.000Z"},
		{in: "2026-10-01T15:30:00Z", want: "2026-10-01T00:00:00.000Z"},
		// The date is kept as written, not shifted to the UTC date (which
		// would be 2026-10-02 here).
		{in: "2026-10-01T20:00:00-07:00", want: "2026-10-01T00:00:00.000Z"},
		{in: "2026-10-01T01:00:00+09:00", want: "2026-10-01T00:00:00.000Z"},
		{in: "", wantErr: true},
		{in: "tomorrow", wantErr: true},
		{in: "2026-13-01", wantErr: true},
		{in: "10/01/2026", wantErr: true},
	}
	for _, tt := range tests {
		got, err := normalizeTaskDue(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("normalizeTaskDue(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("normalizeTaskDue(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTaskDueRange(t *testing.T) {
	tests := []struct {
		name                string
		due, dueMin, dueMax string
		wantMin, wantMax    string
		wantErr             string
	}{
		{name: "none"},
		{name: "single day", due: "2026-10-01", wantMin: "2026-10-01T00:00:00Z", wantMax: "2026-10-01T23:59:59Z"},
		{name: "min only", dueMin: "2026-10-01", wantMin: "2026-10-01T00:00:00Z"},
		{name: "max only", dueMax: "2026-10-05", wantMax: "2026-10-05T23:59:59Z"},
		{name: "range", dueMin: "2026-10-01", dueMax: "2026-10-05T12:00:00Z", wantMin: "2026-10-01T00:00:00Z", wantMax: "2026-10-05T23:59:59Z"},
		{name: "due with range", due: "2026-10-01", dueMin: "2026-10-01", wantErr: "cannot be combined"},
		{name: "inverted", dueMin: "2026-10-05", dueMax: "2026-10-01", wantErr: "after"},
		{name: "bad date", due: "soon", wantErr: "invalid date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lo, hi, err := taskDueRange(tt.due, tt.dueMin, tt.dueMax)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("taskDueRange() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("taskDueRange() error = %v", err)
			}
			if lo != tt.wantMin || hi != tt.wantMax {
				t.Errorf("taskDueRange() = (%q, %q), want (%q, %q)", lo, hi, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestOrderTasks(t *testing.T) {
	items := []*tasks.Task{
		{Id: "C1", Parent: "B", Position: "2"},
		{Id: "B", Position: "2"},
		{Id: "A", Position: "1"},
		{Id: "C0", Parent: "B", Position: "1"},
		{Id: "ORPHAN", Parent: "GONE", Position: "3"},
		{Id: "A1", Parent: "A", Position: "1"},
	}
	ordered, isChild := orderTasks(items)

	var ids []string
	for _, t := range ordered {
		ids = append(ids, t.Id)
	}
	if got, want := strings.Join(ids, ","), "A,A1,B,C0,C1,ORPHAN"; got != want {
		t.Errorf("order = %s, want %s", got, want)
	}
	wantChild := []bool{false, true, false, true, true, false}
	for i := range wantChild {
		if isChild[i] != wantChild[i] {
			t.Errorf("isChild[%d] (%s) = %v, want %v", i, ids[i], isChild[i], wantChild[i])
		}
	}
}

func TestBuildTaskPatch(t *testing.T) {
	tests := []struct {
		name     string
		opts     taskUpdateOptions
		wantJSON map[string]any
		wantErr  string
	}{
		{name: "nothing", opts: taskUpdateOptions{}, wantErr: "nothing to update"},
		{name: "title", opts: taskUpdateOptions{title: "New"}, wantJSON: map[string]any{"title": "New"}},
		{name: "due date", opts: taskUpdateOptions{due: "2026-10-01"}, wantJSON: map[string]any{"due": "2026-10-01T00:00:00.000Z"}},
		{name: "clear due", opts: taskUpdateOptions{clearDue: true}, wantJSON: map[string]any{"due": nil}},
		{name: "clear notes", opts: taskUpdateOptions{clearNotes: true}, wantJSON: map[string]any{"notes": nil}},
		{name: "complete", opts: taskUpdateOptions{status: "done"}, wantJSON: map[string]any{"status": "completed"}},
		{name: "reopen clears completed", opts: taskUpdateOptions{status: "needsAction"}, wantJSON: map[string]any{"status": "needsAction", "completed": nil}},
		{name: "bad status", opts: taskUpdateOptions{status: "maybe"}, wantErr: "invalid status"},
		{name: "bad due", opts: taskUpdateOptions{due: "someday"}, wantErr: "invalid date"},
		{name: "due and clear", opts: taskUpdateOptions{due: "2026-10-01", clearDue: true}, wantErr: "mutually exclusive"},
		{name: "notes and clear", opts: taskUpdateOptions{notes: "x", clearNotes: true}, wantErr: "mutually exclusive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := buildTaskPatch(tt.opts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("buildTaskPatch() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildTaskPatch() error = %v", err)
			}
			raw, err := json.Marshal(patch)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(got) != len(tt.wantJSON) {
				t.Errorf("patch JSON = %s, want keys %v", raw, tt.wantJSON)
			}
			for k, v := range tt.wantJSON {
				gv, ok := got[k]
				if !ok || gv != v {
					t.Errorf("patch JSON = %s, want %s=%v", raw, k, v)
				}
			}
		})
	}
}

func TestTasksDefaultList(t *testing.T) {
	const taskJSON = `{"id": "T1", "title": "Task", "status": "needsAction"}`
	tests := []struct {
		name     string
		wantPath string
		run      func(*gwcli.CmdG, *outputWriter) error
	}{
		{"list", "/tasks/v1/lists/@default/tasks", func(c *gwcli.CmdG, o *outputWriter) error {
			return runTasksList(context.Background(), c, tasksListOptions{}, o)
		}},
		{"create", "/tasks/v1/lists/@default/tasks", func(c *gwcli.CmdG, o *outputWriter) error {
			return runTasksCreate(context.Background(), c, "", taskCreateOptions{title: "Task"}, o)
		}},
		{"get", "/tasks/v1/lists/@default/tasks/T1", func(c *gwcli.CmdG, o *outputWriter) error {
			return runTasksRead(context.Background(), c, "", "T1", o)
		}},
		{"update", "/tasks/v1/lists/@default/tasks/T1", func(c *gwcli.CmdG, o *outputWriter) error {
			return runTasksUpdate(context.Background(), c, "", "T1", taskUpdateOptions{title: "x"}, o)
		}},
		{"complete", "/tasks/v1/lists/@default/tasks/T1", func(c *gwcli.CmdG, o *outputWriter) error {
			return runTasksComplete(context.Background(), c, "", "T1", o)
		}},
		{"delete", "/tasks/v1/lists/@default/tasks/T1", func(c *gwcli.CmdG, o *outputWriter) error {
			return runTasksDelete(context.Background(), c, "", "T1", true, o)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paths []string
			conn := newTasksFake(t, func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/tasks") {
					return jsonResponse(`{"items": []}`), nil
				}
				if req.Method == http.MethodDelete {
					return &http.Response{StatusCode: http.StatusNoContent, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}, nil
				}
				return jsonResponse(taskJSON), nil
			})
			out := &outputWriter{json: true, writer: &bytes.Buffer{}}
			if err := tt.run(conn, out); err != nil {
				t.Fatalf("error = %v", err)
			}
			if len(paths) == 0 || paths[0] != tt.wantPath {
				t.Errorf("first request path = %v, want %s", paths, tt.wantPath)
			}
		})
	}
}

func TestRunTasksListQueryParams(t *testing.T) {
	tests := []struct {
		name   string
		opts   tasksListOptions
		want   map[string]string
		absent []string
	}{
		{
			name:   "default hides completed",
			opts:   tasksListOptions{tasklistID: "L1"},
			want:   map[string]string{"showCompleted": "false", "maxResults": "100"},
			absent: []string{"showHidden", "dueMin", "dueMax"},
		},
		{
			name: "show completed includes hidden",
			opts: tasksListOptions{tasklistID: "L1", showCompleted: true},
			want: map[string]string{"showCompleted": "true", "showHidden": "true"},
		},
		{
			name: "due day",
			opts: tasksListOptions{tasklistID: "L1", due: "2026-10-01"},
			want: map[string]string{"dueMin": "2026-10-01T00:00:00Z", "dueMax": "2026-10-01T23:59:59Z"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := newTasksFake(t, func(req *http.Request) (*http.Response, error) {
				q := req.URL.Query()
				for k, v := range tt.want {
					if got := q.Get(k); got != v {
						t.Errorf("query %s = %q, want %q (url %s)", k, got, v, req.URL)
					}
				}
				for _, k := range tt.absent {
					if q.Has(k) {
						t.Errorf("query unexpectedly has %s (url %s)", k, req.URL)
					}
				}
				return jsonResponse(`{"items": []}`), nil
			})
			out := &outputWriter{json: true, writer: &bytes.Buffer{}}
			if err := runTasksList(context.Background(), conn, tt.opts, out); err != nil {
				t.Fatalf("runTasksList() error = %v", err)
			}
		})
	}
}

func TestRunTasksListInvalidDueMakesNoRequest(t *testing.T) {
	conn := newTasksFake(t, noHTTP(t))
	out := &outputWriter{json: true, writer: &bytes.Buffer{}}
	err := runTasksList(context.Background(), conn, tasksListOptions{due: "next week"}, out)
	if err == nil || !strings.Contains(err.Error(), "invalid date") {
		t.Fatalf("runTasksList() error = %v, want invalid date", err)
	}
}

func TestRunTasksListPaginationAndLimit(t *testing.T) {
	pages := map[string]string{
		"":   `{"items": [{"id": "A", "position": "1"}, {"id": "B", "position": "2"}], "nextPageToken": "p2"}`,
		"p2": `{"items": [{"id": "C", "position": "3"}]}`,
	}
	rt := func(req *http.Request) (*http.Response, error) {
		return jsonResponse(pages[req.URL.Query().Get("pageToken")]), nil
	}

	for _, tc := range []struct {
		limit int
		want  string
	}{{0, "A,B,C"}, {2, "A,B"}, {3, "A,B,C"}} {
		var buf bytes.Buffer
		out := &outputWriter{json: true, writer: &buf}
		if err := runTasksList(context.Background(), newTasksFake(t, rt), tasksListOptions{limit: tc.limit}, out); err != nil {
			t.Fatalf("limit %d: error = %v", tc.limit, err)
		}
		var got []taskOutput
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var ids []string
		for _, g := range got {
			ids = append(ids, g.ID)
		}
		if strings.Join(ids, ",") != tc.want {
			t.Errorf("limit %d: ids = %v, want %s", tc.limit, ids, tc.want)
		}
	}
}

func TestRunTasksListSubtaskTable(t *testing.T) {
	const listJSON = `{"items": [
		{"id": "CHILD", "title": "Child", "parent": "PARENT", "status": "completed", "position": "1"},
		{"id": "PARENT", "title": "Parent", "status": "needsAction", "due": "2026-10-01T00:00:00.000Z", "position": "1"}
	]}`
	conn := newTasksFake(t, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(listJSON), nil
	})
	var buf bytes.Buffer
	out := &outputWriter{writer: &buf}
	if err := runTasksList(context.Background(), conn, tasksListOptions{showCompleted: true}, out); err != nil {
		t.Fatalf("runTasksList() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got:\n%s", buf.String())
	}
	if !strings.HasPrefix(lines[1], "[ ]") || !strings.Contains(lines[1], "Parent") || !strings.Contains(lines[1], "2026-10-01") {
		t.Errorf("parent row = %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "[x]") || !strings.Contains(lines[2], "↳ Child") {
		t.Errorf("child row = %q", lines[2])
	}
}

func TestRunTasksCreateParentAndDue(t *testing.T) {
	conn := newTasksFake(t, func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("parent"); got != "P1" {
			t.Errorf("parent query = %q, want P1", got)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["due"] != "2026-10-01T00:00:00.000Z" {
			t.Errorf("due = %v, want 2026-10-01T00:00:00.000Z", body["due"])
		}
		return jsonResponse(`{"id": "NEW", "title": "Sub", "parent": "P1"}`), nil
	})
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	err := runTasksCreate(context.Background(), conn, "L1", taskCreateOptions{title: "Sub", due: "2026-10-01", parent: "P1"}, out)
	if err != nil {
		t.Fatalf("runTasksCreate() error = %v", err)
	}
	if !strings.Contains(buf.String(), `"parent": "P1"`) {
		t.Errorf("output missing parent: %s", buf.String())
	}
}

func TestRunTasksCreateInvalidDueMakesNoRequest(t *testing.T) {
	conn := newTasksFake(t, noHTTP(t))
	out := &outputWriter{json: true, writer: &bytes.Buffer{}}
	err := runTasksCreate(context.Background(), conn, "L1", taskCreateOptions{title: "x", due: "01/10/2026"}, out)
	if err == nil || !strings.Contains(err.Error(), "invalid date") {
		t.Fatalf("runTasksCreate() error = %v, want invalid date", err)
	}
}

func TestRunTasksReadSubtasks(t *testing.T) {
	const listJSON = `{"items": [
		{"id": "S2", "title": "Second", "parent": "T1", "status": "needsAction", "position": "2"},
		{"id": "OTHER", "title": "Unrelated", "status": "needsAction", "position": "1"},
		{"id": "S1", "title": "First", "parent": "T1", "status": "completed", "position": "1"}
	]}`
	rt := withTaskListRoute(listJSON, func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"id": "T1", "title": "Parent task", "status": "needsAction", "webViewLink": "https://tasks.google.com/task/T1"}`), nil
	})

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runTasksRead(context.Background(), newTasksFake(t, rt), "L1", "T1", out); err != nil {
		t.Fatalf("runTasksRead() error = %v", err)
	}
	var got taskDetailOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != "T1" || got.WebViewLink == "" {
		t.Errorf("task = %+v", got.taskOutput)
	}
	if len(got.Subtasks) != 2 || got.Subtasks[0].ID != "S1" || got.Subtasks[1].ID != "S2" {
		t.Errorf("subtasks = %+v, want S1, S2", got.Subtasks)
	}

	buf.Reset()
	out.json = false
	if err := runTasksRead(context.Background(), newTasksFake(t, rt), "L1", "T1", out); err != nil {
		t.Fatalf("runTasksRead() text error = %v", err)
	}
	text := buf.String()
	for _, want := range []string{"Subtasks:", "[x] First (ID: S1)", "[ ] Second (ID: S2)", "Link: https://tasks.google.com/task/T1"} {
		if !strings.Contains(text, want) {
			t.Errorf("text output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Unrelated") {
		t.Errorf("text output lists an unrelated task:\n%s", text)
	}
}

func TestRunTasksUpdate(t *testing.T) {
	conn := newTasksFake(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", req.Method)
		}
		if !strings.HasSuffix(req.URL.Path, "/lists/L1/tasks/T1") {
			t.Errorf("path = %s", req.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["title"] != "Renamed" || body["status"] != "needsAction" {
			t.Errorf("body = %v", body)
		}
		if v, ok := body["completed"]; !ok || v != nil {
			t.Errorf("body should null out completed, got %v", body)
		}
		return jsonResponse(`{"id": "T1", "title": "Renamed", "status": "needsAction"}`), nil
	})
	var buf bytes.Buffer
	out := &outputWriter{writer: &buf}
	err := runTasksUpdate(context.Background(), conn, "L1", "T1", taskUpdateOptions{title: "Renamed", status: "open"}, out)
	if err != nil {
		t.Fatalf("runTasksUpdate() error = %v", err)
	}
	if !strings.Contains(buf.String(), `Updated task "Renamed"`) {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunTasksUpdateNothingMakesNoRequest(t *testing.T) {
	conn := newTasksFake(t, noHTTP(t))
	out := &outputWriter{writer: &bytes.Buffer{}}
	if err := runTasksUpdate(context.Background(), conn, "L1", "T1", taskUpdateOptions{}, out); err == nil {
		t.Fatal("expected error for empty update")
	}
}

func TestDeleteRequiresForce(t *testing.T) {
	conn := newTasksFake(t, noHTTP(t))
	out := &outputWriter{writer: &bytes.Buffer{}}
	if err := runTasksDelete(context.Background(), conn, "L1", "T1", false, out); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("runTasksDelete() without force error = %v, want --force error", err)
	}
	if err := runTasklistsDelete(context.Background(), conn, "L1", false, out); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("runTasklistsDelete() without force error = %v, want --force error", err)
	}
}

func TestRunTasklistsListPagination(t *testing.T) {
	pages := map[string]string{
		"":   `{"items": [{"id": "L1", "title": "One"}], "nextPageToken": "p2"}`,
		"p2": `{"items": [{"id": "L2", "title": "Two"}]}`,
	}
	conn := newTasksFake(t, func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("maxResults"); got != "100" {
			t.Errorf("maxResults = %q, want 100", got)
		}
		return jsonResponse(pages[req.URL.Query().Get("pageToken")]), nil
	})
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runTasklistsList(context.Background(), conn, out); err != nil {
		t.Fatalf("runTasklistsList() error = %v", err)
	}
	var got []tasklistOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 || got[0].ID != "L1" || got[1].ID != "L2" {
		t.Errorf("task lists = %+v, want L1, L2", got)
	}
}
