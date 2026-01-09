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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/errors"
	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/reporting"
)

const labelsComment = `  // Note: labels management is optional. If you prefer to use the
  // GMail interface to add and remove labels, you can safely remove
  // this section of the config.
`

var labelsLine = "  labels: ["

// ReverseImport converts a list of filters into config rules.
func ReverseImport(fs Filters, ls Labels) (Config, error) {
	var rules []Rule
	for i, f := range fs {
		r, err := fromFilter(f)
		if err != nil {
			return Config{}, errors.WithDetails(
				fmt.Errorf("importing filter #%d: %w", i, err),
				fmt.Sprintf("Filter (internal representation): %s", reporting.Prettify(f, false)))
		}
		rules = append(rules, r)
	}
	var labels []Label
	for _, l := range ls {
		labels = append(labels, fromGmailLabel(l))
	}
	return Config{
		Version: Version,
		Author: Author{
			Name:  "YOUR NAME HERE (auto imported)",
			Email: "your-email@gmail.com",
		},
		Labels: labels,
		Rules:  rules,
	}, nil
}

func fromGmailLabel(l GmailLabel) Label {
	var color *LabelColor
	if l.Color != nil {
		color = &LabelColor{
			Background: l.Color.Background,
			Text:       l.Color.Text,
		}
	}
	return Label{Name: l.Name, Color: color}
}

func fromFilter(f Filter) (Rule, error) {
	n, err := fromFilterCriteria(f.Criteria)
	if err != nil {
		return Rule{}, err
	}
	a, err := fromFilterActions(f.Action)
	return Rule{Filter: n, Actions: a}, err
}

func fromFilterCriteria(c FilterCriteria) (FilterNode, error) {
	nodes := []FilterNode{}
	if c.From != "" {
		n := FilterNode{From: c.From, IsEscaped: needsEscape(c.From)}
		nodes = append(nodes, n)
	}
	if c.To != "" {
		n := FilterNode{To: c.To, IsEscaped: needsEscape(c.To)}
		nodes = append(nodes, n)
	}
	if c.Subject != "" {
		n := FilterNode{Subject: c.Subject, IsEscaped: needsEscape(c.Subject)}
		nodes = append(nodes, n)
	}
	if c.Query != "" {
		n := FilterNode{Query: c.Query}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		return FilterNode{}, errors.New("empty criteria")
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	return FilterNode{And: nodes}, nil
}

func needsEscape(s string) bool {
	return strings.ContainsAny(s, ` '"`)
}

func fromFilterActions(c FilterActions) (Actions, error) {
	res := Actions{
		Category: c.Category,
		Archive:  c.Archive,
		Delete:   c.Delete,
		MarkRead: c.MarkRead,
		Star:     c.Star,
		Forward:  c.Forward,
	}
	if c.AddLabel != "" {
		res.Labels = []string{c.AddLabel}
	}
	var err error
	res.MarkImportant, err = handleTribool(c.MarkImportant, c.MarkNotImportant)
	if err != nil {
		return res, fmt.Errorf("in 'mark important': %w", err)
	}
	if c.MarkNotSpam {
		res.MarkSpam = boolPtr(false)
	}
	return res, nil
}

func handleTribool(isTrue, isFalse bool) (*bool, error) {
	if isTrue && isFalse {
		return nil, errors.New("cannot be both true and false")
	}
	if isTrue || isFalse {
		return &isTrue, nil
	}
	return nil, nil
}

func boolPtr(v bool) *bool {
	return &v
}

// MarshalJsonnet writes a config to jsonnet format.
func MarshalJsonnet(v interface{}, w io.Writer, header string) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	reader := bufio.NewReader(bytes.NewReader(b))
	writer := bufio.NewWriter(w)
	keyRe := regexp.MustCompile(`^ *"([a-zA-Z01]+)":`)
	var line []byte

	_, err = writer.WriteString(header)
	if err != nil {
		return err
	}

	line, _, err = reader.ReadLine()
	for err == nil {
		line = replaceGroupsRe(keyRe, line)
		if string(line) == labelsLine {
			_, err = writer.WriteString(labelsComment)
			if err != nil {
				break
			}
		}
		_, err = writer.Write(line)
		if err != nil {
			break
		}
		_, err = writer.WriteRune('\n')
		if err != nil {
			break
		}
		line, _, err = reader.ReadLine()
	}

	if err == io.EOF {
		return writer.Flush()
	}
	return err
}

func replaceGroupsRe(re *regexp.Regexp, in []byte) []byte {
	m := re.FindSubmatchIndex(in)
	if len(m) == 0 {
		return in
	}
	keyb, keye := m[2], m[3]
	var res []byte
	res = append(res, in[:keyb-1]...)
	res = append(res, in[keyb:keye]...)
	res = append(res, in[keye+1:]...)
	return res
}
