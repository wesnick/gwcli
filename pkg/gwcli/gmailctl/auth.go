// Copyright (c) 2017 Michele Bertasi
// Licensed under the MIT License
// Vendored from github.com/mbrt/gmailctl

package gmailctl

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/tasks/v1"
)

// NewAuthenticator creates an Authenticator instance from credentials JSON file contents.
//
// Credentials can be obtained by creating a new OAuth client ID at the Google API console
// https://console.developers.google.com/apis/credentials.
func NewAuthenticator(credentials io.Reader) (*Authenticator, error) {
	cfg, err := clientFromCredentials(credentials)
	if err != nil {
		return nil, fmt.Errorf("creating config from credentials: %w", err)
	}
	return &Authenticator{
		State: generateOauthState(),
		cfg:   cfg,
	}, nil
}

// Authenticator encapsulates authentication operations for Gmail APIs.
type Authenticator struct {
	State string
	cfg   *oauth2.Config
}

// Service creates a Gmail API service from a token JSON file contents.
//
// If no token is available, AuthURL and CacheToken can be used to
// obtain one.
func (a Authenticator) Service(ctx context.Context, token io.Reader) (*gmail.Service, error) {
	tok, err := parseToken(token)
	if err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}
	return gmail.NewService(ctx, option.WithTokenSource(a.cfg.TokenSource(ctx, tok)))
}

// AuthURL returns the URL the user has to visit to authorize the
// application and obtain an auth code.
func (a Authenticator) AuthURL(redirectURL string) string {
	a.cfg.RedirectURL = redirectURL
	return a.cfg.AuthCodeURL(a.State, oauth2.AccessTypeOffline)
}

// CacheToken creates and caches a token JSON file from an auth code.
//
// The token can be subsequently used to authorize a GmailAPI instance.
func (a Authenticator) CacheToken(ctx context.Context, authCode string, token io.Writer) error {
	tok, err := a.cfg.Exchange(ctx, authCode)
	if err != nil {
		return fmt.Errorf("unable to retrieve token from web: %w", err)
	}
	return json.NewEncoder(token).Encode(tok)
}

func clientFromCredentials(credentials io.Reader) (*oauth2.Config, error) {
	credBytes, err := io.ReadAll(credentials)
	if err != nil {
		return nil, fmt.Errorf("reading credentials: %w", err)
	}
	return google.ConfigFromJSON(credBytes,
		gmail.GmailModifyScope,
		gmail.GmailSettingsBasicScope,
		gmail.GmailLabelsScope,
		tasks.TasksScope,
		calendar.CalendarScope,
	)
}

func parseToken(token io.Reader) (*oauth2.Token, error) {
	tok := &oauth2.Token{}
	err := json.NewDecoder(token).Decode(tok)
	return tok, err
}

func generateOauthState() string {
	b := make([]byte, 128)
	if _, err := rand.Read(b); err != nil {
		// We can't really afford errors in secure random number generation.
		panic(err)
	}
	state := base64.URLEncoding.EncodeToString(b)
	return state
}

// ServiceAccountAuthenticator encapsulates service account authentication for Gmail APIs.
// Service accounts are used for Google Workspace domain-wide delegation.
type ServiceAccountAuthenticator struct {
	credBytes []byte
	userEmail string
}

// NewServiceAccountAuthenticator creates a ServiceAccountAuthenticator from credentials JSON.
// The userEmail parameter specifies which user the service account should impersonate.
func NewServiceAccountAuthenticator(credentials io.Reader, userEmail string) (*ServiceAccountAuthenticator, error) {
	credBytes, err := io.ReadAll(credentials)
	if err != nil {
		return nil, fmt.Errorf("reading service account credentials: %w", err)
	}

	if userEmail == "" {
		return nil, fmt.Errorf("user email is required for service account authentication")
	}

	return &ServiceAccountAuthenticator{
		credBytes: credBytes,
		userEmail: userEmail,
	}, nil
}

// Service creates a Gmail API service using service account credentials.
// The service account must have domain-wide delegation enabled and the necessary scopes authorized.
func (a *ServiceAccountAuthenticator) Service(ctx context.Context) (*gmail.Service, error) {
	config, err := google.JWTConfigFromJSON(
		a.credBytes,
		gmail.GmailModifyScope,
		gmail.GmailSettingsBasicScope,
		gmail.GmailLabelsScope,
		tasks.TasksScope,
		calendar.CalendarScope,
	)
	if err != nil {
		return nil, fmt.Errorf("parsing service account credentials: %w", err)
	}

	// Set the user to impersonate
	config.Subject = a.userEmail

	// Create the Gmail service with the service account credentials
	return gmail.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx)))
}

// TasksService creates a Google Tasks API service using service account credentials.
// The service account must have domain-wide delegation enabled and the necessary scopes authorized.
func (a *ServiceAccountAuthenticator) TasksService(ctx context.Context) (*tasks.Service, error) {
	config, err := google.JWTConfigFromJSON(
		a.credBytes,
		gmail.GmailModifyScope,
		gmail.GmailSettingsBasicScope,
		gmail.GmailLabelsScope,
		tasks.TasksScope,
		calendar.CalendarScope,
	)
	if err != nil {
		return nil, fmt.Errorf("parsing service account credentials: %w", err)
	}

	// Set the user to impersonate
	config.Subject = a.userEmail

	// Create the Tasks service with the service account credentials
	return tasks.NewService(ctx, option.WithTokenSource(config.TokenSource(ctx)))
}

// IsServiceAccount detects if the credentials JSON is for a service account.
func IsServiceAccount(credentials io.Reader) (bool, error) {
	credBytes, err := io.ReadAll(credentials)
	if err != nil {
		return false, fmt.Errorf("reading credentials: %w", err)
	}

	var credType struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(credBytes, &credType); err != nil {
		return false, fmt.Errorf("parsing credentials: %w", err)
	}

	return credType.Type == "service_account", nil
}
