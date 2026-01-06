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

// Vendored from github.com/mbrt/gmailctl

package gmailctl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/errors"
	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/reporting"
)

// Labels is a list of labels.
type Labels []GmailLabel

func (ls Labels) String() string {
	var ss []string
	for _, l := range ls {
		ss = append(ss, l.String())
	}
	return strings.Join(ss, "\n")
}

// Validate checks the given labels for possible issues.
func (ls Labels) Validate() error {
	lmap := map[string]struct{}{}
	for _, l := range ls {
		n := l.Name
		if n == "" {
			return errors.New("invalid label without a name")
		}
		if strings.HasPrefix(n, "/") {
			return fmt.Errorf("label %q shouldn't start with /", n)
		}
		if strings.HasSuffix(n, "/") {
			return fmt.Errorf("label %q shouldn't end with /", n)
		}
		if _, ok := lmap[n]; ok {
			return fmt.Errorf("label %q provided multiple times", n)
		}
		lmap[n] = struct{}{}
	}
	return nil
}

// GmailLabel contains information about a Gmail label.
type GmailLabel struct {
	ID    string
	Name  string
	Color *GmailLabelColor
}

func (l GmailLabel) String() string {
	var ss []string
	if l.ID != "" {
		ss = append(ss, fmt.Sprintf("%s [%s]", l.Name, l.ID))
	} else {
		ss = append(ss, l.Name)
	}
	if l.Color != nil {
		ss = append(ss, fmt.Sprintf("color: %s, %s", l.Color.Background, l.Color.Text))
	}
	return strings.Join(ss, "; ")
}

// GmailLabelColor is the color of a label.
type GmailLabelColor struct {
	Background string
	Text       string
}

// LabelsEquivalent returns true if two labels can be considered equal.
func LabelsEquivalent(upstream, local GmailLabel) bool {
	if upstream.Name != local.Name {
		return false
	}
	upsHasColor := upstream.Color != nil
	locHasColor := local.Color != nil
	if !locHasColor {
		return true
	}
	if !upsHasColor {
		return false
	}
	return *upstream.Color == *local.Color
}

// LabelsFromConfig creates labels from the config format.
func LabelsFromConfig(ls []Label) Labels {
	var res Labels
	for _, l := range ls {
		var color *GmailLabelColor
		if l.Color != nil {
			color = &GmailLabelColor{
				Background: l.Color.Background,
				Text:       l.Color.Text,
			}
		}
		res = append(res, GmailLabel{Name: l.Name, Color: color})
	}
	return res
}

// LabelsDiff contains the diff of two lists of labels.
type LabelsDiff struct {
	Modified []ModifiedLabel
	Added    Labels
	Removed  Labels
	Colorize bool
}

// Empty returns true if the diff is empty.
func (d LabelsDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Modified) == 0
}

func (d LabelsDiff) String() string {
	var old, curr []string
	cleanup := func(l GmailLabel) GmailLabel {
		return GmailLabel{Name: l.Name, Color: l.Color}
	}
	for _, ml := range d.Modified {
		old = append(old, cleanup(ml.Old).String()+"\n")
		curr = append(curr, ml.New.String()+"\n")
	}
	for _, l := range d.Removed {
		old = append(old, cleanup(l).String()+"\n")
	}
	for _, l := range d.Added {
		curr = append(curr, l.String()+"\n")
	}
	s, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        old,
		B:        curr,
		FromFile: "Current",
		ToFile:   "TO BE APPLIED",
		Context:  3,
	})
	if err != nil {
		return fmt.Sprintf("Removed:\n%s\nAdded:\n%sModified:%s",
			strings.Join(old, "\n"),
			strings.Join(curr, "\n"),
			fmt.Sprint(d.Modified),
		)
	}
	if d.Colorize {
		s = reporting.ColorizeDiff(s)
	}
	return s
}

// ModifiedLabel is a label in two versions.
type ModifiedLabel struct {
	Old GmailLabel
	New GmailLabel
}

// DiffLabels computes the diff between two lists of labels.
func DiffLabels(upstream, local Labels, colorize bool) (LabelsDiff, error) {
	sort.Sort(labelsByName(upstream))
	sort.Sort(labelsByName(local))
	res := LabelsDiff{Colorize: colorize}
	i, j := 0, 0
	for i < len(upstream) && j < len(local) {
		ups := upstream[i]
		loc := local[j]
		cmp := strings.Compare(ups.Name, loc.Name)
		switch {
		case cmp < 0:
			res.Removed = append(res.Removed, ups)
			i++
		case cmp > 0:
			res.Added = append(res.Added, loc)
			j++
		default:
			if !LabelsEquivalent(ups, loc) {
				res.Modified = append(res.Modified, ModifiedLabel{Old: ups, New: loc})
			}
			i++
			j++
		}
	}
	for ; i < len(upstream); i++ {
		res.Removed = append(res.Removed, upstream[i])
	}
	for ; j < len(local); j++ {
		res.Added = append(res.Added, local[j])
	}
	return res, nil
}

// ValidateLabelsDiff makes sure that a diff is valid and safe to apply.
func ValidateLabelsDiff(d LabelsDiff, filters Filters) error {
	for _, l := range d.Removed {
		if filters.HasLabel(l.Name) {
			return fmt.Errorf("cannot remove label %q, used in filter", l.Name)
		}
	}
	return nil
}

type labelsByName Labels

func (b labelsByName) Len() int           { return len(b) }
func (b labelsByName) Less(i, j int) bool { return strings.Compare(b[i].Name, b[j].Name) == -1 }
func (b labelsByName) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
