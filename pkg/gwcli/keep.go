package gwcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"golang.org/x/oauth2"
	keep "google.golang.org/api/keep/v1"
	"google.golang.org/api/option"
)

// ErrKeepRequiresServiceAccount is returned when Keep is used with anything
// other than a service-account key. Google exposes the Keep API only to
// Google Workspace (enterprise) domains via domain-wide delegation; there is
// no consent-screen scope a consumer @gmail.com account can grant.
var ErrKeepRequiresServiceAccount = errors.New(`Google Keep requires a service account with domain-wide delegation

The Keep API is enterprise-only: Google does not expose it to regular
(consumer) OAuth accounts, so 'gwcli configure' cannot grant access.

To use Keep:
1. Enable the Google Keep API in your Google Cloud project
2. Create a service account and enable domain-wide delegation for it
3. In the Workspace Admin console (Security > API controls > Domain-wide
   delegation), authorize the service account's numeric Client ID for
   https://www.googleapis.com/auth/keep (and optionally
   https://www.googleapis.com/auth/keep.readonly)
4. Save the service-account key JSON as credentials.json in the gwcli
   config directory (or point --config at a directory containing it)
5. Pass --user (or set GWCLI_USER) to the Workspace user to impersonate`)

// NewKeepService builds a Google Keep client for the service account in
// configDir, impersonating userEmail via domain-wide delegation. It never
// touches token.json and does not fall back to installed-app OAuth.
//
// readOnly selects the read path: the full keep scope is tried first and,
// if domain-wide delegation rejects it, keep.readonly is tried instead, so
// read commands work whichever of the two scopes the admin authorized.
// Write callers pass readOnly=false and request only the full scope.
func NewKeepService(ctx context.Context, configDir, userEmail string, readOnly bool) (*keep.Service, error) {
	paths, err := GetConfigPaths(configDir)
	if err != nil {
		return nil, err
	}
	credBytes, err := os.ReadFile(paths.Credentials)
	if err != nil {
		return nil, fmt.Errorf("reading credentials at %s: %w\n\n%s",
			paths.Credentials, err, ErrKeepRequiresServiceAccount)
	}
	auth, err := newKeepAuthenticator(credBytes, userEmail)
	if err != nil {
		return nil, err
	}
	return auth.KeepService(ctx, readOnly)
}

// newKeepAuthenticator validates that credBytes is a service-account key
// and that an impersonation subject was supplied.
func newKeepAuthenticator(credBytes []byte, userEmail string) (*ServiceAccountAuthenticator, error) {
	var meta struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(credBytes, &meta); err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}
	if meta.Type != "service_account" {
		return nil, ErrKeepRequiresServiceAccount
	}
	if userEmail == "" {
		return nil, fmt.Errorf("Google Keep requires --user (or GWCLI_USER) to specify " +
			"which Workspace user the service account impersonates")
	}
	return &ServiceAccountAuthenticator{credBytes: credBytes, userEmail: userEmail}, nil
}

// KeepService builds a Google Keep client via domain-wide delegation.
//
// Like DriveService, only Keep scopes are requested: DWD token exchange is
// all-or-nothing across the requested scope set, so bundling the Gmail/
// Tasks/Calendar scopes would force the admin to authorize all of them just
// to use Keep.
func (a *ServiceAccountAuthenticator) KeepService(ctx context.Context, readOnly bool) (*keep.Service, error) {
	full, err := a.tokenSource(ctx, keep.KeepScope)
	if err != nil {
		return nil, err
	}
	ts := full
	if readOnly {
		ro, err := a.tokenSource(ctx, keep.KeepReadonlyScope)
		if err != nil {
			return nil, err
		}
		ts = &fallbackTokenSource{primary: full, secondary: ro}
	}
	return keep.NewService(ctx, option.WithTokenSource(oauth2.ReuseTokenSource(nil, ts)))
}

// fallbackTokenSource fetches from primary and, only when domain-wide
// delegation rejects its scope set (unauthorized_client), switches
// permanently to secondary. Any other error is returned as-is.
type fallbackTokenSource struct {
	primary, secondary oauth2.TokenSource

	mu          sync.Mutex
	useFallback bool
}

func (f *fallbackTokenSource) Token() (*oauth2.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.useFallback {
		tok, err := f.primary.Token()
		if err == nil || !IsUnauthorizedClient(err) {
			return tok, err
		}
		f.useFallback = true
	}
	return f.secondary.Token()
}

// IsUnauthorizedClient reports whether err is the token-exchange failure
// Google returns when domain-wide delegation does not authorize the
// requested scope set for the service account's Client ID.
func IsUnauthorizedClient(err error) bool {
	var re *oauth2.RetrieveError
	if !errors.As(err, &re) {
		return false
	}
	if re.ErrorCode != "" {
		return re.ErrorCode == "unauthorized_client"
	}
	// The JWT (service-account) flow does not populate ErrorCode; the
	// OAuth error code is only in the raw response body.
	var body struct {
		Error string `json:"error"`
	}
	return json.Unmarshal(re.Body, &body) == nil && body.Error == "unauthorized_client"
}
