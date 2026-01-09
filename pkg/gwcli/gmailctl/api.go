// Copyright 2024 Wes Nick
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

package gmailctl

import (
	"context"
	"fmt"

	gmailv1 "google.golang.org/api/gmail/v1"

	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl/errors"
)

// GmailAPI wraps Gmail API operations for filters and labels.
// It accepts a pre-authenticated *gmail.Service from gwcli's auth layer,
// enabling service account support.
type GmailAPI struct {
	svc    *gmailv1.Service
	userID string
	ctx    context.Context
}

// NewGmailAPI creates a new API wrapper.
// userID should be "me" for OAuth or the impersonated email for service accounts.
func NewGmailAPI(ctx context.Context, svc *gmailv1.Service, userID string) *GmailAPI {
	if userID == "" {
		userID = "me"
	}
	return &GmailAPI{svc: svc, userID: userID, ctx: ctx}
}

// ListFilters returns all filters from Gmail.
func (a *GmailAPI) ListFilters() (Filters, error) {
	resp, err := a.svc.Users.Settings.Filters.List(a.userID).Context(a.ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("listing filters: %w", err)
	}

	// Get labels for mapping
	labels, err := a.ListLabels()
	if err != nil {
		return nil, fmt.Errorf("listing labels for filter import: %w", err)
	}
	lmap := NewLabelMap(labels)

	return ImportFilters(resp.Filter, lmap)
}

// ListLabels returns all user labels from Gmail.
func (a *GmailAPI) ListLabels() (Labels, error) {
	resp, err := a.svc.Users.Labels.List(a.userID).Context(a.ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("listing labels: %w", err)
	}

	var labels Labels
	for _, l := range resp.Labels {
		// Skip system labels
		if l.Type == "system" {
			continue
		}
		label := GmailLabel{
			ID:   l.Id,
			Name: l.Name,
		}
		if l.Color != nil {
			label.Color = &GmailLabelColor{
				Background: l.Color.BackgroundColor,
				Text:       l.Color.TextColor,
			}
		}
		labels = append(labels, label)
	}
	return labels, nil
}

// AddLabels creates new labels in Gmail.
func (a *GmailAPI) AddLabels(lbs Labels) error {
	for _, l := range lbs {
		label := &gmailv1.Label{
			Name:                  l.Name,
			LabelListVisibility:   "labelShow",
			MessageListVisibility: "show",
		}
		if l.Color != nil {
			label.Color = &gmailv1.LabelColor{
				BackgroundColor: l.Color.Background,
				TextColor:       l.Color.Text,
			}
		}
		_, err := a.svc.Users.Labels.Create(a.userID, label).Context(a.ctx).Do()
		if err != nil {
			return fmt.Errorf("creating label %q: %w", l.Name, err)
		}
	}
	return nil
}

// AddFilters creates new filters in Gmail.
func (a *GmailAPI) AddFilters(fs Filters) error {
	// Get current labels for mapping
	labels, err := a.ListLabels()
	if err != nil {
		return fmt.Errorf("listing labels for filter export: %w", err)
	}
	lmap := NewLabelMap(labels)

	// Export filters to Gmail API format
	gmailFilters, err := ExportFilters(fs, lmap)
	if err != nil {
		return fmt.Errorf("exporting filters: %w", err)
	}

	for i, gf := range gmailFilters {
		_, err := a.svc.Users.Settings.Filters.Create(a.userID, gf).Context(a.ctx).Do()
		if err != nil {
			return fmt.Errorf("creating filter #%d: %w", i, err)
		}
	}
	return nil
}

// UpdateLabels updates existing labels in Gmail.
func (a *GmailAPI) UpdateLabels(lbs Labels) error {
	for _, l := range lbs {
		if l.ID == "" {
			return errors.New("cannot update label without ID")
		}
		label := &gmailv1.Label{
			Name: l.Name,
		}
		if l.Color != nil {
			label.Color = &gmailv1.LabelColor{
				BackgroundColor: l.Color.Background,
				TextColor:       l.Color.Text,
			}
		}
		_, err := a.svc.Users.Labels.Patch(a.userID, l.ID, label).Context(a.ctx).Do()
		if err != nil {
			return fmt.Errorf("updating label %q: %w", l.Name, err)
		}
	}
	return nil
}

// DeleteFilters deletes filters from Gmail by ID.
func (a *GmailAPI) DeleteFilters(ids []string) error {
	for _, id := range ids {
		err := a.svc.Users.Settings.Filters.Delete(a.userID, id).Context(a.ctx).Do()
		if err != nil {
			return fmt.Errorf("deleting filter %q: %w", id, err)
		}
	}
	return nil
}

// DeleteLabels deletes labels from Gmail by ID.
func (a *GmailAPI) DeleteLabels(ids []string) error {
	for _, id := range ids {
		err := a.svc.Users.Labels.Delete(a.userID, id).Context(a.ctx).Do()
		if err != nil {
			return fmt.Errorf("deleting label %q: %w", id, err)
		}
	}
	return nil
}
