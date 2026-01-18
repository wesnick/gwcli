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
)

func TestRunMessagesSearchEmpty(t *testing.T) {
	const emptyJSON = `{
		"messages": [],
		"resultSizeEstimate": 0
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.Contains(req.URL.String(), "/users/me/messages") {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(emptyJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{
		json:   true,
		writer: &buf,
	}

	// Run search with a dummy query
	err = runMessagesSearch(context.Background(), conn, "from:mom", 100, out)
	if err != nil {
		t.Fatalf("runMessagesSearch() error = %v", err)
	}

	outputStr := buf.String()
	// If the bug exists, this will contain "No messages found" and not valid JSON array
	if strings.Contains(outputStr, "No messages found") {
		t.Logf("Bug reproduced: Output contains 'No messages found'")
		// We want the test to fail if we are verifying the fix, but here we are reproducing.
		// I will make the test fail if it finds "No messages found", so running it now confirms failure.
		t.Fatalf("Output should be JSON, but found text 'No messages found'")
	}

	var result []messageListOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v. Output was: %q", err, outputStr)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result))
	}
}

func TestRunMessagesListEmpty(t *testing.T) {
	const emptyJSON = `{
		"messages": [],
		"resultSizeEstimate": 0
	}`

	// Mock labels as well since runMessagesList loads them
	const labelsJSON = `{
		"labels": [
			{"id": "INBOX", "name": "INBOX", "type": "system"},
			{"id": "UNREAD", "name": "UNREAD", "type": "system"}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.String(), "/users/me/labels") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(labelsJSON)),
				}, nil
			}
			if strings.Contains(req.URL.String(), "/users/me/messages") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(emptyJSON)),
				}, nil
			}
			t.Fatalf("unexpected URL: %s", req.URL.String())
			return nil, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{
		json:   true,
		writer: &buf,
	}

	// Run list with INBOX
	err = runMessagesList(context.Background(), conn, "INBOX", 50, false, out)
	if err != nil {
		t.Fatalf("runMessagesList() error = %v", err)
	}

	outputStr := buf.String()
	if strings.Contains(outputStr, "No messages found") {
		t.Fatalf("Output should be JSON, but found text 'No messages found'")
	}

	var result []messageListOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v. Output was: %q", err, outputStr)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result))
	}
}
