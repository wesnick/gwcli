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

// labelsListJSON is a minimal Users.Labels.List response used to seed the
// connection's label cache in tests.
const labelsListJSON = `{
	"labels": [
		{"id": "INBOX", "name": "INBOX", "type": "system"},
		{"id": "Label_1", "name": "Work", "type": "user"}
	]
}`

// labelsTestClient builds an *http.Client whose RoundTripper answers the
// Users.Labels.List call with labelsListJSON and routes every other Gmail
// request to mutate (typically the create/update/delete call under test).
func labelsTestClient(t *testing.T, mutate func(*http.Request) (*http.Response, error)) *http.Client {
	t.Helper()
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.Contains(req.URL.String(), "gmail.googleapis.com") {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			// Label listing (used by LoadLabels) is a GET on .../labels.
			if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/labels") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(labelsListJSON)),
				}, nil
			}
			return mutate(req)
		}),
	}
}

func TestRunLabelsCreate(t *testing.T) {
	const createdJSON = `{"id": "Label_99", "name": "Urgent", "type": "user", "labelListVisibility": "labelShow", "messageListVisibility": "show"}`

	client := labelsTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost {
			t.Errorf("expected POST for create, got %s", req.Method)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(createdJSON)),
		}, nil
	})

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	if err := runLabelsCreate(context.Background(), conn, "Urgent", "show", "labelShow", out); err != nil {
		t.Fatalf("runLabelsCreate() error = %v", err)
	}

	var result labelListOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if result.ID != "Label_99" {
		t.Errorf("expected ID 'Label_99', got %q", result.ID)
	}
	if result.Name != "Urgent" {
		t.Errorf("expected name 'Urgent', got %q", result.Name)
	}
}

func TestRunLabelsCreateEmptyName(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("HTTP request should not be made with empty name")
			return nil, nil
		}),
	}
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runLabelsCreate(context.Background(), conn, "   ", "", "", out)
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected 'name is required' error, got %q", err.Error())
	}
}

func TestRunLabelsCreateDuplicate(t *testing.T) {
	client := labelsTestClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("create RPC should not be issued for a duplicate name (method=%s)", req.Method)
		return nil, nil
	})
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	// "work" matches the existing "Work" label case-insensitively.
	err = runLabelsCreate(context.Background(), conn, "work", "", "", out)
	if err == nil {
		t.Fatal("expected error for duplicate label, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got %q", err.Error())
	}
}

func TestRunLabelsUpdate(t *testing.T) {
	const patchedJSON = `{"id": "Label_1", "name": "Work Renamed", "type": "user"}`

	var patchedID string
	client := labelsTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch {
			t.Errorf("expected PATCH for update, got %s", req.Method)
		}
		patchedID = req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(patchedJSON)),
		}, nil
	})

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	// Resolve by name "Work" -> ID "Label_1".
	if err := runLabelsUpdate(context.Background(), conn, "Work", "Work Renamed", "", "", out); err != nil {
		t.Fatalf("runLabelsUpdate() error = %v", err)
	}
	if patchedID != "Label_1" {
		t.Errorf("expected PATCH on 'Label_1', got %q", patchedID)
	}

	var result labelListOutput
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if result.Name != "Work Renamed" {
		t.Errorf("expected name 'Work Renamed', got %q", result.Name)
	}
}

func TestRunLabelsUpdateNoChanges(t *testing.T) {
	client := labelsTestClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("no RPC should be issued when nothing changes (method=%s)", req.Method)
		return nil, nil
	})
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runLabelsUpdate(context.Background(), conn, "Work", "", "", "", out)
	if err == nil {
		t.Fatal("expected error when no fields provided, got nil")
	}
	if !strings.Contains(err.Error(), "nothing to update") {
		t.Errorf("expected 'nothing to update' error, got %q", err.Error())
	}
}

func TestRunLabelsDelete(t *testing.T) {
	var deletedID string
	client := labelsTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", req.Method)
		}
		deletedID = req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	})

	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	if err := runLabelsDelete(context.Background(), conn, "Work", true, out); err != nil {
		t.Fatalf("runLabelsDelete() error = %v", err)
	}
	if deletedID != "Label_1" {
		t.Errorf("expected DELETE on 'Label_1', got %q", deletedID)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	if result["status"] != "deleted" || result["id"] != "Label_1" {
		t.Errorf("unexpected delete result: %v", result)
	}
}

func TestRunLabelsDeleteRequiresForce(t *testing.T) {
	client := labelsTestClient(t, func(req *http.Request) (*http.Response, error) {
		t.Fatalf("no RPC should be issued without --force (method=%s)", req.Method)
		return nil, nil
	})
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runLabelsDelete(context.Background(), conn, "Work", false, out)
	if err == nil {
		t.Fatal("expected error without --force, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected '--force' error, got %q", err.Error())
	}
}

func TestRunLabelsDeleteSystemLabel(t *testing.T) {
	client := labelsTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodDelete {
			t.Fatal("system label should not be deleted")
		}
		return nil, nil
	})
	conn, err := gwcli.NewFake(client)
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}

	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}

	err = runLabelsDelete(context.Background(), conn, "INBOX", true, out)
	if err == nil {
		t.Fatal("expected error deleting system label, got nil")
	}
	if !strings.Contains(err.Error(), "system label") {
		t.Errorf("expected 'system label' error, got %q", err.Error())
	}
}
