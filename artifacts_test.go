package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
)

const testDocID = "1sBJ5Gr6iM4sBL9Afiwu-JXqCkrgNONRf-iPzISZdgYE"

// gmailMessageJSON builds a minimal Gmail API message whose HTML body links a
// Drive doc via an artifact chip.
func gmailMessageJSON(t *testing.T) string {
	t.Helper()
	html := `<div><a href="https://docs.google.com/document/d/` + testDocID +
		`/edit?usp=meet_tnfm_email" class="artifact-chip"><span>Notes by Gemini</span></a></div>`
	msg := map[string]interface{}{
		"id":       "MSG1",
		"threadId": "MSG1",
		"payload": map[string]interface{}{
			"mimeType": "multipart/alternative",
			"headers": []map[string]string{
				{"name": "Subject", "value": "Notes: TFT Status"},
			},
			"parts": []map[string]interface{}{
				{
					"mimeType": "text/html",
					"body":     map[string]interface{}{"data": base64.StdEncoding.EncodeToString([]byte(html))},
				},
			},
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	return string(b)
}

func TestRunArtifactsList(t *testing.T) {
	msgJSON := gmailMessageJSON(t)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "/gmail/v1/users/me/messages/") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(msgJSON)),
				}, nil
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
	if err := runArtifactsList(context.Background(), conn, "MSG1", out); err != nil {
		t.Fatalf("runArtifactsList() error = %v", err)
	}

	var got []driveArtifact
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output: %v (raw: %s)", err, buf.String())
	}
	if len(got) != 1 || got[0].ID != testDocID || got[0].Title != "Notes by Gemini" || got[0].Type != "document" {
		t.Fatalf("unexpected artifacts: %+v", got)
	}
}

func TestRunArtifactsDownload_ExportsNativeDoc(t *testing.T) {
	msgJSON := gmailMessageJSON(t)
	const exported = "# Notes from TFT Status\n\nbody\n"

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(req.URL.Path, "/gmail/v1/users/me/messages/"):
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(msgJSON))}, nil
			case strings.HasSuffix(req.URL.Path, "/export"):
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(exported))}, nil
			case strings.Contains(req.URL.Path, "/drive/v3/files/"+testDocID):
				meta := `{"id":"` + testDocID + `","name":"Notes by Gemini","mimeType":"application/vnd.google-apps.document"}`
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
	outPath := filepath.Join(dir, "notes.md")
	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}

	if err := runArtifactsDownload(context.Background(), conn, "MSG1", []string{"0"}, "", dir, outPath, out); err != nil {
		t.Fatalf("runArtifactsDownload() error = %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != exported {
		t.Fatalf("exported content = %q, want %q", string(data), exported)
	}
}

func TestWrapDriveErr_ServiceAccountDWD(t *testing.T) {
	// The real service-account error when DWD lacks drive.readonly.
	raw := errAuth("auth: cannot fetch token: 401\n" +
		`{"error":"unauthorized_client","error_description":"...not authorized for any of the scopes requested."}`)
	got := wrapDriveErr(raw)
	if !strings.Contains(got.Error(), "drive.readonly") {
		t.Fatalf("expected actionable scope hint, got: %v", got)
	}
}

type errAuth string

func (e errAuth) Error() string { return string(e) }
