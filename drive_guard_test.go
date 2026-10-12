package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	drive "google.golang.org/api/drive/v3"

	gwcli "github.com/wesnick/gwcli/pkg/gwcli"
)

func TestCompareDriveWriteGuard(t *testing.T) {
	md := &drive.File{
		Id: "F1", Name: "note.md", MimeType: "text/markdown",
		HeadRevisionId: "REV2", Md5Checksum: "abc123", ModifiedTime: "2026-01-02T03:04:05Z",
	}
	cases := []struct {
		name         string
		file         *drive.File
		guard        driveWriteGuard
		wantErr      bool
		wantConflict bool
	}{
		{"revision match", md, driveWriteGuard{Revision: "REV2"}, false, false},
		{"revision stale", md, driveWriteGuard{Revision: "REV1"}, true, true},
		{"md5 match", md, driveWriteGuard{MD5: "abc123"}, false, false},
		{"md5 match case-insensitive", md, driveWriteGuard{MD5: "ABC123"}, false, false},
		{"md5 stale", md, driveWriteGuard{MD5: "def456"}, true, true},
		{"both match", md, driveWriteGuard{Revision: "REV2", MD5: "abc123"}, false, false},
		{"revision ok md5 stale", md, driveWriteGuard{Revision: "REV2", MD5: "zzz"}, true, true},
		{"native doc unsupported", &drive.File{Id: "D1", Name: "Doc", MimeType: "application/vnd.google-apps.document"},
			driveWriteGuard{Revision: "REV1"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := compareDriveWriteGuard(tc.file, tc.guard)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got := errors.Is(err, errDriveConflict); got != tc.wantConflict {
				t.Fatalf("errors.Is(conflict) = %v, want %v (err %v)", got, tc.wantConflict, err)
			}
			if tc.wantConflict {
				if code := driveExitCode(err); code != 4 {
					t.Errorf("driveExitCode = %d, want 4", code)
				}
				msg := err.Error()
				for _, want := range []string{"REV2", "Re-pull", "merge", "--if-revision REV2"} {
					if !strings.Contains(msg, want) {
						t.Errorf("conflict message %q missing %q", msg, want)
					}
				}
			} else if tc.wantErr && driveExitCode(err) != 2 {
				t.Errorf("non-conflict error should exit 2")
			}
		})
	}
	if driveExitCode(errors.New("boom")) != 2 {
		t.Errorf("driveExitCode(generic) != 2")
	}
}

// fakeGuardedDrive serves metadata for F1 (headRevisionId REV2) and records
// whether any content upload happened.
func fakeGuardedDrive(t *testing.T, metaGets, uploads *int) *gwcli.CmdG {
	t.Helper()
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.Contains(req.URL.Path, "/upload/drive/v3/files"):
				*uploads++
				return jsonResponse(`{"id":"F1","name":"note.md","mimeType":"text/markdown","headRevisionId":"REV3"}`), nil
			case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/drive/v3/files"):
				return jsonResponse(`{"files":[{"id":"F1","name":"note.md"}]}`), nil
			case req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/drive/v3/files/F1"):
				*metaGets++
				if f := req.URL.Query().Get("fields"); !strings.Contains(f, "headRevisionId") {
					t.Errorf("guard metadata fields = %q, want headRevisionId", f)
				}
				return jsonResponse(`{"id":"F1","name":"note.md","mimeType":"text/markdown",` +
					`"headRevisionId":"REV2","md5Checksum":"abc123","modifiedTime":"2026-01-02T03:04:05Z"}`), nil
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

func writeTempMarkdown(t *testing.T) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(src, []byte("# Note\n"), 0644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	return src
}

func TestRunDriveUpdate_Guard(t *testing.T) {
	cases := []struct {
		name         string
		guard        driveWriteGuard
		wantMetaGets int
		wantUploads  int
		wantConflict bool
	}{
		{"no guard skips metadata read", driveWriteGuard{}, 0, 1, false},
		{"fresh revision writes", driveWriteGuard{Revision: "REV2"}, 1, 1, false},
		{"stale revision refuses", driveWriteGuard{Revision: "REV1"}, 1, 0, true},
		{"stale md5 refuses", driveWriteGuard{MD5: "nope"}, 1, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var metaGets, uploads int
			conn := fakeGuardedDrive(t, &metaGets, &uploads)
			var buf bytes.Buffer
			out := &outputWriter{json: true, writer: &buf}
			err := runDriveUpdate(context.Background(), conn, "F1", writeTempMarkdown(t), "", tc.guard, out)
			if got := errors.Is(err, errDriveConflict); got != tc.wantConflict {
				t.Fatalf("conflict = %v, want %v (err %v)", got, tc.wantConflict, err)
			}
			if !tc.wantConflict && err != nil {
				t.Fatalf("runDriveUpdate() error = %v", err)
			}
			if metaGets != tc.wantMetaGets || uploads != tc.wantUploads {
				t.Errorf("metaGets=%d uploads=%d, want %d/%d", metaGets, uploads, tc.wantMetaGets, tc.wantUploads)
			}
			if !tc.wantConflict {
				var got map[string]interface{}
				if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
					t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
				}
				if got["headRevisionId"] != "REV3" {
					t.Errorf("write result headRevisionId = %v, want REV3", got["headRevisionId"])
				}
			}
		})
	}
}

func TestRunDriveUpload_UpsertGuard(t *testing.T) {
	var metaGets, uploads int
	conn := fakeGuardedDrive(t, &metaGets, &uploads)
	out := &outputWriter{json: true, writer: &bytes.Buffer{}}
	src := writeTempMarkdown(t)

	err := runDriveUpload(context.Background(), conn, []string{src}, "FOLDER", "", false, true, "",
		driveWriteGuard{Revision: "REV1"}, out)
	if !errors.Is(err, errDriveConflict) {
		t.Fatalf("stale upsert: err = %v, want conflict", err)
	}
	if uploads != 0 {
		t.Fatalf("stale upsert uploaded content")
	}

	if err := runDriveUpload(context.Background(), conn, []string{src}, "FOLDER", "", false, true, "",
		driveWriteGuard{Revision: "REV2"}, out); err != nil {
		t.Fatalf("fresh upsert: %v", err)
	}
	if uploads != 1 {
		t.Fatalf("fresh upsert uploads = %d, want 1", uploads)
	}
}

func TestRunDriveUpload_GuardValidation(t *testing.T) {
	called := false
	conn, err := gwcli.NewFake(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return notFoundResponse(), nil
	})})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	out := &outputWriter{json: true, writer: &bytes.Buffer{}}
	src := writeTempMarkdown(t)
	guard := driveWriteGuard{Revision: "REV1"}

	if err := runDriveUpload(context.Background(), conn, []string{src}, "", "", false, false, "", guard, out); err == nil {
		t.Errorf("guard without --upsert: expected error")
	}
	if err := runDriveUpload(context.Background(), conn, []string{src, src}, "", "", false, true, "", guard, out); err == nil {
		t.Errorf("guard with multiple paths: expected error")
	}
	if err := runDriveUpload(context.Background(), conn, []string{filepath.Dir(src)}, "", "", false, true, "", guard, out); err == nil {
		t.Errorf("guard with a directory: expected error")
	}
	if called {
		t.Errorf("validation failures must not call the API")
	}
}

func TestRunDriveUpload_UpsertGuardMissingFile(t *testing.T) {
	uploads := 0
	conn, err := gwcli.NewFake(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/upload/") {
			uploads++
		}
		if req.Method == "GET" && strings.HasSuffix(req.URL.Path, "/drive/v3/files") {
			return jsonResponse(`{"files":[]}`), nil
		}
		return jsonResponse(`{"id":"NEW"}`), nil
	})})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	out := &outputWriter{json: true, writer: &bytes.Buffer{}}
	err = runDriveUpload(context.Background(), conn, []string{writeTempMarkdown(t)}, "FOLDER", "", false, true, "",
		driveWriteGuard{Revision: "REV1"}, out)
	if !errors.Is(err, errDriveConflict) {
		t.Fatalf("err = %v, want conflict when the guarded file is gone", err)
	}
	if uploads != 0 {
		t.Fatalf("guarded upsert created a new file")
	}
}

func TestRunDriveGet_RevisionFields(t *testing.T) {
	var fields string
	conn, err := gwcli.NewFake(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		fields = req.URL.Query().Get("fields")
		return jsonResponse(`{"id":"F1","name":"note.md","mimeType":"text/markdown","headRevisionId":"REV2",` +
			`"md5Checksum":"abc123","modifiedTime":"2026-01-02T03:04:05Z",` +
			`"webViewLink":"https://drive.google.com/file/d/F1/view"}`), nil
	})})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveGet(context.Background(), conn, "F1", out); err != nil {
		t.Fatalf("runDriveGet() error = %v", err)
	}
	for _, f := range []string{"headRevisionId", "md5Checksum", "modifiedTime", "webViewLink"} {
		if !strings.Contains(fields, f) {
			t.Errorf("fields mask %q missing %s", fields, f)
		}
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["headRevisionId"] != "REV2" || got["md5Checksum"] != "abc123" ||
		got["webViewLink"] != "https://drive.google.com/file/d/F1/view" {
		t.Errorf("drive get JSON = %+v", got)
	}
}

func TestRunDriveExport_JSONIncludesRevision(t *testing.T) {
	conn, err := gwcli.NewFake(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("alt") == "media" {
			return jsonResponse("# Note\n"), nil
		}
		return jsonResponse(`{"id":"F1","name":"note.md","mimeType":"text/markdown",` +
			`"headRevisionId":"REV2","md5Checksum":"abc123","modifiedTime":"2026-01-02T03:04:05Z"}`), nil
	})})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	dir := t.TempDir()
	if err := runDriveExport(context.Background(), conn, "F1", "", dir, "note.md", out); err != nil {
		t.Fatalf("runDriveExport() error = %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if got["headRevisionId"] != "REV2" || got["md5Checksum"] != "abc123" {
		t.Errorf("export JSON = %+v", got)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "note.md")); string(data) != "# Note\n" {
		t.Errorf("exported content = %q", data)
	}
}

// fakeExportDrive serves F1 with the given metadata JSON and content.
func fakeExportDrive(t *testing.T, meta, content string) *gwcli.CmdG {
	t.Helper()
	conn, err := gwcli.NewFake(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("alt") == "media" || strings.HasSuffix(req.URL.Path, "/export") {
			return jsonResponse(content), nil
		}
		return jsonResponse(meta), nil
	})})
	if err != nil {
		t.Fatalf("NewFake() error = %v", err)
	}
	return conn
}

const markdownMeta = `{"id":"F1","name":"note.md","mimeType":"text/markdown",` +
	`"headRevisionId":"REV2","md5Checksum":"abc123","modifiedTime":"2026-01-02T03:04:05Z"}`

func TestRunDriveExport_Stdout(t *testing.T) {
	const content = "---\ntitle: probe\n---\n# Note\n"
	conn := fakeExportDrive(t, markdownMeta, content)
	t.Chdir(t.TempDir())

	var buf bytes.Buffer
	out := &outputWriter{json: false, writer: &buf}
	if err := runDriveExport(context.Background(), conn, "F1", "", "", "-", out); err != nil {
		t.Fatalf("runDriveExport(-) error = %v", err)
	}
	if buf.String() != content {
		t.Fatalf("stdout = %q, want exactly the content %q (no confirmation)", buf.String(), content)
	}
	if entries, _ := os.ReadDir("."); len(entries) != 0 {
		t.Fatalf("--output - wrote files to the working directory: %v", entries)
	}
}

func TestRunDriveExport_StdoutJSON(t *testing.T) {
	const content = "# Note\n\nnaïve café\n"
	conn := fakeExportDrive(t, markdownMeta, content)
	var buf bytes.Buffer
	out := &outputWriter{json: true, writer: &buf}
	if err := runDriveExport(context.Background(), conn, "F1", "", "", "-", out); err != nil {
		t.Fatalf("runDriveExport(-, json) error = %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw %s)", err, buf.String())
	}
	if got["content"] != content {
		t.Errorf("content = %q, want %q", got["content"], content)
	}
	if got["headRevisionId"] != "REV2" || got["md5Checksum"] != "abc123" || got["id"] != "F1" ||
		got["modifiedTime"] != "2026-01-02T03:04:05Z" {
		t.Errorf("metadata = %+v", got)
	}
	if _, ok := got["file"]; ok {
		t.Errorf("stdout JSON should not report a file: %+v", got)
	}
}

func TestRunDriveExport_StdoutRejects(t *testing.T) {
	folder := fakeExportDrive(t, `{"id":"F1","name":"Notes","mimeType":"application/vnd.google-apps.folder"}`, "")
	out := &outputWriter{json: false, writer: &bytes.Buffer{}}
	if err := runDriveExport(context.Background(), folder, "F1", "", "", "-", out); err == nil {
		t.Errorf("folder with --output -: expected error")
	}

	binary := fakeExportDrive(t, `{"id":"F1","name":"x.bin","mimeType":"application/octet-stream"}`, "\xff\xfe\x00")
	out = &outputWriter{json: true, writer: &bytes.Buffer{}}
	if err := runDriveExport(context.Background(), binary, "F1", "", "", "-", out); err == nil {
		t.Errorf("non-UTF-8 content with --output - --json: expected error")
	}

	// Raw (non-JSON) stdout passes binary through untouched.
	var buf bytes.Buffer
	out = &outputWriter{json: false, writer: &buf}
	if err := runDriveExport(context.Background(), binary, "F1", "", "", "-", out); err != nil {
		t.Fatalf("raw binary to stdout: %v", err)
	}
	if buf.String() != "\xff\xfe\x00" {
		t.Errorf("raw stdout = %q", buf.String())
	}
}
