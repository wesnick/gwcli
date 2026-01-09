// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Ported from github.com/mbrt/gmailctl

package gmailctl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLabelsInvalid(t *testing.T) {
	cases := []struct {
		name   string
		labels Labels
	}{
		{
			"unnamed",
			Labels{{Name: ""}},
		},
		{
			"starts with slash",
			Labels{{Name: "/foobar"}},
		},
		{
			"ends with slash",
			Labels{{Name: "foobar/"}},
		},
		{
			"duplicates",
			Labels{
				{Name: "abc"},
				{Name: "abc"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.labels.Validate()
			assert.NotNil(t, err)
		})
	}
}

func TestLabelsValid(t *testing.T) {
	cases := []struct {
		name   string
		labels Labels
	}{
		{"empty", nil},
		{
			"single",
			Labels{{Name: "foobar"}},
		},
		{
			"sub-labels",
			Labels{
				{Name: "abc/def"},
				{Name: "abc"},
				{Name: "abc/def/ghi"},
				{Name: "another"},
			},
		},
		{
			"missing prefix",
			Labels{
				{Name: "abc/def"},
				{Name: "ab"},
			},
		},
		{
			"missing prefix 2",
			Labels{
				{Name: "abc"},
				{Name: "abc/def/ghi"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.labels.Validate()
			assert.Nil(t, err)
		})
	}
}

func TestLabelsDiffValidate(t *testing.T) {
	d := LabelsDiff{
		Removed: Labels{{Name: "foo"}},
	}
	fs := Filters{
		{
			Criteria: FilterCriteria{To: "foobar"},
			Action:   FilterActions{AddLabel: "foo"},
		},
	}
	err := ValidateLabelsDiff(d, fs)
	assert.NotNil(t, err)
}

func TestLabelsDiffValidateOK(t *testing.T) {
	d := LabelsDiff{
		Removed: Labels{{Name: "bar"}},
	}
	fs := Filters{
		{
			Criteria: FilterCriteria{To: "foobar"},
			Action:   FilterActions{AddLabel: "foo"},
		},
	}
	err := ValidateLabelsDiff(d, fs)
	assert.Nil(t, err)
}

func TestLabelsEquivalent(t *testing.T) {
	tests := []struct {
		name     string
		upstream GmailLabel
		local    GmailLabel
		want     bool
	}{
		{
			name:     "same name, no colors",
			upstream: GmailLabel{Name: "test"},
			local:    GmailLabel{Name: "test"},
			want:     true,
		},
		{
			name:     "different names",
			upstream: GmailLabel{Name: "test1"},
			local:    GmailLabel{Name: "test2"},
			want:     false,
		},
		{
			name:     "local has no color",
			upstream: GmailLabel{Name: "test", Color: &GmailLabelColor{Background: "#000", Text: "#fff"}},
			local:    GmailLabel{Name: "test"},
			want:     true,
		},
		{
			name:     "upstream has no color but local does",
			upstream: GmailLabel{Name: "test"},
			local:    GmailLabel{Name: "test", Color: &GmailLabelColor{Background: "#000", Text: "#fff"}},
			want:     false,
		},
		{
			name:     "same colors",
			upstream: GmailLabel{Name: "test", Color: &GmailLabelColor{Background: "#000", Text: "#fff"}},
			local:    GmailLabel{Name: "test", Color: &GmailLabelColor{Background: "#000", Text: "#fff"}},
			want:     true,
		},
		{
			name:     "different colors",
			upstream: GmailLabel{Name: "test", Color: &GmailLabelColor{Background: "#000", Text: "#fff"}},
			local:    GmailLabel{Name: "test", Color: &GmailLabelColor{Background: "#fff", Text: "#000"}},
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LabelsEquivalent(tc.upstream, tc.local)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGmailLabelString(t *testing.T) {
	tests := []struct {
		name  string
		label GmailLabel
		want  string
	}{
		{
			name:  "name only",
			label: GmailLabel{Name: "test"},
			want:  "test",
		},
		{
			name:  "with ID",
			label: GmailLabel{ID: "abc123", Name: "test"},
			want:  "test [abc123]",
		},
		{
			name:  "with color",
			label: GmailLabel{Name: "test", Color: &GmailLabelColor{Background: "#000", Text: "#fff"}},
			want:  "test; color: #000, #fff",
		},
		{
			name:  "with ID and color",
			label: GmailLabel{ID: "abc123", Name: "test", Color: &GmailLabelColor{Background: "#000", Text: "#fff"}},
			want:  "test [abc123]; color: #000, #fff",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.label.String()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLabelMapNameToID(t *testing.T) {
	labels := Labels{
		{ID: "id1", Name: "label1"},
		{ID: "id2", Name: "label2"},
		{ID: "id3", Name: "nested/label"},
	}
	lm := NewLabelMap(labels)

	// Test finding by name
	id, ok := lm.NameToID("label1")
	assert.True(t, ok)
	assert.Equal(t, "id1", id)

	id, ok = lm.NameToID("label2")
	assert.True(t, ok)
	assert.Equal(t, "id2", id)

	id, ok = lm.NameToID("nested/label")
	assert.True(t, ok)
	assert.Equal(t, "id3", id)

	// Test not found
	_, ok = lm.NameToID("nonexistent")
	assert.False(t, ok)
}

func TestLabelMapIDToName(t *testing.T) {
	labels := Labels{
		{ID: "id1", Name: "label1"},
		{ID: "id2", Name: "label2"},
	}
	lm := NewLabelMap(labels)

	// Test finding by ID
	name, ok := lm.IDToName("id1")
	assert.True(t, ok)
	assert.Equal(t, "label1", name)

	name, ok = lm.IDToName("id2")
	assert.True(t, ok)
	assert.Equal(t, "label2", name)

	// Test not found
	_, ok = lm.IDToName("nonexistent")
	assert.False(t, ok)
}
