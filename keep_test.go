package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	keep "google.golang.org/api/keep/v1"
	"google.golang.org/api/option"
)

// fakeKeepClient is an in-memory keepClient for handler tests.
type fakeKeepClient struct {
	pages      []*keep.ListNotesResponse // returned in order, keyed by call count
	listCalls  []fakeListCall
	notes      map[string]*keep.Note
	created    *keep.Note
	deleted    []string
	err        error
	createResp *keep.Note
}

type fakeListCall struct {
	filter    string
	pageSize  int64
	pageToken string
}

func (f *fakeKeepClient) ListNotes(_ context.Context, filter string, pageSize int64, pageToken string) (*keep.ListNotesResponse, error) {
	f.listCalls = append(f.listCalls, fakeListCall{filter, pageSize, pageToken})
	if f.err != nil {
		return nil, f.err
	}
	i := len(f.listCalls) - 1
	if i >= len(f.pages) {
		return &keep.ListNotesResponse{}, nil
	}
	return f.pages[i], nil
}

func (f *fakeKeepClient) GetNote(_ context.Context, name string) (*keep.Note, error) {
	if f.err != nil {
		return nil, f.err
	}
	n, ok := f.notes[name]
	if !ok {
		return nil, &googleapi.Error{Code: 404, Message: "not found"}
	}
	return n, nil
}

func (f *fakeKeepClient) CreateNote(_ context.Context, note *keep.Note) (*keep.Note, error) {
	f.created = note
	if f.err != nil {
		return nil, f.err
	}
	if f.createResp != nil {
		return f.createResp, nil
	}
	resp := *note
	resp.Name = "notes/new123"
	return &resp, nil
}

func (f *fakeKeepClient) DeleteNote(_ context.Context, name string) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, name)
	return nil
}

func textNote(id, title, text string) *keep.Note {
	return &keep.Note{
		Name:       "notes/" + id,
		Title:      title,
		Body:       &keep.Section{Text: &keep.TextContent{Text: text}},
		CreateTime: "2026-01-02T03:04:05Z",
		UpdateTime: "2026-01-03T03:04:05Z",
	}
}

func checklistNote(id, title string) *keep.Note {
	return &keep.Note{
		Name:  "notes/" + id,
		Title: title,
		Body: &keep.Section{List: &keep.ListContent{ListItems: []*keep.ListItem{
			{Text: &keep.TextContent{Text: "Milk"}, Checked: true},
			{Text: &keep.TextContent{Text: "Produce"}, ChildListItems: []*keep.ListItem{
				{Text: &keep.TextContent{Text: "Apples"}},
			}},
		}}},
		UpdateTime: "2026-02-01T00:00:00Z",
	}
}

func jsonOut() (*outputWriter, *bytes.Buffer) {
	var buf bytes.Buffer
	return &outputWriter{json: true, writer: &buf}, &buf
}

func textOut() (*outputWriter, *bytes.Buffer) {
	var buf bytes.Buffer
	return &outputWriter{writer: &buf}, &buf
}

func TestKeepNoteName(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "abc", want: "notes/abc"},
		{in: "notes/abc", want: "notes/abc"},
		{in: "  abc  ", want: "notes/abc"},
		{in: "", wantErr: true},
		{in: "notes/", wantErr: true},
		{in: "notes/a/b", wantErr: true},
	}
	for _, c := range cases {
		got, err := keepNoteName(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("keepNoteName(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("keepNoteName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseKeepCommands(t *testing.T) {
	cases := []struct {
		args    []string
		command string
		check   func(t *testing.T, cli *CLI)
	}{
		{
			args:    []string{"keep", "list", "--filter", "trashed = true", "--limit", "5"},
			command: "keep list",
			check: func(t *testing.T, cli *CLI) {
				if cli.Keep.List.Filter != "trashed = true" || cli.Keep.List.Limit != 5 {
					t.Errorf("list flags = %+v", cli.Keep.List)
				}
			},
		},
		{
			args:    []string{"keep", "list"},
			command: "keep list",
			check: func(t *testing.T, cli *CLI) {
				if cli.Keep.List.Limit != 100 {
					t.Errorf("default limit = %d, want 100", cli.Keep.List.Limit)
				}
			},
		},
		{
			args:    []string{"keep", "get", "abc"},
			command: "keep get <note-id>",
			check: func(t *testing.T, cli *CLI) {
				if cli.Keep.Get.NoteID != "abc" {
					t.Errorf("note id = %q", cli.Keep.Get.NoteID)
				}
			},
		},
		{
			// Items containing commas must not be split (sep:"none").
			args:    []string{"keep", "create", "--title", "Groceries", "--checklist", "Milk, 2%", "--checklist", "Eggs"},
			command: "keep create",
			check: func(t *testing.T, cli *CLI) {
				want := []string{"Milk, 2%", "Eggs"}
				got := cli.Keep.Create.Checklist
				if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
					t.Errorf("checklist = %q, want %q", got, want)
				}
			},
		},
		{
			args:    []string{"--impersonate", "alice@example.com", "keep", "delete", "abc", "--force"},
			command: "keep delete <note-id>",
			check: func(t *testing.T, cli *CLI) {
				if cli.User != "alice@example.com" || !cli.Keep.Delete.Force {
					t.Errorf("user = %q force = %v", cli.User, cli.Keep.Delete.Force)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			t.Setenv("GWCLI_USER", "")
			var cli CLI
			parser, err := kong.New(&cli, kong.Name("gwcli"), kong.Exit(func(int) {}))
			if err != nil {
				t.Fatalf("kong.New: %v", err)
			}
			ctx, err := parser.Parse(c.args)
			if err != nil {
				t.Fatalf("Parse(%q): %v", c.args, err)
			}
			if ctx.Command() != c.command {
				t.Errorf("Command() = %q, want %q", ctx.Command(), c.command)
			}
			c.check(t, &cli)
		})
	}
}

func TestParseUserFromEnv(t *testing.T) {
	t.Setenv("GWCLI_USER", "env@example.com")
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("gwcli"), kong.Exit(func(int) {}))
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	if _, err := parser.Parse([]string{"keep", "list"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cli.User != "env@example.com" {
		t.Errorf("User = %q, want env@example.com", cli.User)
	}
}

func TestRunKeepListPaginatesAndLimits(t *testing.T) {
	fake := &fakeKeepClient{pages: []*keep.ListNotesResponse{
		{Notes: []*keep.Note{textNote("a", "A", "x"), textNote("b", "B", "y")}, NextPageToken: "p2"},
		{Notes: []*keep.Note{checklistNote("c", "C"), textNote("d", "D", "z")}, NextPageToken: "p3"},
	}}
	out, buf := jsonOut()
	if err := runKeepList(context.Background(), fake, "trashed = false", 3, out); err != nil {
		t.Fatalf("runKeepList: %v", err)
	}

	if len(fake.listCalls) != 2 {
		t.Fatalf("list calls = %d, want 2", len(fake.listCalls))
	}
	if c := fake.listCalls[0]; c.pageSize != 3 || c.pageToken != "" || c.filter != "trashed = false" {
		t.Errorf("first call = %+v", c)
	}
	if c := fake.listCalls[1]; c.pageSize != 1 || c.pageToken != "p2" {
		t.Errorf("second call = %+v", c)
	}

	var got []keepNoteOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.String())
	}
	if len(got) != 3 {
		t.Fatalf("got %d notes, want 3", len(got))
	}
	if got[0].ID != "a" || got[0].Name != "notes/a" || got[0].Type != "text" || got[0].Text != "x" {
		t.Errorf("note[0] = %+v", got[0])
	}
	if got[2].Type != "checklist" || len(got[2].Items) != 2 {
		t.Errorf("note[2] = %+v", got[2])
	}
}

func TestRunKeepListNoLimitFollowsAllPages(t *testing.T) {
	fake := &fakeKeepClient{pages: []*keep.ListNotesResponse{
		{Notes: []*keep.Note{textNote("a", "A", "x")}, NextPageToken: "p2"},
		{Notes: []*keep.Note{textNote("b", "", "untitled body")}},
	}}
	out, buf := textOut()
	if err := runKeepList(context.Background(), fake, "", 0, out); err != nil {
		t.Fatalf("runKeepList: %v", err)
	}
	if len(fake.listCalls) != 2 || fake.listCalls[0].pageSize != keepMaxPageSize {
		t.Errorf("list calls = %+v", fake.listCalls)
	}
	s := buf.String()
	for _, want := range []string{"ID", "TYPE", "TITLE", "UPDATED", "a", "(untitled body)", "2026-01-03"} {
		if !strings.Contains(s, want) {
			t.Errorf("table missing %q:\n%s", want, s)
		}
	}
}

func TestRunKeepListEmpty(t *testing.T) {
	out, buf := jsonOut()
	if err := runKeepList(context.Background(), &fakeKeepClient{}, "", 10, out); err != nil {
		t.Fatalf("runKeepList: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("empty JSON = %q, want []", buf.String())
	}
}

func TestRunKeepListRejectsNegativeLimit(t *testing.T) {
	out, _ := jsonOut()
	if err := runKeepList(context.Background(), &fakeKeepClient{}, "", -1, out); err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestRunKeepGetChecklistJSON(t *testing.T) {
	fake := &fakeKeepClient{notes: map[string]*keep.Note{"notes/c": checklistNote("c", "Groceries")}}
	out, buf := jsonOut()
	if err := runKeepGet(context.Background(), fake, "c", out); err != nil {
		t.Fatalf("runKeepGet: %v", err)
	}
	var got keepNoteOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Type != "checklist" || got.Title != "Groceries" {
		t.Fatalf("note = %+v", got)
	}
	if !got.Items[0].Checked || got.Items[0].Text != "Milk" {
		t.Errorf("item[0] = %+v", got.Items[0])
	}
	if len(got.Items[1].Children) != 1 || got.Items[1].Children[0].Text != "Apples" {
		t.Errorf("item[1] children = %+v", got.Items[1].Children)
	}
}

func TestRunKeepGetText(t *testing.T) {
	fake := &fakeKeepClient{notes: map[string]*keep.Note{
		"notes/a": textNote("a", "Ideas", "line one\nline two"),
		"notes/c": checklistNote("c", "Groceries"),
	}}

	out, buf := textOut()
	if err := runKeepGet(context.Background(), fake, "notes/a", out); err != nil {
		t.Fatalf("runKeepGet: %v", err)
	}
	s := buf.String()
	for _, want := range []string{"Title:   Ideas", "ID:      a", "Type:    text", "line one\nline two"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}

	out, buf = textOut()
	if err := runKeepGet(context.Background(), fake, "c", out); err != nil {
		t.Fatalf("runKeepGet: %v", err)
	}
	s = buf.String()
	for _, want := range []string{"[x] Milk", "[ ] Produce", "    [ ] Apples"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
}

func TestRunKeepGetNotFound(t *testing.T) {
	out, _ := jsonOut()
	err := runKeepGet(context.Background(), &fakeKeepClient{}, "missing", out)
	if err == nil || !strings.Contains(err.Error(), "failed to get note") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildKeepNote(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		n, err := buildKeepNote(keepCreateOptions{title: "T", text: "hello"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(n)
		if want := `{"body":{"text":{"text":"hello"}},"title":"T"}`; string(b) != want {
			t.Errorf("payload = %s, want %s", b, want)
		}
	})
	t.Run("text from stdin", func(t *testing.T) {
		n, err := buildKeepNote(keepCreateOptions{text: "-"}, strings.NewReader("from stdin\n\n"))
		if err != nil {
			t.Fatal(err)
		}
		if n.Body.Text.Text != "from stdin" {
			t.Errorf("text = %q", n.Body.Text.Text)
		}
	})
	t.Run("empty stdin", func(t *testing.T) {
		if _, err := buildKeepNote(keepCreateOptions{text: "-"}, strings.NewReader("\n")); err == nil {
			t.Error("expected error for empty stdin")
		}
	})
	t.Run("checklist", func(t *testing.T) {
		n, err := buildKeepNote(keepCreateOptions{title: "G", checklist: []string{"Milk, 2%", "Eggs"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(n)
		want := `{"body":{"list":{"listItems":[{"text":{"text":"Milk, 2%"}},{"text":{"text":"Eggs"}}]}},"title":"G"}`
		if string(b) != want {
			t.Errorf("payload = %s, want %s", b, want)
		}
	})
	t.Run("neither", func(t *testing.T) {
		if _, err := buildKeepNote(keepCreateOptions{title: "T"}, nil); err == nil {
			t.Error("expected error when neither --text nor --checklist given")
		}
	})
	t.Run("both", func(t *testing.T) {
		if _, err := buildKeepNote(keepCreateOptions{text: "a", checklist: []string{"b"}}, nil); err == nil {
			t.Error("expected error when both --text and --checklist given")
		}
	})
	t.Run("blank item", func(t *testing.T) {
		if _, err := buildKeepNote(keepCreateOptions{checklist: []string{"a", " "}}, nil); err == nil {
			t.Error("expected error for blank checklist item")
		}
	})
}

func TestRunKeepCreate(t *testing.T) {
	fake := &fakeKeepClient{}
	out, buf := jsonOut()
	opts := keepCreateOptions{title: "Groceries", checklist: []string{"Milk", "Eggs"}}
	if err := runKeepCreate(context.Background(), fake, opts, nil, out); err != nil {
		t.Fatalf("runKeepCreate: %v", err)
	}
	if fake.created == nil || fake.created.Title != "Groceries" || len(fake.created.Body.List.ListItems) != 2 {
		t.Fatalf("created = %+v", fake.created)
	}
	var got keepNoteOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != "new123" || got.Type != "checklist" || len(got.Items) != 2 {
		t.Errorf("output = %+v", got)
	}

	out, buf = textOut()
	if err := runKeepCreate(context.Background(), &fakeKeepClient{}, keepCreateOptions{title: "T", text: "x"}, nil, out); err != nil {
		t.Fatalf("runKeepCreate: %v", err)
	}
	if !strings.Contains(buf.String(), `Created text note "T" (ID: new123)`) {
		t.Errorf("output = %q", buf.String())
	}
}

func TestRunKeepCreateValidatesBeforeAPICall(t *testing.T) {
	fake := &fakeKeepClient{}
	out, _ := jsonOut()
	if err := runKeepCreate(context.Background(), fake, keepCreateOptions{title: "T"}, nil, out); err == nil {
		t.Fatal("expected validation error")
	}
	if fake.created != nil {
		t.Error("API must not be called when validation fails")
	}
}

func TestRunKeepDelete(t *testing.T) {
	fake := &fakeKeepClient{}
	out, _ := jsonOut()
	if err := runKeepDelete(context.Background(), fake, "abc", false, out); err == nil {
		t.Fatal("expected error without --force")
	}
	if len(fake.deleted) != 0 {
		t.Fatal("delete must not be called without --force")
	}

	out, buf := jsonOut()
	if err := runKeepDelete(context.Background(), fake, "notes/abc", true, out); err != nil {
		t.Fatalf("runKeepDelete: %v", err)
	}
	if len(fake.deleted) != 1 || fake.deleted[0] != "notes/abc" {
		t.Errorf("deleted = %q", fake.deleted)
	}
	var got map[string]string
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["status"] != "deleted" || got["id"] != "abc" {
		t.Errorf("output = %v", got)
	}
}

func TestWrapKeepErr(t *testing.T) {
	dwd := &oauth2.RetrieveError{Body: []byte(`{"error":"unauthorized_client","error_description":"Client is unauthorized"}`)}
	cases := []struct {
		name     string
		err      error
		want     string
		exitCode int
	}{
		{"dwd scope", fmt.Errorf("Get: %w", dwd), "Domain-wide delegation", 3},
		{"api disabled", &googleapi.Error{Code: 403, Message: "Google Keep API has not been used in project 1 before or it is disabled"}, "keep.googleapis.com", 3},
		{"insufficient scope", &googleapi.Error{Code: 403, Message: "Request had insufficient authentication scopes."}, "Domain-wide delegation", 3},
		{"missing note", &googleapi.Error{Code: 403, Message: "The caller does not have permission"}, "note not found", 2},
		{"other", &googleapi.Error{Code: 404, Message: "not found"}, "not found", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := wrapKeepErr(c.err)
			if !strings.Contains(got.Error(), c.want) {
				t.Errorf("wrapKeepErr = %v, want containing %q", got, c.want)
			}
			if !errors.Is(got, c.err) {
				t.Error("wrapped error must preserve the original via %w")
			}
			if code := keepExitCode(fmt.Errorf("failed: %w", got)); code != c.exitCode {
				t.Errorf("keepExitCode = %d, want %d", code, c.exitCode)
			}
		})
	}
	if wrapKeepErr(nil) != nil {
		t.Error("wrapKeepErr(nil) must be nil")
	}
}

// TestKeepServiceClientRequests checks the adapter against the real
// generated client, stubbing HTTP, so request shapes are verified end to end.
func TestKeepServiceClientRequests(t *testing.T) {
	var reqs []string
	var createBody string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		reqs = append(reqs, req.Method+" "+req.URL.Path+"?"+req.URL.RawQuery)
		body := `{}`
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/notes":
			body = `{"notes":[{"name":"notes/a","title":"A"}],"nextPageToken":"n"}`
		case req.Method == http.MethodGet:
			body = `{"name":"notes/a","title":"A","body":{"text":{"text":"hi"}}}`
		case req.Method == http.MethodPost:
			b, _ := io.ReadAll(req.Body)
			createBody = string(b)
			body = `{"name":"notes/new","title":"T"}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	svc, err := keep.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("keep.NewService: %v", err)
	}
	kc := &keepServiceClient{svc: svc}
	ctx := context.Background()

	resp, err := kc.ListNotes(ctx, "trashed = true", 10, "tok")
	if err != nil || len(resp.Notes) != 1 || resp.NextPageToken != "n" {
		t.Fatalf("ListNotes = %+v, %v", resp, err)
	}
	if _, err := kc.GetNote(ctx, "notes/a"); err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if _, err := kc.CreateNote(ctx, &keep.Note{Title: "T", Body: &keep.Section{Text: &keep.TextContent{Text: "x"}}}); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if err := kc.DeleteNote(ctx, "notes/a"); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}

	if len(reqs) != 4 {
		t.Fatalf("requests = %q", reqs)
	}
	for _, want := range []string{"filter=trashed+%3D+true", "pageSize=10", "pageToken=tok"} {
		if !strings.Contains(reqs[0], want) {
			t.Errorf("list request %q missing %q", reqs[0], want)
		}
	}
	if !strings.HasPrefix(reqs[1], "GET /v1/notes/a") {
		t.Errorf("get request = %q", reqs[1])
	}
	if !strings.HasPrefix(reqs[2], "POST /v1/notes") {
		t.Errorf("create request = %q", reqs[2])
	}
	if !strings.HasPrefix(reqs[3], "DELETE /v1/notes/a") {
		t.Errorf("delete request = %q", reqs[3])
	}
	if !strings.Contains(createBody, `"title":"T"`) || !strings.Contains(createBody, `"text":{"text":"x"}`) {
		t.Errorf("create body = %s", createBody)
	}
}
