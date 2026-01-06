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

	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/errors"
	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/reporting"
)

const maxSimplifyPasses = 4

// Logical operations.
const (
	OperationNone OperationType = iota
	OperationAnd
	OperationOr
	OperationNot
)

// OperationType is the type of logical operator.
type OperationType int

func (t OperationType) String() string {
	switch t {
	case OperationNone:
		return "<none>"
	case OperationAnd:
		return "and"
	case OperationOr:
		return "or"
	default:
		return "<unknown>"
	}
}

// Functions.
const (
	FunctionNone FunctionType = iota
	FunctionFrom
	FunctionTo
	FunctionCc
	FunctionBcc
	FunctionReplyTo
	FunctionSubject
	FunctionList
	FunctionHas
	FunctionQuery
)

// FunctionType is the type of a function.
type FunctionType int

func (f FunctionType) String() string {
	switch f {
	case FunctionNone:
		return "<none>"
	case FunctionFrom:
		return "from"
	case FunctionTo:
		return "to"
	case FunctionCc:
		return "cc"
	case FunctionBcc:
		return "bcc"
	case FunctionReplyTo:
		return "replyto"
	case FunctionSubject:
		return "subject"
	case FunctionList:
		return "list"
	case FunctionHas:
		return "has"
	case FunctionQuery:
		return "query"
	default:
		return "<unknown>"
	}
}

// CriteriaAST is the abstract syntax tree of a filter criteria.
type CriteriaAST interface {
	RootOperation() OperationType
	RootFunction() FunctionType
	IsLeaf() bool
	AcceptVisitor(v Visitor)
	Clone() CriteriaAST
}

// Node is an AST node with children nodes.
type Node struct {
	Operation OperationType
	Children  []CriteriaAST
}

func (n *Node) RootOperation() OperationType { return n.Operation }
func (n *Node) RootFunction() FunctionType   { return FunctionNone }
func (n *Node) IsLeaf() bool                 { return false }
func (n *Node) AcceptVisitor(v Visitor)      { v.VisitNode(n) }
func (n *Node) Clone() CriteriaAST {
	var children []CriteriaAST
	for _, c := range n.Children {
		children = append(children, c.Clone())
	}
	return &Node{Operation: n.Operation, Children: children}
}

// Leaf is an AST node with no children.
type Leaf struct {
	Function FunctionType
	Grouping OperationType
	Args     []string
	IsRaw    bool
}

func (n *Leaf) RootOperation() OperationType { return n.Grouping }
func (n *Leaf) RootFunction() FunctionType   { return n.Function }
func (n *Leaf) IsLeaf() bool                 { return true }
func (n *Leaf) AcceptVisitor(v Visitor)      { v.VisitLeaf(n) }
func (n *Leaf) Clone() CriteriaAST {
	return &Leaf{Function: n.Function, Grouping: n.Grouping, Args: n.Args, IsRaw: n.IsRaw}
}

// Visitor implements the visitor pattern for CriteriaAST.
type Visitor interface {
	VisitNode(n *Node)
	VisitLeaf(n *Leaf)
}

// SimplifyCriteria applies multiple simplifications to a criteria.
func SimplifyCriteria(tree CriteriaAST) (CriteriaAST, error) {
	res, err := simplify(tree)
	sortTree(res)
	return res, err
}

func simplify(tree CriteriaAST) (CriteriaAST, error) {
	changes := 1
	for i := 0; changes > 0 && i < maxSimplifyPasses; i++ {
		changes = logicalGrouping(tree)
		changes += functionsGrouping(tree)
		newTree, c := removeRedundancy(tree)
		changes += c
		tree = newTree
	}
	return tree, nil
}

func logicalGrouping(tree CriteriaAST) int {
	root, ok := tree.(*Node)
	if !ok {
		return 0
	}
	count := 0
	for _, child := range root.Children {
		count += logicalGrouping(child)
	}
	if root.Operation == OperationNot {
		return count
	}
	newChildren := []CriteriaAST{}
	for _, child := range root.Children {
		childNode, ok := child.(*Node)
		if !ok || childNode.Operation != root.Operation {
			newChildren = append(newChildren, child)
			continue
		}
		newChildren = append(newChildren, childNode.Children...)
		count++
	}
	root.Children = newChildren
	return count
}

func functionsGrouping(tree CriteriaAST) int {
	root, ok := tree.(*Node)
	if !ok {
		return 0
	}
	count := 0
	for _, child := range root.Children {
		count += functionsGrouping(child)
	}
	if len(root.Children) <= 1 {
		return count
	}
	newChildren := []CriteriaAST{}
	functions := map[FunctionType][]string{}
	rawFunctions := map[FunctionType]bool{}
	for _, child := range root.Children {
		leaf, ok := child.(*Leaf)
		if !ok || (len(leaf.Args) > 1 && leaf.Grouping != root.Operation) {
			newChildren = append(newChildren, child)
			continue
		}
		functions[leaf.Function] = append(functions[leaf.Function], leaf.Args...)
		if leaf.IsRaw {
			rawFunctions[leaf.Function] = true
		}
	}
	for ft, args := range functions {
		_, raw := rawFunctions[ft]
		newChildren = append(newChildren, &Leaf{
			Function: ft, Grouping: root.Operation, Args: args, IsRaw: raw,
		})
		count++
	}
	root.Children = newChildren
	return count
}

func removeRedundancy(tree CriteriaAST) (CriteriaAST, int) {
	root, ok := tree.(*Node)
	if !ok {
		return tree, 0
	}
	count := 0
	newChildren := []CriteriaAST{}
	for _, child := range root.Children {
		newChild, c := removeRedundancy(child)
		count += c
		newChildren = append(newChildren, newChild)
	}
	root.Children = newChildren
	if root.Operation == OperationNot {
		newRoot, c := simplifyNot(root)
		return newRoot, count + c
	}
	if len(root.Children) != 1 {
		return root, count
	}
	return root.Children[0], count + 1
}

func simplifyNot(root *Node) (CriteriaAST, int) {
	if len(root.Children) != 1 {
		return root, 0
	}
	child, ok := root.Children[0].(*Node)
	if !ok || child.Operation != OperationNot {
		return root, 0
	}
	if len(child.Children) != 1 {
		return root, 0
	}
	return child.Children[0], 1
}

func sortTreeNodes(nodes []CriteriaAST) {
	for _, child := range nodes {
		sortTree(child)
	}
	sort.Slice(nodes, func(i, j int) bool {
		ni, nj := nodes[i], nodes[j]
		if ni.IsLeaf() != nj.IsLeaf() {
			return ni.IsLeaf()
		}
		if ni.RootOperation() != nj.RootOperation() {
			return ni.RootOperation() < nj.RootOperation()
		}
		return ni.RootFunction() < nj.RootFunction()
	})
}

func sortTree(tree CriteriaAST) {
	if root, ok := tree.(*Node); ok {
		sortTreeNodes(root.Children)
	}
}

// ParsedRule is an intermediate representation of a Gmail filter.
type ParsedRule struct {
	Criteria CriteriaAST
	Actions  Actions
}

// Parse parses config file rules into their intermediate representation.
func Parse(config Config) ([]ParsedRule, error) {
	res := []ParsedRule{}
	for i, rule := range config.Rules {
		r, err := parseRule(rule)
		if err != nil {
			return nil, errors.WithDetails(
				fmt.Errorf("rule #%d: %w", i, err),
				fmt.Sprintf("Rule: %s", reporting.Prettify(rule, false)),
			)
		}
		res = append(res, r)
	}
	return res, nil
}

func parseRule(rule Rule) (ParsedRule, error) {
	res := ParsedRule{}
	crit, err := parseCriteria(rule.Filter)
	if err != nil {
		return res, fmt.Errorf("parsing criteria: %w", err)
	}
	scrit, err := SimplifyCriteria(crit)
	if err != nil {
		return res, fmt.Errorf("simplifying criteria: %w", err)
	}
	if rule.Actions.Empty() {
		return res, errors.New("empty action")
	}
	return ParsedRule{Criteria: scrit, Actions: rule.Actions}, nil
}

func parseCriteria(f FilterNode) (CriteriaAST, error) {
	if err := checkSyntax(f); err != nil {
		return nil, err
	}
	if op, children := parseOperation(f); op != OperationNone {
		var astchildren []CriteriaAST
		for _, c := range children {
			astc, err := parseCriteria(c)
			if err != nil {
				return nil, err
			}
			astchildren = append(astchildren, astc)
		}
		return &Node{Operation: op, Children: astchildren}, nil
	}
	if fn, arg := parseFunction(f); fn != FunctionNone {
		return &Leaf{Function: fn, Grouping: OperationNone, Args: []string{arg}, IsRaw: f.IsEscaped}, nil
	}
	return nil, errors.New("empty filter node")
}

func checkSyntax(f FilterNode) error {
	fs := f.NonEmptyFields()
	if len(fs) != 1 {
		if len(fs) == 0 {
			return errors.New("empty filter node")
		}
		return fmt.Errorf("multiple fields specified in the same filter node: %s", strings.Join(fs, ","))
	}
	if !f.IsEscaped {
		return nil
	}
	allowed := []string{"from", "to", "subject"}
	for _, s := range allowed {
		if fs[0] == s {
			return nil
		}
	}
	return fmt.Errorf("'isRaw' can be used only with fields %s", strings.Join(allowed, ", "))
}

func parseOperation(f FilterNode) (OperationType, []FilterNode) {
	if len(f.And) > 0 {
		return OperationAnd, f.And
	}
	if len(f.Or) > 0 {
		return OperationOr, f.Or
	}
	if f.Not != nil {
		return OperationNot, []FilterNode{*f.Not}
	}
	return OperationNone, nil
}

func parseFunction(f FilterNode) (FunctionType, string) {
	if f.From != "" {
		return FunctionFrom, f.From
	}
	if f.To != "" {
		return FunctionTo, f.To
	}
	if f.Cc != "" {
		return FunctionCc, f.Cc
	}
	if f.Bcc != "" {
		return FunctionBcc, f.Bcc
	}
	if f.ReplyTo != "" {
		return FunctionReplyTo, f.ReplyTo
	}
	if f.Subject != "" {
		return FunctionSubject, f.Subject
	}
	if f.List != "" {
		return FunctionList, f.List
	}
	if f.Has != "" {
		return FunctionHas, f.Has
	}
	if f.Query != "" {
		return FunctionQuery, f.Query
	}
	return FunctionNone, ""
}
