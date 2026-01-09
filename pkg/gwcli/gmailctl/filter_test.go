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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func boolptr(a bool) *bool {
	return &a
}

func TestCriteria(t *testing.T) {
	tests := []struct {
		name           string
		criteria       FilterCriteria
		gmailSearch    string
		gmailSearchURL string
	}{
		{
			name:           "from criteria",
			criteria:       FilterCriteria{From: "someone@gmail.com"},
			gmailSearch:    "from:someone@gmail.com",
			gmailSearchURL: "https://mail.google.com/mail/u/0/#search/from%3Asomeone%40gmail.com",
		},
		{
			name: "complicated query",
			criteria: FilterCriteria{
				Query: "{from:noreply@acme.com to:{me@google.com me@acme.com}}",
			},
			gmailSearch:    "{from:noreply@acme.com to:{me@google.com me@acme.com}}",
			gmailSearchURL: "https://mail.google.com/mail/u/0/#search/%7Bfrom%3Anoreply%40acme.com+to%3A%7Bme%40google.com+me%40acme.com%7D%7D",
		},
		{
			name: "all fields",
			criteria: FilterCriteria{
				From:    "someone@gmail.com",
				To:      "me@gmail.com",
				Subject: "Hello world",
				Query:   "unsubscribe",
			},
			gmailSearch:    "from:someone@gmail.com to:me@gmail.com subject:Hello world unsubscribe",
			gmailSearchURL: "https://mail.google.com/mail/u/0/#search/from%3Asomeone%40gmail.com+to%3Ame%40gmail.com+subject%3AHello+world+unsubscribe",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.criteria.ToGmailSearch(), tc.gmailSearch)
			assert.Equal(t, tc.criteria.ToGmailSearchURL(), tc.gmailSearchURL)
		})
	}
}

func TestQuotes(t *testing.T) {
	rules := []ParsedRule{
		{
			Criteria: &Leaf{
				Function: FunctionFrom,
				Grouping: OperationOr,
				Args: []string{
					"a",
					"with spaces",
					"with+plus",
					"with+plus@email.com",
					`"already-quoted"`,
				},
			},
			Actions: Actions{
				Archive: true,
			},
		},
	}
	// The plus sign is quoted, except for when in full email addresses.
	expected := Filters{
		{
			Criteria: FilterCriteria{
				From: `{a "with spaces" "with+plus" with+plus@email.com "already-quoted"}`,
			},
			Action: FilterActions{
				Archive: true,
			},
		},
	}
	got, err := FromRules(rules)
	assert.Nil(t, err)
	assert.Equal(t, expected, got)
}

func TestAndNode(t *testing.T) {
	rules := []ParsedRule{
		{
			Criteria: &Node{
				Operation: OperationAnd,
				Children: []CriteriaAST{
					&Leaf{
						Function: FunctionFrom,
						Args:     []string{"a"},
					},
					&Leaf{
						Function: FunctionTo,
						Grouping: OperationAnd,
						Args:     []string{"a", "b", "c"},
					},
				},
			},
			Actions: Actions{
				Delete:   true,
				Category: CategoryForums,
			},
		},
	}
	expected := Filters{
		{
			Criteria: FilterCriteria{
				From: "a",
				To:   "(a b c)",
			},
			Action: FilterActions{
				Delete:   true,
				Category: CategoryForums,
			},
		},
	}
	got, err := FromRules(rules)
	assert.Nil(t, err)
	assert.Equal(t, expected, got)
}

func TestNotOr(t *testing.T) {
	rules := []ParsedRule{
		{
			Criteria: &Node{
				Operation: OperationNot,
				Children: []CriteriaAST{
					&Node{
						Operation: OperationOr,
						Children: []CriteriaAST{
							&Leaf{
								Function: FunctionTo,
								Grouping: OperationOr,
								Args:     []string{"a", "b"},
							},
							&Leaf{
								Function: FunctionCc,
								Grouping: OperationAnd,
								Args:     []string{"c", "d"},
							},
						},
					},
				},
			},
			Actions: Actions{
				MarkRead: true,
			},
		},
	}
	expected := Filters{
		{
			Criteria: FilterCriteria{
				Query: "-{to:{a b} cc:(c d)}",
			},
			Action: FilterActions{
				MarkRead: true,
			},
		},
	}
	got, err := FromRules(rules)
	assert.Nil(t, err)
	assert.Equal(t, expected, got)
}

func TestQuoting(t *testing.T) {
	rules := []ParsedRule{
		{
			Criteria: &Node{
				Operation: OperationAnd,
				Children: []CriteriaAST{
					&Leaf{
						Function: FunctionHas,
						Grouping: OperationAnd,
						Args:     []string{"foo", "this is quoted"},
					},
					&Leaf{
						Function: FunctionQuery,
						Args:     []string{`from:foo has:spreadsheet`},
					},
				},
			},
			Actions: Actions{
				MarkImportant: boolptr(true),
			},
		},
	}
	expected := Filters{
		{
			Criteria: FilterCriteria{
				Query: `(foo "this is quoted") from:foo has:spreadsheet`,
			},
			Action: FilterActions{
				MarkImportant: true,
			},
		},
	}
	got, err := FromRules(rules)
	assert.Nil(t, err)
	assert.Equal(t, expected, got)
}

func TestSplitLeaf(t *testing.T) {
	rule := ParsedRule{
		Criteria: &Leaf{
			Function: FunctionFrom,
			Grouping: OperationOr,
			Args:     []string{"a", "b", "c"},
		},
		Actions: Actions{Archive: true},
	}
	expected := Filters{
		{
			Criteria: FilterCriteria{From: "{a b}"},
			Action:   FilterActions{Archive: true},
		},
		{
			Criteria: FilterCriteria{From: "c"},
			Action:   FilterActions{Archive: true},
		},
	}
	got, err := FromRule(rule, 2)
	assert.Nil(t, err)
	assert.Equal(t, expected, got)
}

func TestSplitActions(t *testing.T) {
	rules := []ParsedRule{
		{
			Criteria: &Leaf{
				Function: FunctionFrom,
				Args:     []string{"a"},
			},
			Actions: Actions{
				Archive:  true,
				MarkRead: true,
				Labels:   []string{"l1", "l2", "l3"},
			},
		},
	}
	expected := Filters{
		{
			Criteria: FilterCriteria{
				From: "a",
			},
			Action: FilterActions{
				Archive:  true,
				MarkRead: true,
				AddLabel: "l1",
			},
		},
		{
			Criteria: FilterCriteria{
				From: "a",
			},
			Action: FilterActions{
				AddLabel: "l2",
			},
		},
		{
			Criteria: FilterCriteria{
				From: "a",
			},
			Action: FilterActions{
				AddLabel: "l3",
			},
		},
	}
	got, err := FromRules(rules)
	assert.Nil(t, err)
	assert.Equal(t, expected, got)
}

func TestActions(t *testing.T) {
	rules := []ParsedRule{
		{
			Criteria: &Leaf{
				Function: FunctionFrom,
				Args:     []string{"a"},
			},
			Actions: Actions{
				Archive:       true,
				Delete:        true,
				MarkRead:      true,
				Star:          true,
				MarkSpam:      boolptr(false),
				MarkImportant: boolptr(true),
				Category:      CategoryForums,
				Forward:       "foo@bar.com",
			},
		},
	}
	expected := Filters{
		{
			Criteria: FilterCriteria{
				From: "a",
			},
			Action: FilterActions{
				Archive:       true,
				Delete:        true,
				MarkRead:      true,
				Star:          true,
				MarkNotSpam:   true,
				MarkImportant: true,
				Category:      CategoryForums,
				Forward:       "foo@bar.com",
			},
		},
	}
	got, err := FromRules(rules)
	assert.Nil(t, err)
	assert.Equal(t, expected, got)
}

const contextLines = 5

func TestNoDiff(t *testing.T) {
	prev := Filters{
		{
			ID: "abcdefg",
			Criteria: FilterCriteria{
				From: "someone@gmail.com",
			},
			Action: FilterActions{
				MarkRead: true,
			},
		},
	}
	curr := Filters{
		{
			Criteria: FilterCriteria{
				From: "someone@gmail.com",
			},
			Action: FilterActions{
				MarkRead: true,
			},
		},
	}

	fd, err := DiffFilters(prev, curr, false, contextLines, false /* colorize */)
	assert.Nil(t, err)
	// No difference even if the ID is present in only one of them.
	assert.True(t, fd.Empty())
}

func TestDiffOutput(t *testing.T) {
	prev := Filters{
		{
			ID: "abcdefg",
			Criteria: FilterCriteria{
				From:  "someone@gmail.com",
				Query: "(a b) subject:(foo bar)",
			},
			Action: FilterActions{
				MarkRead: true,
				Category: CategoryPersonal,
			},
		},
	}
	curr := Filters{
		{
			Criteria: FilterCriteria{
				From:  "{someone@gmail.com else@gmail.com}",
				Query: "(a c) subject:(foo baz)",
			},
			Action: FilterActions{
				MarkRead: true,
				Category: CategoryPersonal,
			},
		},
	}

	fd, err := DiffFilters(prev, curr, false, contextLines, false /* colorize */)
	assert.Nil(t, err)

	// Note: The output may have slight whitespace variations
	got := strings.TrimSpace(fd.String())

	// Verify key elements are present
	assert.Contains(t, got, "--- Current")
	assert.Contains(t, got, "+++ TO BE APPLIED")
	assert.Contains(t, got, "-    from: someone@gmail.com")
	assert.Contains(t, got, "+    from: {someone@gmail.com else@gmail.com}")
	assert.Contains(t, got, "-        b")
	assert.Contains(t, got, "+        c")
	assert.Contains(t, got, "-        bar")
	assert.Contains(t, got, "+        baz")
	assert.Contains(t, got, "mark as read")
	assert.Contains(t, got, "categorize as: personal")
}

func someFilters() Filters {
	return Filters{
		{
			ID: "abcdefg",
			Criteria: FilterCriteria{
				From: "someone@gmail.com",
			},
			Action: FilterActions{
				AddLabel: "label1",
			},
		},
		{
			ID: "qwerty",
			Criteria: FilterCriteria{
				To: "me@gmail.com",
			},
			Action: FilterActions{
				MarkRead: true,
				AddLabel: "label2",
			},
		},
		{
			ID: "zxcvb",
			Criteria: FilterCriteria{
				Query: "-{foobar baz}",
			},
			Action: FilterActions{
				MarkImportant: true,
			},
		},
	}
}

func TestDiffAddRemove(t *testing.T) {
	prev := someFilters()
	curr := Filters{
		{
			Criteria: FilterCriteria{
				From: "{someone@gmail.com else@gmail.com}",
			},
			Action: FilterActions{
				MarkRead: true,
				Category: CategoryPersonal,
			},
		},
		{
			Criteria: FilterCriteria{
				Query: "-{foobar baz}",
			},
			Action: FilterActions{
				MarkImportant: true,
			},
		},
		{
			Criteria: FilterCriteria{
				From: "someone@gmail.com",
			},
			Action: FilterActions{
				AddLabel: "label1",
			},
		},
	}

	fd, err := DiffFilters(prev, curr, false, contextLines, false /* colorize */)
	expected := FiltersDiff{
		Added:        Filters{curr[0]},
		Removed:      Filters{prev[1]},
		ContextLines: contextLines,
	}
	assert.Nil(t, err)
	assert.Equal(t, expected, fd)
}

func TestDiffReorder(t *testing.T) {
	prev := someFilters()
	curr := Filters{
		{
			Criteria: FilterCriteria{
				To: "me@gmail.com",
			},
			Action: FilterActions{
				MarkRead: true,
				AddLabel: "label2",
			},
		},
		{
			Criteria: FilterCriteria{
				Query: "-{foobar baz}",
			},
			Action: FilterActions{
				MarkImportant: true,
			},
		},
		{
			Criteria: FilterCriteria{
				From: "someone@gmail.com",
			},
			Action: FilterActions{
				AddLabel: "label1",
			},
		},
	}

	fd, err := DiffFilters(prev, curr, false, contextLines, false /* colorize */)
	assert.Nil(t, err)
	assert.Len(t, fd.Added, 0)
	assert.Len(t, fd.Removed, 0)
}

func TestDuplicate(t *testing.T) {
	prev := Filters{}
	curr := Filters{
		{
			Criteria: FilterCriteria{
				From: "someone@gmail.com",
			},
			Action: FilterActions{
				MarkRead: true,
			},
		},
		{
			Criteria: FilterCriteria{
				From: "someone@gmail.com",
			},
			Action: FilterActions{
				MarkRead: true,
			},
		},
	}

	fd, err := DiffFilters(prev, curr, false, contextLines, false /* colorize */)
	assert.Nil(t, err)
	// Only one of the two identical filters is present
	assert.Equal(t, curr[1:], fd.Added)
}

func TestFilterActionsEmpty(t *testing.T) {
	assert.True(t, FilterActions{}.Empty())
	assert.False(t, FilterActions{MarkRead: true}.Empty())
	assert.False(t, FilterActions{AddLabel: "test"}.Empty())
}

func TestFilterCriteriaEmpty(t *testing.T) {
	assert.True(t, FilterCriteria{}.Empty())
	assert.False(t, FilterCriteria{From: "test"}.Empty())
	assert.False(t, FilterCriteria{Query: "test"}.Empty())
}

func TestFiltersHasLabel(t *testing.T) {
	fs := Filters{
		{
			Criteria: FilterCriteria{From: "a"},
			Action:   FilterActions{AddLabel: "label1"},
		},
		{
			Criteria: FilterCriteria{From: "b"},
			Action:   FilterActions{AddLabel: "label2"},
		},
	}
	assert.True(t, fs.HasLabel("label1"))
	assert.True(t, fs.HasLabel("label2"))
	assert.False(t, fs.HasLabel("label3"))
}
