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
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
	gmailv1 "google.golang.org/api/gmail/v1"

	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/errors"
	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/reporting"
)

// Label IDs for special Gmail labels
const (
	labelIDInbox     = "INBOX"
	labelIDTrash     = "TRASH"
	labelIDImportant = "IMPORTANT"
	labelIDUnread    = "UNREAD"
	labelIDSpam      = "SPAM"
	labelIDStar      = "STARRED"

	labelIDCategoryPersonal   = "CATEGORY_PERSONAL"
	labelIDCategorySocial     = "CATEGORY_SOCIAL"
	labelIDCategoryUpdates    = "CATEGORY_UPDATES"
	labelIDCategoryForums     = "CATEGORY_FORUMS"
	labelIDCategoryPromotions = "CATEGORY_PROMOTIONS"
)

// LabelMap maps label names and IDs together.
type LabelMap struct {
	ntid map[string]string
	idtn map[string]string
}

// NewLabelMap creates a new LabelMap given a list of labels.
func NewLabelMap(labels []GmailLabel) LabelMap {
	nameIDMap := map[string]string{}
	idNameMap := map[string]string{}
	for _, l := range labels {
		nameIDMap[l.Name] = l.ID
		idNameMap[l.ID] = l.Name
	}
	return LabelMap{ntid: nameIDMap, idtn: idNameMap}
}

// NameToID maps the name of a label to its ID.
func (m LabelMap) NameToID(name string) (string, bool) {
	id, ok := m.ntid[name]
	return id, ok
}

// IDToName maps the id of a string to its name.
func (m LabelMap) IDToName(id string) (string, bool) {
	name, ok := m.idtn[id]
	return name, ok
}

// AddLabel adds a label to the mapping
func (m LabelMap) AddLabel(id, name string) {
	m.ntid[name] = id
	m.idtn[id] = name
}

// ExportFilters exports Gmail filters into Gmail API objects
func ExportFilters(filters Filters, lmap LabelMap) ([]*gmailv1.Filter, error) {
	res := make([]*gmailv1.Filter, len(filters))
	for i, filter := range filters {
		ef, err := exportFilter(filter, lmap)
		if err != nil {
			return nil, errors.WithDetails(
				fmt.Errorf("exporting filter #%d: %w", i, err),
				fmt.Sprintf("Filter (internal representation): %s", reporting.Prettify(filter, false)),
			)
		}
		res[i] = ef
	}
	return res, nil
}

func exportFilter(filter Filter, lmap LabelMap) (*gmailv1.Filter, error) {
	if filter.Action.Empty() {
		return nil, errors.New("no action specified")
	}
	if filter.Criteria.Empty() {
		return nil, errors.New("no criteria specified")
	}
	action, err := exportAction(filter.Action, lmap)
	if err != nil {
		return nil, fmt.Errorf("in export action: %w", err)
	}
	criteria, err := exportCriteria(filter.Criteria)
	if err != nil {
		return nil, fmt.Errorf("in export criteria: %w", err)
	}
	return &gmailv1.Filter{Action: action, Criteria: criteria}, nil
}

func exportAction(action FilterActions, lmap LabelMap) (*gmailv1.FilterAction, error) {
	lops := labelOps{}
	exportFlags(action, &lops)
	if action.Category != "" {
		cat, err := exportCategory(action.Category)
		if err != nil {
			return nil, err
		}
		lops.AddLabel(cat)
	}
	if action.AddLabel != "" {
		id, ok := lmap.NameToID(action.AddLabel)
		if !ok {
			return nil, fmt.Errorf("label %q not found", action.AddLabel)
		}
		lops.AddLabel(id)
	}
	return &gmailv1.FilterAction{
		AddLabelIds:    lops.addLabels,
		RemoveLabelIds: lops.removeLabels,
		Forward:        action.Forward,
	}, nil
}

func exportFlags(action FilterActions, lops *labelOps) {
	if action.Archive {
		lops.RemoveLabel(labelIDInbox)
	}
	if action.Delete {
		lops.AddLabel(labelIDTrash)
	}
	if action.MarkImportant {
		lops.AddLabel(labelIDImportant)
	}
	if action.MarkNotImportant {
		lops.RemoveLabel(labelIDImportant)
	}
	if action.MarkRead {
		lops.RemoveLabel(labelIDUnread)
	}
	if action.MarkNotSpam {
		lops.RemoveLabel(labelIDSpam)
	}
	if action.Star {
		lops.AddLabel(labelIDStar)
	}
}

func exportCategory(category Category) (string, error) {
	switch category {
	case CategoryPersonal:
		return labelIDCategoryPersonal, nil
	case CategorySocial:
		return labelIDCategorySocial, nil
	case CategoryUpdates:
		return labelIDCategoryUpdates, nil
	case CategoryForums:
		return labelIDCategoryForums, nil
	case CategoryPromotions:
		return labelIDCategoryPromotions, nil
	}
	return "", fmt.Errorf("unknown category %q", category)
}

func exportCriteria(criteria FilterCriteria) (*gmailv1.FilterCriteria, error) {
	return &gmailv1.FilterCriteria{
		From:    criteria.From,
		To:      criteria.To,
		Subject: criteria.Subject,
		Query:   criteria.Query,
	}, nil
}

type labelOps struct {
	addLabels    []string
	removeLabels []string
}

func (o *labelOps) AddLabel(name string)    { o.addLabels = append(o.addLabels, name) }
func (o *labelOps) RemoveLabel(name string) { o.removeLabels = append(o.removeLabels, name) }

// Unsupported fields maps
var (
	unsupportedCriteriaFields = map[string]bool{
		"ExcludeChats":   true,
		"Size":           true,
		"SizeComparison": true,
	}
	unsupportedActionFields = map[string]bool{}
)

// ImportFilters imports Gmail API filters into internal format.
func ImportFilters(filters []*gmailv1.Filter, lmap LabelMap) (Filters, error) {
	res := Filters{}
	var reserr error
	for _, gfilter := range filters {
		impFilter, err := importFilter(gfilter, lmap)
		if err != nil {
			err = fmt.Errorf("importing filter %q: %w", gfilter.Id, err)
			reserr = multierror.Append(reserr, err)
		} else {
			res = append(res, impFilter)
		}
	}
	return res, reserr
}

func importFilter(gf *gmailv1.Filter, lmap LabelMap) (Filter, error) {
	action, err := importAction(gf.Action, lmap)
	if err != nil {
		return Filter{}, fmt.Errorf("importing action: %w", err)
	}
	criteria, err := importCriteria(gf.Criteria)
	if err != nil {
		return Filter{}, fmt.Errorf("importing criteria: %w", err)
	}
	return Filter{ID: gf.Id, Action: action, Criteria: criteria}, nil
}

func importAction(action *gmailv1.FilterAction, lmap LabelMap) (FilterActions, error) {
	res := FilterActions{}
	if action == nil {
		return res, errors.New("empty action")
	}
	if err := checkUnsupportedFields(*action, unsupportedActionFields); err != nil {
		return res, fmt.Errorf("criteria: %w", err)
	}
	if err := importAddLabels(&res, action.AddLabelIds, lmap); err != nil {
		return res, err
	}
	if err := importRemoveLabels(&res, action.RemoveLabelIds); err != nil {
		return res, err
	}
	res.Forward = action.Forward
	if res.Empty() {
		return res, errors.New("empty or unsupported action")
	}
	return res, nil
}

func importAddLabels(res *FilterActions, addLabelIDs []string, lmap LabelMap) error {
	for _, labelID := range addLabelIDs {
		category := importCategory(labelID)
		if category != "" {
			if res.Category != "" {
				return fmt.Errorf("multiple categories: '%s', '%s'", category, res.Category)
			}
			res.Category = category
			continue
		}
		switch labelID {
		case labelIDTrash:
			res.Delete = true
		case labelIDImportant:
			res.MarkImportant = true
		case labelIDStar:
			res.Star = true
		default:
			labelName, ok := lmap.IDToName(labelID)
			if !ok {
				return fmt.Errorf("unknown label ID '%s'", labelID)
			}
			res.AddLabel = labelName
		}
	}
	return nil
}

func importRemoveLabels(res *FilterActions, removeLabelIDs []string) error {
	for _, labelID := range removeLabelIDs {
		switch labelID {
		case labelIDInbox:
			res.Archive = true
		case labelIDUnread:
			res.MarkRead = true
		case labelIDImportant:
			res.MarkNotImportant = true
		case labelIDSpam:
			res.MarkNotSpam = true
		default:
			return fmt.Errorf("unupported label to remove %q", labelID)
		}
	}
	return nil
}

func importCategory(labelID string) Category {
	switch labelID {
	case labelIDCategoryPersonal:
		return CategoryPersonal
	case labelIDCategorySocial:
		return CategorySocial
	case labelIDCategoryUpdates:
		return CategoryUpdates
	case labelIDCategoryForums:
		return CategoryForums
	case labelIDCategoryPromotions:
		return CategoryPromotions
	default:
		return ""
	}
}

func importCriteria(criteria *gmailv1.FilterCriteria) (FilterCriteria, error) {
	if criteria == nil {
		return FilterCriteria{}, errors.New("empty criteria")
	}
	if err := checkUnsupportedFields(*criteria, unsupportedCriteriaFields); err != nil {
		return FilterCriteria{}, fmt.Errorf("criteria: %w", err)
	}
	query := appendQuery(nil, criteria.Query)
	if criteria.NegatedQuery != "" {
		query = appendQuery(query, fmt.Sprintf("-{%s}", criteria.NegatedQuery))
	}
	if criteria.HasAttachment {
		query = appendQuery(query, "has:attachment")
	}
	return FilterCriteria{
		From:    criteria.From,
		To:      criteria.To,
		Subject: criteria.Subject,
		Query:   strings.Join(query, " "),
	}, nil
}

func checkUnsupportedFields(a interface{}, unsupported map[string]bool) error {
	t := reflect.TypeOf(a)
	v := reflect.ValueOf(a)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		name := t.Field(i).Name
		if !unsupported[name] {
			continue
		}
		if !isDefault(field) {
			return fmt.Errorf("usage of unsupported field %q (value %v)", name, field.Interface())
		}
	}
	return nil
}

func appendQuery(q []string, a string) []string {
	if a == "" {
		return q
	}
	return append(q, a)
}

func isDefault(v reflect.Value) bool {
	return reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}
