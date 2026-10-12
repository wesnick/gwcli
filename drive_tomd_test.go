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

// toMdFake is a fake Drive server for `drive to-md`: DOC1 is a Google Doc in
// folder PARENT with two open comments, one resolved and one deleted.
type toMdFake struct {
	docMeta       string
	existingMD    string // files.list response body for the upsert lookup
	listQuery     string
	created       map[string]interface{}
	createBody    string
	updatedID     string
	trashed       bool
	exportMIME    string
	commentsCalls int
	noComments    bool
}

func (f *toMdFake) conn(t *testing.T) *gwcli.CmdG {
	t.Helper()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		p := req.URL.Path
		switch {
		case strings.HasSuffix(p, "/files/DOC1/comments"):
			f.commentsCalls++
			if f.noComments {
				return jsonResponse(`{"comments":[]}`), nil
			}
			return jsonResponse(`{"comments":[
				{"id":"C1","content":"a"},
				{"id":"C2","content":"b","replies":[{"id":"R1","content":"x"}]},
				{"id":"C3","content":"c","resolved":true},
				{"id":"C4","deleted":true}
			]}`), nil
		case strings.HasSuffix(p, "/files/DOC1/export"):
			f.exportMIME = req.URL.Query().Get("mimeType")
			return jsonResponse("# Plan\n\nbody\n"), nil
		case req.Method == "GET" && strings.HasSuffix(p, "/drive/v3/files/DOC1"):
			return jsonResponse(f.docMeta), nil
		case req.Method == "PATCH" && strings.HasSuffix(p, "/drive/v3/files/DOC1"):
			var body map[string]interface{}
			_ = json.NewDecoder(req.Body).Decode(&body)
			f.trashed, _ = body["trashed"].(bool)
			return jsonResponse(`{"id":"DOC1"}`), nil
		case req.Method == "GET" && strings.HasSuffix(p, "/drive/v3/files"):
			f.listQuery = req.URL.Query().Get("q")
			if f.existingMD == "" {
				return jsonResponse(`{"files":[]}`), nil
			}
			return jsonResponse(f.existingMD), nil
		case req.Method == "POST" && strings.HasSuffix(p, "/upload/drive/v3/files"):
			b, _ := io.ReadAll(req.Body)
			f.createBody = string(b)
			return jsonResponse(`{"id":"MD1","name":"Plan.md","mimeType":"text/markdown",` +
				`"parents":["PARENT"],"headRevisionId":"REV1"}`), nil
		case req.Method == "PATCH" && strings.Contains(p, "/upload/drive/v3/files/"):
			f.updatedID = p[strings.LastIndex(p, "/")+1:]
			return jsonResponse(`{"id":"` + f.updatedID + `","name":"Plan.md","mimeType":"text/markdown"}`), nil
		}
		return notFoundResponse(), nil
	})}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	return conn
}

const planDocMeta = `{"id":"DOC1","name":"Plan","mimeType":"application/vnd.google-apps.document","parents":["PARENT"]}`

func TestRunDriveToMd_CreatesMarkdownInDocParent(t *testing.T) {
	f := &toMdFake{docMeta: planDocMeta}
	var buf, warn bytes.Buffer
	out := &outputWriter{json: true, writer: &buf, errWriter: &warn}
	if err := runDriveToMd(context.Background(), f.conn(t), "https://docs.google.com/document/d/DOC1/edit", "", false, false, out); err != nil {
		t.Fatalf("runDriveToMd() error = %v", err)
	}
	if f.exportMIME != "text/markdown" {
		t.Errorf("export mimeType = %q, want text/markdown", f.exportMIME)
	}
	for _, want := range []string{`"name":"Plan.md"`, `"parents":["PARENT"]`, `"mimeType":"text/markdown"`, "# Plan"} {
		if !strings.Contains(f.createBody, want) {
			t.Errorf("create body missing %s:\n%s", want, f.createBody)
		}
	}
	if strings.Contains(f.createBody, "application/vnd.google-apps.") {
		t.Errorf("migration must not request a Google-apps conversion:\n%s", f.createBody)
	}
	if f.trashed {
		t.Errorf("original trashed without --trash-original")
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if got["id"] != "MD1" || got["sourceId"] != "DOC1" || got["droppedComments"] != float64(2) ||
		got["sourceTrashed"] != false || got["headRevisionId"] != "REV1" {
		t.Errorf("to-md JSON = %+v", got)
	}
	if !strings.Contains(warn.String(), "2 unresolved comment(s)") {
		t.Errorf("warning = %q, want the dropped comment count", warn.String())
	}
}

func TestRunDriveToMd_FolderUpsertTrash(t *testing.T) {
	f := &toMdFake{docMeta: planDocMeta, existingMD: `{"files":[{"id":"OLDMD","name":"Plan.md"}]}`}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf, errWriter: &bytes.Buffer{}}
	if err := runDriveToMd(context.Background(), f.conn(t), "DOC1", "https://drive.google.com/drive/folders/DEST", true, true, out); err != nil {
		t.Fatalf("runDriveToMd() error = %v", err)
	}
	if !strings.Contains(f.listQuery, "'DEST' in parents") || !strings.Contains(f.listQuery, "mimeType = 'text/markdown'") ||
		!strings.Contains(f.listQuery, "name = 'Plan.md'") {
		t.Errorf("upsert lookup query = %q", f.listQuery)
	}
	if f.updatedID != "OLDMD" {
		t.Errorf("upsert updated %q, want OLDMD", f.updatedID)
	}
	if f.createBody != "" {
		t.Errorf("upsert also created a new file")
	}
	if !f.trashed {
		t.Errorf("--trash-original did not trash the Doc")
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if got["sourceTrashed"] != true {
		t.Errorf("sourceTrashed = %v, want true", got["sourceTrashed"])
	}
}

func TestRunDriveToMd_RejectsNonDoc(t *testing.T) {
	f := &toMdFake{docMeta: `{"id":"DOC1","name":"x.md","mimeType":"text/markdown"}`}
	out := &outputWriter{json: true, writer: &bytes.Buffer{}, errWriter: &bytes.Buffer{}}
	if err := runDriveToMd(context.Background(), f.conn(t), "DOC1", "", false, false, out); err == nil {
		t.Fatalf("expected error for a non-Doc source")
	}
	if f.commentsCalls != 0 || f.createBody != "" {
		t.Errorf("non-Doc source must stop before reading comments or writing")
	}
}

func TestRunDriveToMd_NoCommentsNoWarning(t *testing.T) {
	f := &toMdFake{docMeta: planDocMeta, noComments: true}
	var buf, warn bytes.Buffer
	out := &outputWriter{json: false, writer: &buf, errWriter: &warn}
	if err := runDriveToMd(context.Background(), f.conn(t), "DOC1", "", false, false, out); err != nil {
		t.Fatalf("runDriveToMd() error = %v", err)
	}
	if warn.Len() != 0 {
		t.Errorf("unexpected warning with no open comments: %q", warn.String())
	}
	if !strings.Contains(buf.String(), "Migrated: Plan.md (id MD1)") {
		t.Errorf("text output = %q", buf.String())
	}
}

func TestDriveToMdName(t *testing.T) {
	cases := map[string]string{
		"Plan":     "Plan.md",
		"notes.md": "notes.md",
		"NOTES.MD": "NOTES.MD",
		"v1.2":     "v1.2.md",
	}
	for in, want := range cases {
		if got := driveToMdName(in); got != want {
			t.Errorf("driveToMdName(%q) = %q, want %q", in, got, want)
		}
	}
}
