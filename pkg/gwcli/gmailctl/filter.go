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
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/errors"
	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/graph"
	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/reporting"
)

// There's no documented limit on filter size on Gmail, but this educated guess
// is better than nothing.
const defaultSizeLimit = 20

// Filters is a list of filters created in Gmail.
type Filters []Filter

func (fs Filters) String() string {
	w := filterWriter{}
	first := true
	for _, f := range fs {
		if !first {
			w.WriteRune('\n')
		}
		first = false
		w.WriteString(f.String())
	}
	return w.String()
}

func (fs Filters) DebugString() string {
	w := filterWriter{}
	first := true
	for _, f := range fs {
		if !first {
			w.WriteRune('\n')
		}
		first = false
		w.WriteString(f.DebugString())
	}
	return w.String()
}

// HasLabel returns true if the given label is used by at least one filter.
func (fs Filters) HasLabel(name string) bool {
	for _, f := range fs {
		if f.HasLabel(name) {
			return true
		}
	}
	return false
}

// Filter matches 1:1 a filter created on Gmail.
type Filter struct {
	ID       string
	Action   FilterActions
	Criteria FilterCriteria
}

func (f Filter) String() string {
	w := filterWriter{}
	w.WriteString("* Criteria:\n")
	w.WriteParam("from", f.Criteria.From)
	w.WriteParam("to", f.Criteria.To)
	w.WriteParam("subject", f.Criteria.Subject)
	w.WriteParam("query", indentQuery(f.Criteria.Query, 2))
	w.WriteString("  Actions:\n")
	w.WriteBool("archive", f.Action.Archive)
	w.WriteBool("delete", f.Action.Delete)
	w.WriteBool("mark as important", f.Action.MarkImportant)
	w.WriteBool("never mark as important", f.Action.MarkNotImportant)
	w.WriteBool("never mark as spam", f.Action.MarkNotSpam)
	w.WriteBool("mark as read", f.Action.MarkRead)
	w.WriteBool("star", f.Action.Star)
	w.WriteParam("categorize as", string(f.Action.Category))
	w.WriteParam("apply label", f.Action.AddLabel)
	w.WriteParam("forward to", f.Action.Forward)
	return w.String()
}

func (f Filter) DebugString() string {
	w := filterWriter{}
	w.WriteString(fmt.Sprintf("# Search: %s\n", f.Criteria.ToGmailSearch()))
	w.WriteString(fmt.Sprintf("# URL: %s\n", f.Criteria.ToGmailSearchURL()))
	w.WriteString(f.String())
	return w.String()
}

func indentQuery(query string, level int) string {
	var indented bytes.Buffer
	if !indentInternal(strings.NewReader(query), &indented, level+1) {
		return query
	}
	return "\n" + strings.TrimRight(indented.String(), "\n ")
}

func indentInternal(queryReader io.RuneReader, out *bytes.Buffer, level int) bool {
	type parseState int
	const (
		other parseState = iota
		skipSpaces
		inQuotes
	)
	for i := 0; i < level; i++ {
		out.Write([]byte("  "))
	}
	indentationWasNeeded := false
	writeIndentation := func(n int) {
		out.WriteByte('\n')
		for i := 0; i < n; i++ {
			out.Write([]byte("  "))
		}
		indentationWasNeeded = true
	}
	state := skipSpaces
	for {
		r, _, err := queryReader.ReadRune()
		if err != nil {
			break
		}
		switch state {
		case inQuotes:
			out.WriteRune(r)
			if r == '"' {
				state = other
			}
		case skipSpaces, other:
			switch r {
			case ' ':
				if state == skipSpaces {
					continue
				}
				writeIndentation(level)
			case '{', '(':
				out.WriteRune(r)
				level++
				writeIndentation(level)
				state = skipSpaces
			case '}', ')':
				writeIndentation(level - 1)
				out.WriteRune(r)
				level--
				writeIndentation(level)
				state = skipSpaces
			case ':':
				out.WriteByte(':')
				state = skipSpaces
			case '"':
				state = inQuotes
				out.WriteByte('"')
			default:
				if state != inQuotes {
					state = other
				}
				out.WriteRune(r)
			}
		}
	}
	return indentationWasNeeded
}

// HasLabel returns true if the given label is used by the filter.
func (f Filter) HasLabel(name string) bool {
	return f.Action.AddLabel == name
}

// FilterActions represents an action associated with a Gmail filter.
type FilterActions struct {
	AddLabel         string
	Category         Category
	Archive          bool
	Delete           bool
	MarkImportant    bool
	MarkNotImportant bool
	MarkRead         bool
	MarkNotSpam      bool
	Star             bool
	Forward          string
}

// Empty returns true if no action is specified.
func (a FilterActions) Empty() bool {
	return a == FilterActions{}
}

// FilterCriteria represents the filtering criteria associated with a Gmail filter.
type FilterCriteria struct {
	From    string
	To      string
	Subject string
	Query   string
}

// Empty returns true if no criteria is specified.
func (c FilterCriteria) Empty() bool {
	return c == FilterCriteria{}
}

// ToGmailSearch returns the equivalent query in Gmail search syntax.
func (c FilterCriteria) ToGmailSearch() string {
	var res []string
	if c.From != "" {
		res = append(res, fmt.Sprintf("from:%s", c.From))
	}
	if c.To != "" {
		res = append(res, fmt.Sprintf("to:%s", c.To))
	}
	if c.Subject != "" {
		res = append(res, fmt.Sprintf("subject:%s", c.Subject))
	}
	if c.Query != "" {
		res = append(res, c.Query)
	}
	return strings.Join(res, " ")
}

// ToGmailSearchURL returns the equivalent query in an URL to Gmail search.
func (c FilterCriteria) ToGmailSearchURL() string {
	return fmt.Sprintf(
		"https://mail.google.com/mail/u/0/#search/%s",
		url.QueryEscape(c.ToGmailSearch()))
}

type filterWriter struct {
	b   strings.Builder
	err error
}

func (w *filterWriter) WriteParam(name, value string) {
	if value == "" {
		return
	}
	w.WriteString("    ")
	w.WriteString(name)
	w.WriteString(": ")
	w.WriteString(value)
	w.WriteRune('\n')
}

func (w *filterWriter) WriteBool(name string, value bool) {
	if !value {
		return
	}
	w.WriteString("    ")
	w.WriteString(name)
	w.WriteRune('\n')
}

func (w *filterWriter) WriteString(a string) {
	if w.err != nil {
		return
	}
	_, w.err = w.b.WriteString(a)
}

func (w *filterWriter) WriteRune(a rune) {
	if w.err != nil {
		return
	}
	_, w.err = w.b.WriteRune(a)
}

func (w *filterWriter) String() string { return w.b.String() }

// FromRules translates rules into entries that map directly into Gmail filters.
func FromRules(rs []ParsedRule) (Filters, error) {
	return FromRulesWithLimit(rs, defaultSizeLimit)
}

// FromRulesWithLimit translates rules into entries that map directly into Gmail.
func FromRulesWithLimit(rs []ParsedRule, sizeLimit int) (Filters, error) {
	res := Filters{}
	for i, rule := range rs {
		filters, err := FromRule(rule, sizeLimit)
		if err != nil {
			return res, fmt.Errorf("generating rule #%d: %w", i, err)
		}
		res = append(res, filters...)
	}
	return res, nil
}

// FromRule translates a rule into entries that map directly into Gmail filters.
func FromRule(rule ParsedRule, sizeLimit int) (Filters, error) {
	var crits []FilterCriteria
	for _, c := range splitCriteria(rule.Criteria, sizeLimit) {
		criteria, err := GenerateCriteria(c)
		if err != nil {
			return nil, fmt.Errorf("generating criteria: %w", err)
		}
		crits = append(crits, criteria)
	}
	actions, err := generateActions(rule.Actions)
	if err != nil {
		return nil, fmt.Errorf("generating actions: %w", err)
	}
	return combineCriteriaWithActions(crits, actions), nil
}

// GenerateCriteria translates a rule criteria into an entry that maps directly into Gmail filters.
func GenerateCriteria(crit CriteriaAST) (FilterCriteria, error) {
	if node, ok := crit.(*Node); ok {
		return generateNode(node)
	}
	if leaf, ok := crit.(*Leaf); ok {
		return generateLeaf(leaf)
	}
	return FilterCriteria{}, errors.New("found unknown criteria node")
}

func generateNode(node *Node) (FilterCriteria, error) {
	switch node.Operation {
	case OperationOr:
		query := ""
		for _, child := range node.Children {
			cq, err := generateCriteriaAsString(child)
			if err != nil {
				return FilterCriteria{}, err
			}
			query = joinQueries(query, cq)
		}
		return FilterCriteria{Query: fmt.Sprintf("{%s}", query)}, nil
	case OperationAnd:
		res := FilterCriteria{}
		for _, child := range node.Children {
			crit, err := GenerateCriteria(child)
			if err != nil {
				return res, err
			}
			res = joinCriteria(res, crit)
		}
		return res, nil
	case OperationNot:
		if ln := len(node.Children); ln != 1 {
			return FilterCriteria{}, fmt.Errorf("after 'not' got %d children, expected 1", ln)
		}
		cq, err := generateCriteriaAsString(node.Children[0])
		return FilterCriteria{Query: fmt.Sprintf("-%s", cq)}, err
	}
	return FilterCriteria{}, fmt.Errorf("unknown node operation %d", node.Operation)
}

func generateLeaf(leaf *Leaf) (FilterCriteria, error) {
	needEscape := leaf.Function != FunctionQuery && !leaf.IsRaw
	query := joinStrings(needEscape, leaf.Args...)
	if len(leaf.Args) > 1 {
		var err error
		if query, err = groupWithOperation(query, leaf.Grouping); err != nil {
			return FilterCriteria{}, err
		}
	}
	switch leaf.Function {
	case FunctionFrom:
		return FilterCriteria{From: query}, nil
	case FunctionTo:
		return FilterCriteria{To: query}, nil
	case FunctionSubject:
		return FilterCriteria{Subject: query}, nil
	case FunctionCc:
		return FilterCriteria{Query: fmt.Sprintf("cc:%s", query)}, nil
	case FunctionBcc:
		return FilterCriteria{Query: fmt.Sprintf("bcc:%s", query)}, nil
	case FunctionReplyTo:
		return FilterCriteria{Query: fmt.Sprintf("replyto:%s", query)}, nil
	case FunctionList:
		return FilterCriteria{Query: fmt.Sprintf("list:%s", query)}, nil
	case FunctionHas, FunctionQuery:
		return FilterCriteria{Query: query}, nil
	default:
		return FilterCriteria{}, fmt.Errorf("unknown function type %d", leaf.Function)
	}
}

func generateCriteriaAsString(crit CriteriaAST) (string, error) {
	if node, ok := crit.(*Node); ok {
		return generateNodeAsString(node)
	}
	if leaf, ok := crit.(*Leaf); ok {
		return generateLeafAsString(leaf)
	}
	return "", errors.New("found unknown criteria node")
}

func generateNodeAsString(node *Node) (string, error) {
	query := ""
	for _, child := range node.Children {
		cq, err := generateCriteriaAsString(child)
		if err != nil {
			return "", err
		}
		query = joinQueries(query, cq)
	}
	return groupWithOperation(query, node.Operation)
}

func generateLeafAsString(leaf *Leaf) (string, error) {
	needEscape := leaf.Function != FunctionQuery && !leaf.IsRaw
	query := joinStrings(needEscape, leaf.Args...)
	if len(leaf.Args) > 1 {
		var err error
		if query, err = groupWithOperation(query, leaf.Grouping); err != nil {
			return "", err
		}
	}
	switch leaf.Function {
	case FunctionHas, FunctionQuery:
		return query, nil
	default:
		return fmt.Sprintf("%v:%s", leaf.Function, query), nil
	}
}

func groupWithOperation(query string, op OperationType) (string, error) {
	switch op {
	case OperationOr:
		return fmt.Sprintf("{%s}", query), nil
	case OperationAnd:
		return fmt.Sprintf("(%s)", query), nil
	case OperationNot:
		return fmt.Sprintf("-%s", query), nil
	default:
		return "", fmt.Errorf("unknown node operation %d", op)
	}
}

func joinCriteria(c1, c2 FilterCriteria) FilterCriteria {
	return FilterCriteria{
		From:    joinQueries(c1.From, c2.From),
		To:      joinQueries(c1.To, c2.To),
		Subject: joinQueries(c1.Subject, c2.Subject),
		Query:   joinQueries(c1.Query, c2.Query),
	}
}

func joinQueries(f1, f2 string) string {
	if f1 == "" {
		return f2
	}
	if f2 == "" {
		return f1
	}
	return fmt.Sprintf("%s %s", f1, f2)
}

func joinStrings(escape bool, a ...string) string {
	if escape {
		return joinQuoted(a...)
	}
	return strings.Join(a, " ")
}

func joinQuoted(a ...string) string {
	return strings.Join(quoteStrings(a...), " ")
}

func quoteStrings(a ...string) []string {
	res := make([]string, len(a))
	for i, s := range a {
		res[i] = quote(s)
	}
	return res
}

func quote(a string) string {
	if strings.HasPrefix(a, `"`) && strings.HasSuffix(a, `"`) {
		return a
	}
	if strings.ContainsAny(a, " \t{}()") {
		return fmt.Sprintf(`"%s"`, a)
	}
	if strings.Contains(a, "+") && !strings.Contains(a, "@") {
		return fmt.Sprintf(`"%s"`, a)
	}
	return a
}

func splitCriteria(tree CriteriaAST, limit int) []CriteriaAST {
	var res []CriteriaAST
	for _, c := range splitRootOr(tree) {
		res = append(res, splitBigCriteria(c, limit)...)
	}
	return res
}

type splitVisitor struct {
	limit int
	res   []CriteriaAST
}

func (v *splitVisitor) VisitNode(n *Node) {
	rem := n.Children
	for len(rem) > v.limit {
		v.res = append(v.res, &Node{Operation: n.Operation, Children: rem[:v.limit]})
		rem = rem[v.limit:]
	}
	v.res = append(v.res, &Node{Operation: n.Operation, Children: rem})
}

func (v *splitVisitor) VisitLeaf(n *Leaf) {
	rem := n.Args
	for len(rem) > v.limit {
		v.res = append(v.res, &Leaf{Function: n.Function, Grouping: n.Grouping, IsRaw: n.IsRaw, Args: rem[:v.limit]})
		rem = rem[v.limit:]
	}
	v.res = append(v.res, &Leaf{Function: n.Function, Grouping: n.Grouping, IsRaw: n.IsRaw, Args: rem})
}

func splitBigCriteria(tree CriteriaAST, limit int) []CriteriaAST {
	if size := countNodes(tree); size < limit {
		return []CriteriaAST{tree}
	}
	if tree.RootOperation() == OperationOr {
		sv := splitVisitor{limit: limit}
		tree.AcceptVisitor(&sv)
		return sv.res
	}
	if tree.RootOperation() == OperationAnd {
		return splitNestedAnd(tree, limit)
	}
	return []CriteriaAST{tree}
}

func splitNestedAnd(root CriteriaAST, limit int) []CriteriaAST {
	n, ok := root.(*Node)
	if !ok {
		return []CriteriaAST{root}
	}
	maxChildren := 0
	childID := -1
	for i, c := range n.Children {
		if count := countNodes(c); count > maxChildren && c.RootOperation() == OperationOr {
			childID = i
			maxChildren = count
		}
	}
	if childID < 0 {
		return []CriteriaAST{root}
	}
	bigChild := n.Children[childID]
	siblingsSize := countNodes(root) - maxChildren
	newLimit := limit - siblingsSize
	if newLimit < 1 {
		newLimit = 1
	}
	sv := splitVisitor{limit: newLimit}
	bigChild.AcceptVisitor(&sv)
	var siblings []CriteriaAST
	for i, c := range n.Children {
		if i == childID {
			continue
		}
		siblings = append(siblings, c)
	}
	var res []CriteriaAST
	for _, c := range sv.res {
		res = append(res, &Node{
			Operation: OperationAnd,
			Children:  append([]CriteriaAST{c}, cloneCriteria(siblings)...),
		})
	}
	return res
}

func cloneCriteria(tl []CriteriaAST) []CriteriaAST {
	var res []CriteriaAST
	for _, n := range tl {
		res = append(res, n.Clone())
	}
	return res
}

type countVisitor struct{ res int }

func (v *countVisitor) VisitNode(n *Node) {
	for _, c := range n.Children {
		c.AcceptVisitor(v)
	}
	v.res++
}
func (v *countVisitor) VisitLeaf(n *Leaf) {
	v.res += len(n.Args)
}

func countNodes(tree CriteriaAST) int {
	cv := countVisitor{}
	tree.AcceptVisitor(&cv)
	return cv.res
}

func splitRootOr(tree CriteriaAST) []CriteriaAST {
	root, ok := tree.(*Node)
	if !ok || root.Operation != OperationOr {
		return []CriteriaAST{tree}
	}
	return root.Children
}

func generateActions(actions Actions) ([]FilterActions, error) {
	res := []FilterActions{
		{
			Archive:          actions.Archive,
			Delete:           actions.Delete,
			MarkImportant:    fromOptionalBool(actions.MarkImportant, true),
			MarkNotImportant: fromOptionalBool(actions.MarkImportant, false),
			MarkRead:         actions.MarkRead,
			Category:         actions.Category,
			MarkNotSpam:      fromOptionalBool(actions.MarkSpam, false),
			Star:             actions.Star,
			Forward:          actions.Forward,
		},
	}
	if fromOptionalBool(actions.MarkSpam, true) {
		return nil, errors.New("gmail filters don't allow one to send messages to spam directly")
	}
	if len(actions.Labels) == 0 {
		return res, nil
	}
	res[0].AddLabel = actions.Labels[0]
	for _, label := range actions.Labels[1:] {
		res = append(res, FilterActions{AddLabel: label})
	}
	return res, nil
}

func fromOptionalBool(opt *bool, positive bool) bool {
	if opt == nil {
		return false
	}
	return *opt == positive
}

func combineCriteriaWithActions(criteria []FilterCriteria, actions []FilterActions) Filters {
	var res Filters
	for _, c := range criteria {
		for _, a := range actions {
			res = append(res, Filter{Criteria: c, Action: a})
		}
	}
	return res
}

// FiltersDiff contains filters that have been added and removed locally.
type FiltersDiff struct {
	Added          Filters
	Removed        Filters
	PrintDebugInfo bool
	ContextLines   int
	Colorize       bool
}

// Empty returns true if the diff is empty.
func (f FiltersDiff) Empty() bool {
	return len(f.Added) == 0 && len(f.Removed) == 0
}

func (f FiltersDiff) String() string {
	var removed, added string
	if f.PrintDebugInfo {
		removed = f.Removed.DebugString()
		added = f.Added.DebugString()
	} else {
		removed = f.Removed.String()
		added = f.Added.String()
	}
	s, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(removed),
		B:        difflib.SplitLines(added),
		FromFile: "Current",
		ToFile:   "TO BE APPLIED",
		Context:  f.ContextLines,
	})
	if err != nil {
		return fmt.Sprintf("Removed:\n%s\nAdded:\n%s", removed, added)
	}
	if f.Colorize {
		s = reporting.ColorizeDiff(s)
	}
	return s
}

// DiffFilters computes the diff between two lists of filters.
func DiffFilters(upstream, local Filters, debugInfo bool, contextLines int, colorize bool) (FiltersDiff, error) {
	added, removed := changedFilters(upstream, local)
	return NewMinimalFiltersDiff(added, removed, debugInfo, contextLines, colorize), nil
}

// NewMinimalFiltersDiff creates a new FiltersDiff with reordered filters.
func NewMinimalFiltersDiff(added, removed Filters, printDebugInfo bool, contextLines int, colorize bool) FiltersDiff {
	if len(added) > 0 && len(removed) > 0 {
		added, removed = reorderWithHungarian(added, removed)
	}
	return FiltersDiff{added, removed, printDebugInfo, contextLines, colorize}
}

func changedFilters(upstream, local Filters) (added, removed Filters) {
	hupstream := newHashedFilters(upstream)
	hlocal := newHashedFilters(local)
	i, j := 0, 0
	for i < len(hupstream) && j < len(hlocal) {
		ups := hupstream[i]
		loc := hlocal[j]
		cmp := strings.Compare(ups.hash, loc.hash)
		switch {
		case cmp < 0:
			removed = append(removed, ups.filter)
			i++
		case cmp > 0:
			added = append(added, loc.filter)
			j++
		default:
			i++
			j++
		}
	}
	for ; i < len(hupstream); i++ {
		removed = append(removed, hupstream[i].filter)
	}
	for ; j < len(hlocal); j++ {
		added = append(added, hlocal[j].filter)
	}
	return added, removed
}

type hashedFilter struct {
	hash   string
	filter Filter
}

type hashedFilters []hashedFilter

func (hs hashedFilters) Len() int           { return len(hs) }
func (hs hashedFilters) Less(i, j int) bool { return strings.Compare(hs[i].hash, hs[j].hash) == -1 }
func (hs hashedFilters) Swap(i, j int)      { hs[i], hs[j] = hs[j], hs[i] }

func newHashedFilters(fs Filters) hashedFilters {
	uniqueFs := map[string]Filter{}
	for _, f := range fs {
		hf := hashFilter(f)
		uniqueFs[hf.hash] = f
	}
	res := hashedFilters{}
	for h, f := range uniqueFs {
		res = append(res, hashedFilter{h, f})
	}
	sort.Sort(res)
	return res
}

func hashFilter(f Filter) hashedFilter {
	noIDFilter := Filter{Action: f.Action, Criteria: f.Criteria}
	h := hashStruct(noIDFilter)
	return hashedFilter{h, f}
}

func hashStruct(a interface{}) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%#v", a)))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func reorderWithHungarian(f1, f2 Filters) (Filters, Filters) {
	c := costMatrix(f1, f2)
	mapping := hungarian(c)
	return reorderWithMapping(f1, f2, mapping)
}

func costMatrix(fs1, fs2 Filters) [][]float64 {
	ss1 := filterStrings(fs1)
	ss2 := filterStrings(fs2)
	var c [][]float64
	for i, s1 := range ss1 {
		c = append(c, nil)
		for _, s2 := range ss2 {
			c[i] = append(c[i], diffCost(s1, s2))
		}
	}
	return c
}

type filterLines []string

func filterStrings(fs Filters) []filterLines {
	var res []filterLines
	for _, f := range fs {
		res = append(res, difflib.SplitLines(f.String()))
	}
	return res
}

func diffCost(s1, s2 filterLines) float64 {
	m := difflib.NewMatcher(s1, s2)
	return 1 - m.Ratio()
}

func hungarian(c [][]float64) []int {
	if len(c) == 0 {
		return nil
	}
	var mnk graph.Munkres
	mnk.Init(len(c), len(c[0]))
	mnk.SetCostMatrix(c)
	mnk.Run()
	return mnk.Links
}

func reorderWithMapping(f1, f2 Filters, mapping []int) (Filters, Filters) {
	var r1, r2 Filters
	mappedF1 := map[int]struct{}{}
	mappedF2 := map[int]struct{}{}
	for i, j := range mapping {
		if j < 0 {
			continue
		}
		r1 = append(r1, f1[i])
		r2 = append(r2, f2[j])
		mappedF1[i] = struct{}{}
		mappedF2[j] = struct{}{}
	}
	for i, f := range f1 {
		if _, ok := mappedF1[i]; !ok {
			r1 = append(r1, f)
		}
	}
	for i, f := range f2 {
		if _, ok := mappedF2[i]; !ok {
			r2 = append(r2, f)
		}
	}
	return r1, r2
}
