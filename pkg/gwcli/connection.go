package gwcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wesnick/cmdg/pkg/gwcli/gmailctl"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
)

// CmdG is the main struct for gwcli operations
type CmdG struct {
	gmail        *gmail.Service
	configPaths  *ConfigPaths
	messageCache map[string]*Message
	labelCache   map[string]*Label
	m            sync.Mutex
}

// New creates a new CmdG connection with gmailctl-based authentication
func New(configDir string) (*CmdG, error) {
	paths, err := GetConfigPaths(configDir)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	gmailSvc, err := InitializeAuth(ctx, paths)
	if err != nil {
		return nil, err
	}

	c := &CmdG{
		gmail:        gmailSvc,
		configPaths:  paths,
		messageCache: make(map[string]*Message),
		labelCache:   make(map[string]*Label),
	}

	return c, nil
}

// GmailService returns the Gmail API service
func (c *CmdG) GmailService() *gmail.Service {
	return c.gmail
}

// Labels returns all cached labels
func (c *CmdG) Labels() map[string]*Label {
	c.m.Lock()
	defer c.m.Unlock()

	// Return a copy to prevent external modification
	labels := make(map[string]*Label, len(c.labelCache))
	for k, v := range c.labelCache {
		labels[k] = v
	}
	return labels
}

// LoadLabelsFromConfig loads labels from gmailctl Jsonnet config
func (c *CmdG) LoadLabelsFromConfig(configPath string) error {
	// Try to read config
	cfg, err := gmailctl.ReadFile(configPath, "")
	if err != nil {
		if errors.Is(err, gmailctl.ErrNotFound) {
			return nil // Config doesn't exist - not an error
		}
		return fmt.Errorf("reading config: %w", err)
	}

	c.m.Lock()
	defer c.m.Unlock()

	// Convert gmailctl labels to our Label type
	for _, label := range cfg.Labels {
		c.labelCache[label.Name] = &Label{
			ID:    label.Name, // Will be resolved to actual ID later
			Label: label.Name,
		}
	}

	return nil
}

// LoadLabels loads labels from config (if exists) or Gmail API
func (c *CmdG) LoadLabels(ctx context.Context) error {
	// Try config file first
	if c.configPaths != nil {
		if err := c.LoadLabelsFromConfig(c.configPaths.Config); err == nil {
			// Successfully loaded from config, now resolve IDs
			if len(c.labelCache) > 0 {
				return c.resolveLabelIDs(ctx)
			}
		}

		// Try gmailctl location as fallback
		home, _ := os.UserHomeDir()
		if home != "" {
			gmailctlConfig := filepath.Join(home, ".gmailctl", "config.jsonnet")
			if err := c.LoadLabelsFromConfig(gmailctlConfig); err == nil {
				if len(c.labelCache) > 0 {
					return c.resolveLabelIDs(ctx)
				}
			}
		}
	}

	// Fall back to Gmail API
	return c.loadLabelsFromAPI(ctx)
}

// resolveLabelIDs resolves label names to IDs using Gmail API
func (c *CmdG) resolveLabelIDs(ctx context.Context) error {
	// Fetch all labels from API
	resp, err := c.gmail.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("fetching labels from API: %w", err)
	}

	// Create name → ID mapping
	nameToID := make(map[string]string)
	nameToLabel := make(map[string]*gmail.Label)
	for _, label := range resp.Labels {
		nameToID[label.Name] = label.Id
		nameToLabel[label.Name] = label
	}

	// Update our cache with actual IDs
	c.m.Lock()
	defer c.m.Unlock()

	for name, label := range c.labelCache {
		if id, ok := nameToID[name]; ok {
			label.ID = id
			if gmailLabel, ok := nameToLabel[name]; ok {
				label.Response = gmailLabel
			}
		}
	}

	return nil
}

// loadLabelsFromAPI loads labels directly from Gmail API (fallback)
func (c *CmdG) loadLabelsFromAPI(ctx context.Context) error {
	resp, err := c.gmail.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("fetching labels from API: %w", err)
	}

	c.m.Lock()
	defer c.m.Unlock()

	for _, l := range resp.Labels {
		c.labelCache[l.Id] = &Label{
			ID:       l.Id,
			Label:    l.Name,
			Response: l,
		}
	}

	return nil
}

// BatchLabel applies a label to multiple messages
func (c *CmdG) BatchLabel(ctx context.Context, messageIDs []string, labelID string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	req := &gmail.ModifyMessageRequest{
		AddLabelIds: []string{labelID},
	}

	for _, msgID := range messageIDs {
		if _, err := c.gmail.Users.Messages.Modify("me", msgID, req).Context(ctx).Do(); err != nil {
			return fmt.Errorf("labeling message %s: %w", msgID, err)
		}
	}

	return nil
}

// BatchUnlabel removes a label from multiple messages
func (c *CmdG) BatchUnlabel(ctx context.Context, messageIDs []string, labelID string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	req := &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{labelID},
	}

	for _, msgID := range messageIDs {
		if _, err := c.gmail.Users.Messages.Modify("me", msgID, req).Context(ctx).Do(); err != nil {
			return fmt.Errorf("unlabeling message %s: %w", msgID, err)
		}
	}

	return nil
}

// BatchTrash moves multiple messages to trash
func (c *CmdG) BatchTrash(ctx context.Context, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	for _, msgID := range messageIDs {
		if _, err := c.gmail.Users.Messages.Trash("me", msgID).Context(ctx).Do(); err != nil {
			return fmt.Errorf("trashing message %s: %w", msgID, err)
		}
	}

	return nil
}

// MessagePage represents a page of messages
type MessagePage struct {
	Messages       []*gmail.Message
	NextPageToken  string
	ResultSizeEstimate int64
}

// ListMessages lists messages matching the given criteria
func (c *CmdG) ListMessages(ctx context.Context, labelID, query, pageToken string) (*MessagePage, error) {
	req := c.gmail.Users.Messages.List("me")

	if labelID != "" {
		req = req.LabelIds(labelID)
	}

	if query != "" {
		req = req.Q(query)
	}

	if pageToken != "" {
		req = req.PageToken(pageToken)
	}

	resp, err := req.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("listing messages: %w", err)
	}

	return &MessagePage{
		Messages:           resp.Messages,
		NextPageToken:      resp.NextPageToken,
		ResultSizeEstimate: resp.ResultSizeEstimate,
	}, nil
}

// ThreadID represents a Gmail thread ID
type ThreadID string

// SendParts sends a message with multipart content
func (c *CmdG) SendParts(ctx context.Context, threadID ThreadID, multipartType string, headers map[string]string, parts []io.Reader) error {
	// This is a simplified implementation - you may need to expand based on actual usage
	return fmt.Errorf("SendParts not yet implemented")
}

// TokenInfo represents OAuth token information
type TokenInfo struct {
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	ExpiresIn     int64    `json:"expires_in"`
	Scope         string   `json:"scope"`
	Scopes        []string `json:"scopes,omitempty"`
	UserID        string   `json:"user_id"`
	Audience      string   `json:"aud"`
	IssuedTo      string   `json:"issued_to"`
	AppName       string   `json:"app_name,omitempty"`
}

// GetTokenInfo retrieves information about the OAuth token
func (c *CmdG) GetTokenInfo(ctx context.Context) (*TokenInfo, error) {
	// Get the token from the token file
	tokenFile, err := os.Open(c.configPaths.Token)
	if err != nil {
		return nil, fmt.Errorf("opening token file: %w", err)
	}
	defer tokenFile.Close()

	var token oauth2.Token
	if err := json.NewDecoder(tokenFile).Decode(&token); err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}

	// Call Google's tokeninfo endpoint
	url := fmt.Sprintf("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=%s", token.AccessToken)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("calling tokeninfo endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tokeninfo returned status %d", resp.StatusCode)
	}

	var info TokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding tokeninfo response: %w", err)
	}

	// Parse scopes from space-separated string if scopes array is empty
	if len(info.Scopes) == 0 && info.Scope != "" {
		info.Scopes = strings.Split(info.Scope, " ")
	}

	return &info, nil
}

// GetProfile retrieves the user's Gmail profile
func (c *CmdG) GetProfile(ctx context.Context) (*gmail.Profile, error) {
	profile, err := c.gmail.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("getting profile: %w", err)
	}
	return profile, nil
}

// Configure runs the OAuth configuration flow (legacy compatibility)
func Configure(configPath string) error {
	// This is the old-style configure that uses a single config file path
	// We need to convert it to the new paths structure

	// Extract directory from path
	configDir := filepath.Dir(configPath)

	paths, err := GetConfigPaths(configDir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	return ConfigureAuth(ctx, paths, 8080)
}
