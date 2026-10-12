package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/alecthomas/kong"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
)

func notFoundResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}
}

// commentsPage1/2 form a two-page comments.list response: one open comment
// with a reply and a deleted reply, one resolved comment, one deleted comment.
const commentsPage1 = `{"nextPageToken":"PAGE2","comments":[
  {"id":"C1","author":{"displayName":"Alice","emailAddress":"alice@example.com"},
   "content":"Should this be\nplural?","quotedFileContent":{"mimeType":"text/plain","value":"the widget"},
   "anchor":"{\"r\":\"head\"}","resolved":false,"createdTime":"2026-01-02T03:04:05Z",
   "modifiedTime":"2026-01-02T04:00:00Z",
   "replies":[
     {"id":"R1","author":{"displayName":"Bob"},"content":"Yes","createdTime":"2026-01-02T03:30:00Z"},
     {"id":"R2","author":{"displayName":"Bob"},"deleted":true,"createdTime":"2026-01-02T03:31:00Z"}
   ]},
  {"id":"C2","author":{"displayName":"Bob"},"content":"Done","resolved":true,
   "replies":[{"id":"R3","content":"","action":"resolve"}]}
]}`

const commentsPage2 = `{"comments":[
  {"id":"C3","author":{"displayName":"Carol"},"deleted":true,"content":""}
]}`

// fakeCommentsList serves the two pages above and records every request's
// query string.
func fakeCommentsList(t *testing.T, queries *[]map[string][]string) *gwcli.CmdG {
	t.Helper()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/drive/v3/files/F1/comments") {
				q := req.URL.Query()
				*queries = append(*queries, q)
				if q.Get("pageToken") == "PAGE2" {
					return jsonResponse(commentsPage2), nil
				}
				return jsonResponse(commentsPage1), nil
			}
			return notFoundResponse(), nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	return conn
}

func TestRunDriveCommentsList_DefaultsToOpenComments(t *testing.T) {
	var queries []map[string][]string
	conn := fakeCommentsList(t, &queries)

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveCommentsList(context.Background(), conn, "https://drive.google.com/file/d/F1/view", false, false, out); err != nil {
		t.Fatalf("runDriveCommentsList() error = %v", err)
	}

	if len(queries) != 2 {
		t.Fatalf("expected 2 paginated requests, got %d", len(queries))
	}
	for i, q := range queries {
		if f := strings.Join(q["fields"], ","); !strings.Contains(f, "comments(") || !strings.Contains(f, "nextPageToken") {
			t.Errorf("request %d fields = %q, want a comments(...) mask with nextPageToken", i, f)
		}
		if got := strings.Join(q["includeDeleted"], ","); got != "false" {
			t.Errorf("request %d includeDeleted = %q, want false", i, got)
		}
	}

	var got []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if len(got) != 1 || got[0]["id"] != "C1" {
		t.Fatalf("expected only open comment C1, got %s", buf.String())
	}
	c := got[0]
	author := c["author"].(map[string]interface{})
	if author["displayName"] != "Alice" || author["emailAddress"] != "alice@example.com" {
		t.Errorf("author = %+v", author)
	}
	q, _ := c["quotedFileContent"].(map[string]interface{})
	if q["value"] != "the widget" {
		t.Errorf("quotedFileContent = %+v, want value 'the widget'", q)
	}
	if _, ok := q["mimeType"]; ok {
		t.Errorf("quotedFileContent should only carry value, got %+v", q)
	}
	if c["anchor"] != `{"r":"head"}` {
		t.Errorf("anchor = %v, want passthrough", c["anchor"])
	}
	if c["resolved"] != false {
		t.Errorf("resolved = %v, want false", c["resolved"])
	}
	for _, k := range []string{"content", "createdTime", "modifiedTime"} {
		if _, ok := c[k]; !ok {
			t.Errorf("missing key %q in %+v", k, c)
		}
	}
	replies := c["replies"].([]interface{})
	if len(replies) != 1 {
		t.Fatalf("expected deleted reply to be dropped, got %+v", replies)
	}
	r := replies[0].(map[string]interface{})
	if r["id"] != "R1" || r["content"] != "Yes" {
		t.Errorf("reply = %+v", r)
	}
}

func TestRunDriveCommentsList_IncludeFlags(t *testing.T) {
	cases := []struct {
		name                            string
		includeResolved, includeDeleted bool
		wantIDs                         []string
		wantC1Replies                   int
	}{
		{"default", false, false, []string{"C1"}, 1},
		{"resolved", true, false, []string{"C1", "C2"}, 1},
		{"deleted", false, true, []string{"C1", "C3"}, 2},
		{"both", true, true, []string{"C1", "C2", "C3"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var queries []map[string][]string
			conn := fakeCommentsList(t, &queries)
			var buf bytes.Buffer
			out := &outputWriter{json: true, writer: &buf}
			if err := runDriveCommentsList(context.Background(), conn, "F1", tc.includeResolved, tc.includeDeleted, out); err != nil {
				t.Fatalf("runDriveCommentsList() error = %v", err)
			}
			want := "false"
			if tc.includeDeleted {
				want = "true"
			}
			if got := strings.Join(queries[0]["includeDeleted"], ","); got != want {
				t.Errorf("includeDeleted = %q, want %q", got, want)
			}
			var got []driveCommentOutput
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
			}
			var ids []string
			for _, c := range got {
				ids = append(ids, c.ID)
			}
			if strings.Join(ids, ",") != strings.Join(tc.wantIDs, ",") {
				t.Errorf("ids = %v, want %v", ids, tc.wantIDs)
			}
			if len(got[0].Replies) != tc.wantC1Replies {
				t.Errorf("C1 replies = %d, want %d", len(got[0].Replies), tc.wantC1Replies)
			}
		})
	}
}

func TestRunDriveCommentsList_Table(t *testing.T) {
	var queries []map[string][]string
	conn := fakeCommentsList(t, &queries)
	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}
	if err := runDriveCommentsList(context.Background(), conn, "F1", true, false, out); err != nil {
		t.Fatalf("runDriveCommentsList() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header + 2 rows, got:\n%s", buf.String())
	}
	if !strings.Contains(lines[0], "REPLIES") {
		t.Errorf("header missing REPLIES: %q", lines[0])
	}
	// Multi-line content is flattened onto one row; reply count is shown.
	if !strings.Contains(lines[1], "Should this be plural?") || !strings.Contains(lines[1], " 1 ") {
		t.Errorf("C1 row = %q", lines[1])
	}
	if !strings.Contains(lines[2], "C2") || !strings.Contains(lines[2], "yes") {
		t.Errorf("C2 row = %q", lines[2])
	}
}

func TestRunDriveCommentsList_Empty(t *testing.T) {
	conn, err := gwcli.NewFake(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"comments":[]}`), nil
	})})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveCommentsList(context.Background(), conn, "F1", false, false, out); err != nil {
		t.Fatalf("runDriveCommentsList() error = %v", err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("empty list output = %q, want []", buf.String())
	}
}

// fakeCommentWrite records the method, path, fields mask and JSON body of
// every request and answers with resp.
func fakeCommentWrite(t *testing.T, resp string, method, path, fields *string, body *map[string]interface{}) *gwcli.CmdG {
	t.Helper()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			*method = req.Method
			*path = req.URL.Path
			*fields = req.URL.Query().Get("fields")
			*body = map[string]interface{}{}
			if req.Body != nil {
				_ = json.NewDecoder(req.Body).Decode(body)
			}
			return jsonResponse(resp), nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	return conn
}

func TestRunDriveCommentsAdd(t *testing.T) {
	var method, path, fields string
	var body map[string]interface{}
	conn := fakeCommentWrite(t, `{"id":"C9","content":"Looks good","author":{"displayName":"Agent"}}`,
		&method, &path, &fields, &body)

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveCommentsAdd(context.Background(), conn, "F1", "Looks good", out); err != nil {
		t.Fatalf("runDriveCommentsAdd() error = %v", err)
	}
	if method != "POST" || !strings.HasSuffix(path, "/drive/v3/files/F1/comments") {
		t.Errorf("request = %s %s, want POST .../files/F1/comments", method, path)
	}
	if fields == "" {
		t.Errorf("fields mask not set")
	}
	if body["content"] != "Looks good" {
		t.Errorf("body = %+v, want content", body)
	}
	if _, ok := body["anchor"]; ok {
		t.Errorf("add must not construct an anchor: %+v", body)
	}
	var got driveCommentOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if got.ID != "C9" || got.Content != "Looks good" {
		t.Errorf("output = %+v", got)
	}
}

func TestRunDriveCommentsAdd_RequiresContent(t *testing.T) {
	called := false
	conn, err := gwcli.NewFake(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return jsonResponse(`{}`), nil
	})})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	out := &outputWriter{json: true, writer: &bytes.Buffer{}}
	if err := runDriveCommentsAdd(context.Background(), conn, "F1", "  ", out); err == nil {
		t.Fatalf("expected error for empty content")
	}
	if called {
		t.Errorf("no API call expected for empty content")
	}
}

func TestRunDriveCommentsReply(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		resolve    bool
		wantAction string
		wantErr    bool
	}{
		{"plain", "Fixed in v2", false, "", false},
		{"resolve with text", "Fixed", true, "resolve", false},
		{"resolve without text", "", true, "resolve", false},
		{"no text no resolve", "", false, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var method, path, fields string
			var body map[string]interface{}
			resp := `{"id":"R9","content":"` + tc.content + `","action":"` + tc.wantAction + `"}`
			conn := fakeCommentWrite(t, resp, &method, &path, &fields, &body)
			var buf bytes.Buffer
			out := &outputWriter{json: true, writer: &buf}
			err := runDriveCommentsReply(context.Background(), conn, "F1", "C1", tc.content, tc.resolve, out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if method != "" {
					t.Errorf("no API call expected, got %s %s", method, path)
				}
				return
			}
			if err != nil {
				t.Fatalf("runDriveCommentsReply() error = %v", err)
			}
			if method != "POST" || !strings.HasSuffix(path, "/drive/v3/files/F1/comments/C1/replies") {
				t.Errorf("request = %s %s", method, path)
			}
			if fields == "" {
				t.Errorf("fields mask not set")
			}
			if got, _ := body["action"].(string); got != tc.wantAction {
				t.Errorf("action = %q, want %q", got, tc.wantAction)
			}
			if got, _ := body["content"].(string); got != tc.content {
				t.Errorf("content = %q, want %q", got, tc.content)
			}
			var got map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
			}
			if got["commentId"] != "C1" || got["fileId"] != "F1" || got["resolved"] != tc.resolve {
				t.Errorf("output = %+v", got)
			}
		})
	}
}

func TestReadCommentContent(t *testing.T) {
	got, err := readCommentContent("-", strings.NewReader("line one\nline two\n\n"))
	if err != nil || got != "line one\nline two" {
		t.Errorf("readCommentContent(-) = %q, %v", got, err)
	}
	got, err = readCommentContent("literal", strings.NewReader("ignored"))
	if err != nil || got != "literal" {
		t.Errorf("readCommentContent(literal) = %q, %v", got, err)
	}
}

func TestParseDriveCommentsCommands(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"drive", "comments", "list", "F1", "--include-resolved"}, "drive comments list <file>"},
		{[]string{"drive", "comments", "add", "F1", "--content", "hi"}, "drive comments add <file>"},
		{[]string{"drive", "comments", "reply", "F1", "C1", "--resolve"}, "drive comments reply <file> <comment-id>"},
		{[]string{"drive", "comments", "resolve", "F1", "C1"}, "drive comments resolve <file> <comment-id>"},
	}
	for _, tc := range cases {
		var cli CLI
		parser, err := kong.New(&cli, kong.Name("gwcli"), kong.Exit(func(int) {}))
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		ctx, err := parser.Parse(tc.args)
		if err != nil {
			t.Fatalf("Parse(%v): %v", tc.args, err)
		}
		if ctx.Command() != tc.want {
			t.Errorf("Parse(%v) command = %q, want %q", tc.args, ctx.Command(), tc.want)
		}
	}
	// add requires --content.
	var cli CLI
	parser, _ := kong.New(&cli, kong.Name("gwcli"), kong.Exit(func(int) {}))
	if _, err := parser.Parse([]string{"drive", "comments", "add", "F1"}); err == nil {
		t.Errorf("drive comments add without --content: expected parse error")
	}
}
