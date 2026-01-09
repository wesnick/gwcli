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

// roundTripFunc makes it easy to stub HTTP responses in tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRunTasklistsList(t *testing.T) {
	const tasklistsJSON = `{
		"kind": "tasks#taskLists",
		"items": [
			{
				"kind": "tasks#taskList",
				"id": "MTIzNDU2Nzg5MA",
				"title": "My Tasks",
				"updated": "2024-01-15T10:30:00.000Z",
				"selfLink": "https://www.googleapis.com/tasks/v1/users/@me/lists/MTIzNDU2Nzg5MA"
			},
			{
				"kind": "tasks#taskList",
				"id": "OTg3NjU0MzIxMA",
				"title": "Work Tasks",
				"updated": "2024-01-14T08:15:00.000Z",
				"selfLink": "https://www.googleapis.com/tasks/v1/users/@me/lists/OTg3NjU0MzIxMA"
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.Contains(req.URL.String(), "tasks.googleapis.com") {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(tasklistsJSON)),
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

	err = runTasklistsList(context.Background(), conn, out)
	if err != nil {
		t.Fatalf("runTasklistsList() error = %v", err)
	}

	var result []tasklistOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 task lists, got %d", len(result))
	}

	if result[0].ID != "MTIzNDU2Nzg5MA" {
		t.Errorf("expected first task list ID to be 'MTIzNDU2Nzg5MA', got %q", result[0].ID)
	}
	if result[0].Title != "My Tasks" {
		t.Errorf("expected first task list title to be 'My Tasks', got %q", result[0].Title)
	}
	if result[0].Updated != "2024-01-15T10:30:00.000Z" {
		t.Errorf("expected first task list updated to be '2024-01-15T10:30:00.000Z', got %q", result[0].Updated)
	}

	if result[1].ID != "OTg3NjU0MzIxMA" {
		t.Errorf("expected second task list ID to be 'OTg3NjU0MzIxMA', got %q", result[1].ID)
	}
	if result[1].Title != "Work Tasks" {
		t.Errorf("expected second task list title to be 'Work Tasks', got %q", result[1].Title)
	}
}

func TestRunTasklistsListEmpty(t *testing.T) {
	const emptyJSON = `{
		"kind": "tasks#taskLists",
		"items": []
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
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

	err = runTasklistsList(context.Background(), conn, out)
	if err != nil {
		t.Fatalf("runTasklistsList() error = %v", err)
	}

	var result []tasklistOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 task lists, got %d", len(result))
	}
}

func TestRunTasklistsCreate(t *testing.T) {
	const createdJSON = `{
		"kind": "tasks#taskList",
		"id": "TkVXVEFTS0xJU1Q",
		"title": "New Project",
		"updated": "2024-01-16T12:00:00.000Z",
		"selfLink": "https://www.googleapis.com/tasks/v1/users/@me/lists/TkVXVEFTS0xJU1Q"
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != "POST" {
				t.Errorf("expected POST request, got %s", req.Method)
			}
			if !strings.Contains(req.URL.String(), "tasks.googleapis.com") {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(createdJSON)),
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

	err = runTasklistsCreate(context.Background(), conn, "New Project", out)
	if err != nil {
		t.Fatalf("runTasklistsCreate() error = %v", err)
	}

	var result tasklistOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result.ID != "TkVXVEFTS0xJU1Q" {
		t.Errorf("expected task list ID to be 'TkVXVEFTS0xJU1Q', got %q", result.ID)
	}
	if result.Title != "New Project" {
		t.Errorf("expected task list title to be 'New Project', got %q", result.Title)
	}
	if result.Updated != "2024-01-16T12:00:00.000Z" {
		t.Errorf("expected task list updated to be '2024-01-16T12:00:00.000Z', got %q", result.Updated)
	}
}

func TestRunTasklistsCreateEmptyTitle(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("HTTP request should not be made with empty title")
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

	err = runTasklistsCreate(context.Background(), conn, "", out)
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("expected error message to contain 'title is required', got %q", err.Error())
	}
}

func TestRunTasklistsDelete(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != "DELETE" {
				t.Errorf("expected DELETE request, got %s", req.Method)
			}
			if !strings.Contains(req.URL.String(), "tasks.googleapis.com") {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			if !strings.Contains(req.URL.String(), "MTIzNDU2Nzg5MA") {
				t.Errorf("expected URL to contain task list ID 'MTIzNDU2Nzg5MA', got %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
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

	err = runTasklistsDelete(context.Background(), conn, "MTIzNDU2Nzg5MA", false, out)
	if err != nil {
		t.Fatalf("runTasklistsDelete() error = %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if result["deleted"] != "MTIzNDU2Nzg5MA" {
		t.Errorf("expected deleted ID to be 'MTIzNDU2Nzg5MA', got %q", result["deleted"])
	}
}

func TestRunTasklistsDeleteEmptyID(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("HTTP request should not be made with empty ID")
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

	err = runTasklistsDelete(context.Background(), conn, "", false, out)
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
	if !strings.Contains(err.Error(), "ID is required") {
		t.Errorf("expected error message to contain 'ID is required', got %q", err.Error())
	}
}

func TestFormatTaskDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full RFC3339 timestamp",
			input:    "2024-01-15T10:30:00.000Z",
			expected: "2024-01-15",
		},
		{
			name:     "date only",
			input:    "2024-01-15",
			expected: "2024-01-15",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "short string",
			input:    "2024",
			expected: "2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTaskDate(tt.input)
			if result != tt.expected {
				t.Errorf("formatTaskDate(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRunTasklistsListTextOutput(t *testing.T) {
	const tasklistsJSON = `{
		"kind": "tasks#taskLists",
		"items": [
			{
				"kind": "tasks#taskList",
				"id": "MTIzNDU2Nzg5MA",
				"title": "My Tasks",
				"updated": "2024-01-15T10:30:00.000Z"
			}
		]
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(tasklistsJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{
		json:   false,
		writer: &buf,
	}

	err = runTasklistsList(context.Background(), conn, out)
	if err != nil {
		t.Fatalf("runTasklistsList() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "TITLE") {
		t.Errorf("expected output to contain 'TITLE' header, got %q", output)
	}
	if !strings.Contains(output, "My Tasks") {
		t.Errorf("expected output to contain 'My Tasks', got %q", output)
	}
	if !strings.Contains(output, "MTIzNDU2Nzg5MA") {
		t.Errorf("expected output to contain task list ID, got %q", output)
	}
	if !strings.Contains(output, "2024-01-15") {
		t.Errorf("expected output to contain formatted date, got %q", output)
	}
}

func TestRunTasklistsCreateTextOutput(t *testing.T) {
	const createdJSON = `{
		"kind": "tasks#taskList",
		"id": "TkVXVEFTS0xJU1Q",
		"title": "New Project",
		"updated": "2024-01-16T12:00:00.000Z"
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(createdJSON)),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{
		json:   false,
		writer: &buf,
	}

	err = runTasklistsCreate(context.Background(), conn, "New Project", out)
	if err != nil {
		t.Fatalf("runTasklistsCreate() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Created task list") {
		t.Errorf("expected output to contain 'Created task list', got %q", output)
	}
	if !strings.Contains(output, "New Project") {
		t.Errorf("expected output to contain 'New Project', got %q", output)
	}
	if !strings.Contains(output, "TkVXVEFTS0xJU1Q") {
		t.Errorf("expected output to contain task list ID, got %q", output)
	}
}

func TestRunTasklistsDeleteTextOutput(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{
		json:   false,
		writer: &buf,
	}

	err = runTasklistsDelete(context.Background(), conn, "MTIzNDU2Nzg5MA", false, out)
	if err != nil {
		t.Fatalf("runTasklistsDelete() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Deleted task list") {
		t.Errorf("expected output to contain 'Deleted task list', got %q", output)
	}
	if !strings.Contains(output, "MTIzNDU2Nzg5MA") {
		t.Errorf("expected output to contain task list ID, got %q", output)
	}
}
