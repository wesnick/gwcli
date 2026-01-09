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

func TestSimplify(t *testing.T) {
	expr := or(
		fn1(FunctionFrom, "a"),
		fn1(FunctionFrom, "b"),
		fn1(FunctionSubject, "c"),
		and(
			fn1(FunctionList, "d"),
			and(
				fn1(FunctionFrom, "e"),
				not(not(
					fn1(FunctionList, "f"),
				)),
			),
		),
	)

	expected := or(
		and(
			fn(FunctionList, OperationAnd, "f", "d"),
			fn(FunctionFrom, OperationAnd, "e"),
		),
		fn(FunctionSubject, OperationOr, "c"),
		fn(FunctionFrom, OperationOr, "a", "b"),
	)
	got, err := SimplifyCriteria(expr)
	assert.Nil(t, err)

	// Maps make the children sorting pseudo-random. We have to sort
	// the trees to be able to find make it deterministic.
	sortTree(expected)
	sortTree(got)
	assert.Equal(t, expected, got)

}

func TestParseCriteria(t *testing.T) {
	tests := []struct {
		name     string
		filter   FilterNode
		expected CriteriaAST
	}{
		{
			name:     "simple from",
			filter:   FilterNode{From: "someone@gmail.com"},
			expected: fn1(FunctionFrom, "someone@gmail.com"),
		},
		{
			name:     "simple to",
			filter:   FilterNode{To: "someone@gmail.com"},
			expected: fn1(FunctionTo, "someone@gmail.com"),
		},
		{
			name:     "simple subject",
			filter:   FilterNode{Subject: "hello world"},
			expected: fn1(FunctionSubject, "hello world"),
		},
		{
			name: "and operation",
			filter: FilterNode{
				And: []FilterNode{
					{From: "a"},
					{To: "b"},
				},
			},
			expected: and(fn1(FunctionFrom, "a"), fn1(FunctionTo, "b")),
		},
		{
			name: "or operation",
			filter: FilterNode{
				Or: []FilterNode{
					{From: "a"},
					{From: "b"},
				},
			},
			expected: or(fn1(FunctionFrom, "a"), fn1(FunctionFrom, "b")),
		},
		{
			name: "not operation",
			filter: FilterNode{
				Not: &FilterNode{From: "spam@example.com"},
			},
			expected: not(fn1(FunctionFrom, "spam@example.com")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCriteria(tc.filter)
			assert.Nil(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestCheckSyntaxErrors(t *testing.T) {
	tests := []struct {
		name   string
		filter FilterNode
	}{
		{
			name:   "empty filter",
			filter: FilterNode{},
		},
		{
			name: "multiple fields",
			filter: FilterNode{
				From: "a",
				To:   "b",
			},
		},
		{
			name: "isEscaped with invalid field",
			filter: FilterNode{
				Has:       "something",
				IsEscaped: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSyntax(tc.filter)
			assert.NotNil(t, err)
		})
	}
}

func TestOperationTypeString(t *testing.T) {
	assert.Equal(t, "<none>", OperationNone.String())
	assert.Equal(t, "and", OperationAnd.String())
	assert.Equal(t, "or", OperationOr.String())
}

func TestFunctionTypeString(t *testing.T) {
	assert.Equal(t, "<none>", FunctionNone.String())
	assert.Equal(t, "from", FunctionFrom.String())
	assert.Equal(t, "to", FunctionTo.String())
	assert.Equal(t, "cc", FunctionCc.String())
	assert.Equal(t, "bcc", FunctionBcc.String())
	assert.Equal(t, "replyto", FunctionReplyTo.String())
	assert.Equal(t, "subject", FunctionSubject.String())
	assert.Equal(t, "list", FunctionList.String())
	assert.Equal(t, "has", FunctionHas.String())
	assert.Equal(t, "query", FunctionQuery.String())
}

// Helper functions for building AST nodes in tests
func and(children ...CriteriaAST) *Node {
	return &Node{
		Operation: OperationAnd,
		Children:  children,
	}
}

func or(children ...CriteriaAST) *Node {
	return &Node{
		Operation: OperationOr,
		Children:  children,
	}
}

func not(child CriteriaAST) *Node {
	return &Node{
		Operation: OperationNot,
		Children:  []CriteriaAST{child},
	}
}

func fn(ftype FunctionType, op OperationType, args ...string) *Leaf {
	return &Leaf{
		Function: ftype,
		Grouping: op,
		Args:     args,
	}
}

func fn1(ftype FunctionType, arg string) *Leaf {
	return fn(ftype, OperationNone, arg)
}
