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
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "/drive/v3/files") {
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
	if err := runDriveList(context.Background(), conn, "", "", 100, out); err != nil {
		t.Fatalf("runDriveList() error = %v", err)
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
	if err := runDriveUpload(context.Background(), conn, src, "FOLDER1", "", out); err != nil {
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
