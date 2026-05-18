package gwcli

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
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/tasks/v1"
)

// authScopes is the OAuth/JWT scope set shared by the Gmail, Tasks, and
// Calendar clients. The Drive scope is requested separately (see
// ServiceAccountAuthenticator.DriveService) because domain-wide-delegation
// token exchange is all-or-nothing across the requested scope set.
var authScopes = []string{
	gmail.GmailModifyScope,
	gmail.GmailSettingsBasicScope,
	gmail.GmailLabelsScope,
	tasks.TasksScope,
	calendar.CalendarScope,
}

// IsServiceAccount reports whether the credentials JSON is a service-account
// key (as opposed to an installed/desktop OAuth client).
func IsServiceAccount(credentials io.Reader) (bool, error) {
	credBytes, err := io.ReadAll(credentials)
	if err != nil {
		return false, fmt.Errorf("reading credentials: %w", err)
	}
	var meta struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(credBytes, &meta); err != nil {
		return false, fmt.Errorf("parsing credentials: %w", err)
	}
	return meta.Type == "service_account", nil
}

// Authenticator drives the installed-app OAuth flow for a regular Google
// account and builds API clients from a cached token.
type Authenticator struct {
	// State is the opaque CSRF value echoed through the consent redirect.
	State string
	cfg   *oauth2.Config
}

// NewAuthenticator builds an Authenticator from the contents of an OAuth
// client-credentials JSON file (Desktop app type).
func NewAuthenticator(credentials io.Reader) (*Authenticator, error) {
	credBytes, err := io.ReadAll(credentials)
	if err != nil {
		return nil, fmt.Errorf("reading credentials: %w", err)
	}
	cfg, err := google.ConfigFromJSON(credBytes, authScopes...)
	if err != nil {
		return nil, fmt.Errorf("creating config from credentials: %w", err)
	}
	return &Authenticator{State: randomState(), cfg: cfg}, nil
}

// AuthURL returns the consent-screen URL the user must visit to obtain an
// authorization code, configured for the given local redirect URL.
func (a *Authenticator) AuthURL(redirectURL string) string {
	a.cfg.RedirectURL = redirectURL
	return a.cfg.AuthCodeURL(a.State, oauth2.AccessTypeOffline)
}

// CacheToken exchanges an authorization code for a token and writes it as
// JSON to token.
func (a *Authenticator) CacheToken(ctx context.Context, authCode string, token io.Writer) error {
	tok, err := a.cfg.Exchange(ctx, authCode)
	if err != nil {
		return fmt.Errorf("exchanging auth code for token: %w", err)
	}
	return json.NewEncoder(token).Encode(tok)
}

// Service builds a Gmail client from a previously cached token. The same
// token source also authorizes the Tasks/Calendar/Drive clients created by
// the connection layer.
func (a *Authenticator) Service(ctx context.Context, token io.Reader) (*gmail.Service, error) {
	tok := &oauth2.Token{}
	if err := json.NewDecoder(token).Decode(tok); err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}
	return gmail.NewService(ctx, option.WithTokenSource(a.cfg.TokenSource(ctx, tok)))
}

// ServiceAccountAuthenticator builds API clients from a service-account key
// using Google Workspace domain-wide delegation to impersonate a user.
type ServiceAccountAuthenticator struct {
	credBytes []byte
	userEmail string
}

// NewServiceAccountAuthenticator builds a ServiceAccountAuthenticator from a
// service-account key JSON. userEmail is the mailbox to impersonate.
func NewServiceAccountAuthenticator(credentials io.Reader, userEmail string) (*ServiceAccountAuthenticator, error) {
	credBytes, err := io.ReadAll(credentials)
	if err != nil {
		return nil, fmt.Errorf("reading service account credentials: %w", err)
	}
	if userEmail == "" {
		return nil, fmt.Errorf("user email is required for service account authentication")
	}
	return &ServiceAccountAuthenticator{credBytes: credBytes, userEmail: userEmail}, nil
}

// tokenSource builds a domain-wide-delegation token source for the given
// scope set, impersonating the configured user.
func (a *ServiceAccountAuthenticator) tokenSource(ctx context.Context, scopes ...string) (oauth2.TokenSource, error) {
	cfg, err := google.JWTConfigFromJSON(a.credBytes, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing service account credentials: %w", err)
	}
	cfg.Subject = a.userEmail
	return cfg.TokenSource(ctx), nil
}

// Service builds a Gmail client via domain-wide delegation.
func (a *ServiceAccountAuthenticator) Service(ctx context.Context) (*gmail.Service, error) {
	ts, err := a.tokenSource(ctx, authScopes...)
	if err != nil {
		return nil, err
	}
	return gmail.NewService(ctx, option.WithTokenSource(ts))
}

// TasksService builds a Google Tasks client via domain-wide delegation.
func (a *ServiceAccountAuthenticator) TasksService(ctx context.Context) (*tasks.Service, error) {
	ts, err := a.tokenSource(ctx, authScopes...)
	if err != nil {
		return nil, err
	}
	return tasks.NewService(ctx, option.WithTokenSource(ts))
}

// CalendarService builds a Google Calendar client via domain-wide delegation.
func (a *ServiceAccountAuthenticator) CalendarService(ctx context.Context) (*calendar.Service, error) {
	ts, err := a.tokenSource(ctx, authScopes...)
	if err != nil {
		return nil, err
	}
	return calendar.NewService(ctx, option.WithTokenSource(ts))
}

// DriveService builds a Google Drive client via domain-wide delegation.
//
// Only the Drive scope is requested: DWD token exchange is all-or-nothing
// across the requested scope set, so bundling the Gmail/Tasks/Calendar scopes
// here would force DWD to authorize every one of them just to use Drive. The
// full drive scope (not readonly) is used intentionally to allow write
// operations (upload/update).
func (a *ServiceAccountAuthenticator) DriveService(ctx context.Context) (*drive.Service, error) {
	ts, err := a.tokenSource(ctx, drive.DriveScope)
	if err != nil {
		return nil, err
	}
	return drive.NewService(ctx, option.WithTokenSource(ts))
}

// randomState returns a cryptographically random OAuth state value.
func randomState() string {
	b := make([]byte, 128)
	if _, err := rand.Read(b); err != nil {
		// Secure RNG failure is unrecoverable.
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}
