package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
)

func TestResolveDriveRef(t *testing.T) {
	// A Drive/Docs URL is parsed into its ID and type.
	art, err := resolveDriveRef("https://docs.google.com/document/d/DOC123/edit?usp=sharing")
	if err != nil {
		t.Fatalf("resolveDriveRef(url) error = %v", err)
	}
	if art.ID != "DOC123" || art.Type != "document" {
		t.Fatalf("resolveDriveRef(url) = %+v, want ID=DOC123 type=document", art)
	}

	// A bare file ID is passed through as-is.
	art, err = resolveDriveRef("RAWFILEID")
	if err != nil {
		t.Fatalf("resolveDriveRef(id) error = %v", err)
	}
	if art.ID != "RAWFILEID" || art.Type != "drive-file" {
		t.Fatalf("resolveDriveRef(id) = %+v, want ID=RAWFILEID type=drive-file", art)
	}

	if _, err := resolveDriveRef(""); err == nil {
		t.Fatalf("resolveDriveRef(\"\") expected error, got nil")
	}
}

func TestRunDriveGet(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/drive/v3/files/DOC123") {
				meta := `{"id":"DOC123","name":"Quarterly Plan",` +
					`"mimeType":"application/vnd.google-apps.document",` +
					`"modifiedTime":"2026-01-02T03:04:05Z",` +
					`"owners":[{"displayName":"Alice","emailAddress":"alice@example.com"}]}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(meta))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveGet(context.Background(), conn, "https://docs.google.com/document/d/DOC123/edit", out); err != nil {
		t.Fatalf("runDriveGet() error = %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, buf.String())
	}
	if got["id"] != "DOC123" || got["name"] != "Quarterly Plan" {
		t.Fatalf("unexpected metadata: %+v", got)
	}
}

func TestRunDriveExport_NativeDoc(t *testing.T) {
	const exported = "# Quarterly Plan\n\nbody\n"
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/export"):
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(exported))}, nil
			case strings.Contains(req.URL.Path, "/drive/v3/files/DOC123"):
				meta := `{"id":"DOC123","name":"Quarterly Plan","mimeType":"application/vnd.google-apps.document"}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(meta))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "plan.md")
	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}

	if err := runDriveExport(context.Background(), conn, "DOC123", "", dir, outPath, out); err != nil {
		t.Fatalf("runDriveExport() error = %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != exported {
		t.Fatalf("exported content = %q, want %q", string(data), exported)
	}
}

func TestRunDriveExport_FormatOverride(t *testing.T) {
	var exportMIME string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/export"):
				exportMIME = req.URL.Query().Get("mimeType")
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("PDFDATA"))}, nil
			case strings.Contains(req.URL.Path, "/drive/v3/files/DOC123"):
				meta := `{"id":"DOC123","name":"Plan","mimeType":"application/vnd.google-apps.document"}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(meta))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	dir := t.TempDir()
	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}
	if err := runDriveExport(context.Background(), conn, "DOC123", "pdf", dir, "", out); err != nil {
		t.Fatalf("runDriveExport() error = %v", err)
	}
	if exportMIME != "application/pdf" {
		t.Fatalf("export mimeType = %q, want application/pdf", exportMIME)
	}
}

func TestRunDriveList(t *testing.T) {
	var gotQuery string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "/drive/v3/files") {
				gotQuery = req.URL.Query().Get("q")
				body := `{"files":[{"id":"F1","name":"a.txt","mimeType":"text/plain","size":"12"},` +
					`{"id":"F2","name":"b.pdf","mimeType":"application/pdf","size":"34"}]}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveList(context.Background(), conn, "", "FOLDER1", 100, out); err != nil {
		t.Fatalf("runDriveList() error = %v", err)
	}
	if !strings.Contains(gotQuery, "trashed = false") {
		t.Fatalf("list query %q missing trashed = false filter", gotQuery)
	}
	var got []driveListFile
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if len(got) != 2 || got[0].ID != "F1" || got[1].Name != "b.pdf" {
		t.Fatalf("unexpected list: %+v", got)
	}
}

func TestRunDriveUpload(t *testing.T) {
	var gotName string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/upload/drive/v3/files") || strings.HasSuffix(req.URL.Path, "/drive/v3/files") {
				resp := `{"id":"NEW1","name":"hello.txt","mimeType":"text/plain","size":"5"}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(resp))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveUpload(context.Background(), conn, []string{src}, "FOLDER1", "", false, false, "", out); err != nil {
		t.Fatalf("runDriveUpload() error = %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if got["id"] != "NEW1" {
		t.Fatalf("unexpected upload result: %+v", got)
	}
	_ = gotName
}

func TestDriveQuoteValue(t *testing.T) {
	if got := driveQuoteValue(`a'b`); got != `a\'b` {
		t.Errorf("driveQuoteValue(a'b) = %q, want a\\'b", got)
	}
	if got := driveQuoteValue(`a\b`); got != `a\\b` {
		t.Errorf("driveQuoteValue(a\\b) = %q, want a\\\\b", got)
	}
}

func TestRunDriveMkdir_DedupeReusesExisting(t *testing.T) {
	createCalled := false
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/drive/v3/files"):
				body := `{"files":[{"id":"EXIST1","name":"Intern Package",` +
					`"mimeType":"application/vnd.google-apps.folder"}]}`
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			case req.Method == "POST" && strings.HasSuffix(req.URL.Path, "/drive/v3/files"):
				createCalled = true
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"NEW"}`))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveMkdir(context.Background(), conn, "Intern Package", "", false, out); err != nil {
		t.Fatalf("runDriveMkdir() error = %v", err)
	}
	if createCalled {
		t.Fatalf("expected dedupe to reuse existing folder, but Create was called")
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if got["id"] != "EXIST1" {
		t.Fatalf("mkdir result id = %v, want EXIST1", got["id"])
	}
}

func TestRunDriveMove_Reparents(t *testing.T) {
	var addParents, removeParents string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == "GET" && strings.Contains(req.URL.Path, "/drive/v3/files/F1"):
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"F1","parents":["OLD"]}`))}, nil
			case req.Method == "PATCH" && strings.Contains(req.URL.Path, "/drive/v3/files/F1"):
				addParents = req.URL.Query().Get("addParents")
				removeParents = req.URL.Query().Get("removeParents")
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"F1","name":"x","parents":["DEST"]}`))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveMove(context.Background(), conn, "F1", "DEST", out); err != nil {
		t.Fatalf("runDriveMove() error = %v", err)
	}
	if addParents != "DEST" || removeParents != "OLD" {
		t.Fatalf("reparent = add %q remove %q, want add DEST remove OLD", addParents, removeParents)
	}
}

func TestRunDriveRm_TrashRequiresForce(t *testing.T) {
	conn, err := gwcli.NewFake(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"F1","name":"x"}`))}, nil
	})})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveRm(context.Background(), conn, "F1", false, false, out); err == nil {
		t.Fatalf("runDriveRm without --force expected error, got nil")
	}
}

func TestRunDriveShare_AnyoneNeedsNoEmail(t *testing.T) {
	var sentType, sentRole string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" && strings.Contains(req.URL.Path, "/permissions") {
				var p map[string]interface{}
				_ = json.NewDecoder(req.Body).Decode(&p)
				sentType, _ = p["type"].(string)
				sentRole, _ = p["role"].(string)
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"P1","type":"anyone","role":"reader"}`))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveShare(context.Background(), conn, "F1", "anyone", "reader", "", "", false, out); err != nil {
		t.Fatalf("runDriveShare() error = %v", err)
	}
	if sentType != "anyone" || sentRole != "reader" {
		t.Fatalf("share sent type=%q role=%q, want anyone/reader", sentType, sentRole)
	}
}

func TestRunDriveUpload_UpsertReplacesExisting(t *testing.T) {
	updatedID := ""
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/drive/v3/files"):
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"files":[{"id":"EXIST","name":"data.csv"}]}`))}, nil
			case strings.Contains(req.URL.Path, "/upload/drive/v3/files/EXIST") || (req.Method == "PATCH" && strings.Contains(req.URL.Path, "/drive/v3/files/EXIST")):
				updatedID = "EXIST"
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"EXIST","name":"data.csv"}`))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(src, []byte("a,b\n1,2\n"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveUpload(context.Background(), conn, []string{src}, "FOLDER", "", false, true, "", out); err != nil {
		t.Fatalf("runDriveUpload(upsert) error = %v", err)
	}
	if updatedID != "EXIST" {
		t.Fatalf("upsert did not update existing file (updatedID=%q)", updatedID)
	}
}

func TestResolveExportFormat(t *testing.T) {
	cases := map[string]struct{ mime, ext string }{
		"pdf":      {"application/pdf", ".pdf"},
		"MD":       {"text/markdown", ".md"},
		"docx":     {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx"},
		"text/csv": {"text/csv", ".csv"},
	}
	for in, want := range cases {
		m, e, ok := resolveExportFormat(in)
		if !ok || m != want.mime || e != want.ext {
			t.Errorf("resolveExportFormat(%q) = (%q,%q,%v), want (%q,%q,true)", in, m, e, ok, want.mime, want.ext)
		}
	}
	if _, _, ok := resolveExportFormat("nonsense"); ok {
		t.Errorf("resolveExportFormat(nonsense) ok = true, want false")
	}
	if _, _, ok := resolveExportFormat(""); ok {
		t.Errorf("resolveExportFormat(\"\") ok = true, want false")
	}
}

func TestResolveDriveTargetMime(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"doc":          "application/vnd.google-apps.document",
		"Document":     "application/vnd.google-apps.document",
		"sheet":        "application/vnd.google-apps.spreadsheet",
		"spreadsheet":  "application/vnd.google-apps.spreadsheet",
		"slides":       "application/vnd.google-apps.presentation",
		"presentation": "application/vnd.google-apps.presentation",
		"drawing":      "application/vnd.google-apps.drawing",
		"form":         "application/vnd.google-apps.form",
		"application/vnd.google-apps.script": "application/vnd.google-apps.script",
	}
	for in, want := range cases {
		got, err := resolveDriveTargetMime(in)
		if err != nil {
			t.Errorf("resolveDriveTargetMime(%q) unexpected error %v", in, err)
		}
		if got != want {
			t.Errorf("resolveDriveTargetMime(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := resolveDriveTargetMime("xlsx"); err == nil {
		t.Errorf("resolveDriveTargetMime(xlsx) err = nil, want error")
	}
	if _, err := resolveDriveTargetMime("application/pdf"); err == nil {
		t.Errorf("resolveDriveTargetMime(application/pdf) err = nil, want error")
	}
}

// TestRunDriveUpload_ConvertCSVToSheet verifies a .csv upload with --convert
// declares the spreadsheet target *and* a text/csv source content type (so
// Drive does not content-sniff it as text/plain and import it as a Doc).
func TestRunDriveUpload_ConvertCSVToSheet(t *testing.T) {
	var body string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/upload/drive/v3/files") {
				b, _ := io.ReadAll(req.Body)
				body = string(b)
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"S1","name":"data.csv","mimeType":"application/vnd.google-apps.spreadsheet"}`))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(src, []byte("a,b\n1,2\n"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveUpload(context.Background(), conn, []string{src}, "FOLDER", "", true, false, "", out); err != nil {
		t.Fatalf("runDriveUpload(convert) error = %v", err)
	}
	if !strings.Contains(body, "application/vnd.google-apps.spreadsheet") {
		t.Fatalf("upload body missing spreadsheet target mimeType: %s", body)
	}
	if !strings.Contains(body, "text/csv") {
		t.Fatalf("upload body missing text/csv source content type: %s", body)
	}
}

// TestRunDriveUpload_AsOverride verifies --as forces the target type even
// without --convert and overrides extension inference.
func TestRunDriveUpload_AsOverride(t *testing.T) {
	var body string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/upload/drive/v3/files") {
				b, _ := io.ReadAll(req.Body)
				body = string(b)
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"id":"D1","name":"notes.csv","mimeType":"application/vnd.google-apps.document"}`))}, nil
			}
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "notes.csv")
	if err := os.WriteFile(src, []byte("a,b\n1,2\n"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveUpload(context.Background(), conn, []string{src}, "FOLDER", "", false, false, "doc", out); err != nil {
		t.Fatalf("runDriveUpload(as=doc) error = %v", err)
	}
	if !strings.Contains(body, "application/vnd.google-apps.document") {
		t.Fatalf("upload body missing document target mimeType from --as: %s", body)
	}
}
