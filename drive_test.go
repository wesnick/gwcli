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

	if err := runDriveExport(context.Background(), conn, "DOC123", dir, outPath, out); err != nil {
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
