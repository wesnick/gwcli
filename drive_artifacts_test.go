package main

import (
	"reflect"
	"testing"
)

func TestParseDriveURL(t *testing.T) {
	cases := []struct {
		url      string
		wantID   string
		wantType string
		wantOK   bool
	}{
		{"https://docs.google.com/document/d/ABC_123-xyz/edit?usp=meet_tnfm_email", "ABC_123-xyz", "document", true},
		{"https://docs.google.com/spreadsheets/d/SHEET1/edit#gid=0", "SHEET1", "spreadsheet", true},
		{"https://docs.google.com/presentation/d/SLIDES1/edit", "SLIDES1", "presentation", true},
		{"https://drive.google.com/file/d/FILE1/view", "FILE1", "drive-file", true},
		{"https://drive.google.com/open?id=OPEN1", "OPEN1", "drive-file", true},
		{"https://drive.google.com/drive/folders/FOLDER1", "FOLDER1", "folder", true},
		{"https://example.com/not/drive", "", "", false},
		{"https://google.com/search?q=docs", "", "", false},
	}
	for _, c := range cases {
		id, typ, ok := parseDriveURL(c.url)
		if id != c.wantID || typ != c.wantType || ok != c.wantOK {
			t.Errorf("parseDriveURL(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.url, id, typ, ok, c.wantID, c.wantType, c.wantOK)
		}
	}
}

func TestDetectDriveArtifacts_MeetNotes(t *testing.T) {
	const id = "1sBJ5Gr6iM4sBL9Afiwu-JXqCkrgNONRf-iPzISZdgYE"
	// Mirrors the real Gemini "Notes" email: the doc appears three times —
	// as a link-button, deep-link, and artifact chip. The chip label wins
	// and the three links collapse to one deduped artifact.
	html := `<div>
		<a href="https://docs.google.com/document/d/` + id + `/edit?usp=meet_tnfm_email" class="link-button"><span>Open meeting notes</span></a>
		<a href="https://docs.google.com/document/d/` + id + `/edit?usp=meet_tnfm_email&tab=t.kl4popnauei#heading=h.axfjrm411zec">Decisions section</a>
		<a href="https://docs.google.com/document/d/` + id + `/edit?usp=meet_tnfm_email" class="artifact-chip"><span>Notes by Gemini</span></a>
	</div>`
	// Chips/buttons are surfaced regardless of the Meet header.
	got := detectDriveArtifacts(html, "")
	want := []driveArtifact{{
		Index: 0,
		ID:    id,
		Type:  "document",
		Title: "Notes by Gemini",
		URL:   "https://docs.google.com/document/d/" + id + "/edit",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("detectDriveArtifacts() = %+v, want %+v", got, want)
	}
}

func TestDetectDriveArtifacts_MeetHeaderGatesPlainLink(t *testing.T) {
	const id = "PLAINDOCID123"
	// A bare doc link with no chip/button class: only surfaced when its ID
	// is present in the (decoded) Meet metadata header.
	html := `<p>see <a href="https://docs.google.com/document/d/` + id + `/edit">the doc</a></p>`

	if got := detectDriveArtifacts(html, ""); len(got) != 0 {
		t.Errorf("expected no artifacts without Meet header/chip, got %+v", got)
	}

	// base64("prefix PLAINDOCID123 suffix") — the header is base64 and the
	// bare file ID appears verbatim in the decoded payload.
	encoded := "cHJlZml4IFBMQUlORE9DSUQxMjMgc3VmZml4"
	got := detectDriveArtifacts(html, encoded)
	if len(got) != 1 || got[0].ID != id || got[0].Title != "the doc" {
		t.Errorf("expected the doc surfaced via Meet header, got %+v", got)
	}
}

func TestDetectDriveArtifacts_NoFalsePositives(t *testing.T) {
	html := `<p>Visit <a href="https://example.com">our site</a> and
		<a href="https://google.com/search?q=foo">search</a>.</p>`
	if got := detectDriveArtifacts(html, ""); len(got) != 0 {
		t.Errorf("expected no artifacts, got %+v", got)
	}
	if got := detectDriveArtifacts("", ""); got != nil {
		t.Errorf("expected nil for empty body, got %+v", got)
	}
}

func TestCanonicalDriveURL(t *testing.T) {
	cases := map[string]string{
		"document":     "https://docs.google.com/document/d/X/edit",
		"spreadsheet":  "https://docs.google.com/spreadsheets/d/X/edit",
		"presentation": "https://docs.google.com/presentation/d/X/edit",
		"folder":       "https://drive.google.com/drive/folders/X",
		"drive-file":   "https://drive.google.com/file/d/X/view",
	}
	for typ, want := range cases {
		if got := canonicalDriveURL("X", typ); got != want {
			t.Errorf("canonicalDriveURL(X, %q) = %q, want %q", typ, got, want)
		}
	}
}
