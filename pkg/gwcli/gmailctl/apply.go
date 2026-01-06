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
)

// DefaultContextLines is the default number of lines of context to show in the filter diff.
const DefaultContextLines = 5

// GmailConfig represents a Gmail configuration.
type GmailConfig struct {
	Labels  Labels
	Filters Filters
}

// ConfigParseRes represents the result of a config parse.
type ConfigParseRes struct {
	GmailConfig
	Rules []ParsedRule
}

// FromConfig creates a GmailConfig from a parsed configuration file.
func FromConfig(cfg Config) (ConfigParseRes, error) {
	res := ConfigParseRes{}
	var err error
	res.Rules, err = Parse(cfg)
	if err != nil {
		return res, fmt.Errorf("cannot parse config file: %w", err)
	}
	res.Filters, err = FromRules(res.Rules)
	if err != nil {
		return res, fmt.Errorf("exporting to filters: %w", err)
	}
	res.Labels = LabelsFromConfig(cfg.Labels)
	return res, nil
}

// FetchAPI provides access to Gmail get APIs.
type FetchAPI interface {
	ListFilters() (Filters, error)
	ListLabels() (Labels, error)
}

// FromAPI creates a GmailConfig from Gmail APIs.
func FromAPI(api FetchAPI) (GmailConfig, error) {
	l, err := api.ListLabels()
	if err != nil {
		return GmailConfig{}, fmt.Errorf("listing labels from Gmail: %w", err)
	}
	f, err := api.ListFilters()
	if err != nil {
		if len(f) == 0 {
			return GmailConfig{}, fmt.Errorf("getting filters from Gmail: %w", err)
		}
	}
	return GmailConfig{Labels: l, Filters: f}, err
}

// ConfigDiff contains the difference between local and upstream configuration.
type ConfigDiff struct {
	FiltersDiff FiltersDiff
	LabelsDiff  LabelsDiff
	LocalConfig GmailConfig
}

func (d ConfigDiff) String() string {
	var res []string
	if !d.FiltersDiff.Empty() {
		res = append(res, "Filters:")
		res = append(res, d.FiltersDiff.String())
	}
	if !d.LabelsDiff.Empty() {
		res = append(res, "Labels:")
		res = append(res, d.LabelsDiff.String())
	}
	return strings.Join(res, "\n")
}

// Empty returns whether the diff contains no changes.
func (d ConfigDiff) Empty() bool {
	return d.FiltersDiff.Empty() && d.LabelsDiff.Empty()
}

// Validate returns whether the given diff is valid.
func (d ConfigDiff) Validate() error {
	if d.LabelsDiff.Empty() {
		return nil
	}
	if err := d.LocalConfig.Labels.Validate(); err != nil {
		return fmt.Errorf("validating labels: %w", err)
	}
	if err := ValidateLabelsDiff(d.LabelsDiff, d.LocalConfig.Filters); err != nil {
		return fmt.Errorf("invalid labels diff: %w", err)
	}
	return nil
}

// Diff computes the diff between local and upstream configuration.
func Diff(local, upstream GmailConfig, debugInfo bool, contextLines int, colorize bool) (ConfigDiff, error) {
	res := ConfigDiff{LocalConfig: local}
	var err error
	res.FiltersDiff, err = DiffFilters(upstream.Filters, local.Filters, debugInfo, contextLines, colorize)
	if err != nil {
		return res, fmt.Errorf("cannot compute filters diff: %w", err)
	}
	if len(local.Labels) > 0 {
		res.LabelsDiff, err = DiffLabels(upstream.Labels, local.Labels, colorize)
		if err != nil {
			return res, fmt.Errorf("cannot compute labels diff: %w", err)
		}
	}
	return res, nil
}

// ApplyAPI provides access to Gmail APIs for applying changes.
type ApplyAPI interface {
	AddLabels(lbs Labels) error
	AddFilters(fs Filters) error
	UpdateLabels(lbs Labels) error
	DeleteFilters(ids []string) error
	DeleteLabels(ids []string) error
}

// Apply applies the changes identified by the diff to the remote configuration.
func Apply(d ConfigDiff, api ApplyAPI, allowRemoveLabels bool) error {
	if err := addLabels(d.LabelsDiff.Added, api); err != nil {
		return fmt.Errorf("creating labels: %w", err)
	}
	if err := addFilters(d.FiltersDiff.Added, api); err != nil {
		return fmt.Errorf("creating filters: %w", err)
	}
	if err := updateLabels(d.LabelsDiff.Modified, api); err != nil {
		return fmt.Errorf("updating labels: %w", err)
	}
	if err := removeFilters(d.FiltersDiff.Removed, api); err != nil {
		return fmt.Errorf("deleting filters: %w", err)
	}
	if !allowRemoveLabels {
		return nil
	}
	if err := removeLabels(d.LabelsDiff.Removed, api); err != nil {
		return fmt.Errorf("removing labels: %w", err)
	}
	return nil
}

func addLabels(lbs Labels, api ApplyAPI) error {
	if len(lbs) == 0 {
		return nil
	}
	sort.Sort(byLen(lbs))
	return api.AddLabels(lbs)
}

func addFilters(ls Filters, api ApplyAPI) error {
	if len(ls) > 0 {
		return api.AddFilters(ls)
	}
	return nil
}

func updateLabels(ms []ModifiedLabel, api ApplyAPI) error {
	if len(ms) == 0 {
		return nil
	}
	var lbs Labels
	for _, m := range ms {
		label := m.New
		label.ID = m.Old.ID
		lbs = append(lbs, label)
	}
	return api.UpdateLabels(lbs)
}

func removeFilters(ls Filters, api ApplyAPI) error {
	if len(ls) == 0 {
		return nil
	}
	ids := make([]string, len(ls))
	for i, f := range ls {
		ids[i] = f.ID
	}
	return api.DeleteFilters(ids)
}

func removeLabels(lbs Labels, api ApplyAPI) error {
	if len(lbs) == 0 {
		return nil
	}
	sort.Sort(byLen(lbs))
	var ids []string
	for i := len(lbs) - 1; i >= 0; i-- {
		ids = append(ids, lbs[i].ID)
	}
	return api.DeleteLabels(ids)
}

type byLen Labels

func (b byLen) Len() int           { return len(b) }
func (b byLen) Less(i, j int) bool { return len(b[i].Name) < len(b[j].Name) }
func (b byLen) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
