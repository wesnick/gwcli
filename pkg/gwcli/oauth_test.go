package gwcli

import (
	"strings"
	"testing"
)

const fakeOAuthCreds = `{
	"installed": {
		"client_id": "test-client-id.apps.googleusercontent.com",
		"client_secret": "test-secret",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://oauth2.googleapis.com/token",
		"redirect_uris": ["http://localhost"]
	}
}`

func TestNewAuthenticatorScopes(t *testing.T) {
	a, err := NewAuthenticator(strings.NewReader(fakeOAuthCreds))
	if err != nil {
		t.Fatalf("NewAuthenticator() error = %v", err)
	}

	want := map[string]bool{
		"https://www.googleapis.com/auth/gmail.modify":         false,
		"https://www.googleapis.com/auth/gmail.settings.basic": false,
		"https://www.googleapis.com/auth/gmail.labels":         false,
		"https://www.googleapis.com/auth/tasks":                false,
		"https://www.googleapis.com/auth/calendar":             false,
	}
	for _, s := range a.cfg.Scopes {
		if _, ok := want[s]; ok {
			want[s] = true
		}
	}
	for scope, seen := range want {
		if !seen {
			t.Errorf("missing scope %q, got %v", scope, a.cfg.Scopes)
		}
	}

	if a.State == "" {
		t.Error("expected a non-empty OAuth state")
	}
}

func TestIsServiceAccount(t *testing.T) {
	if ok, err := IsServiceAccount(strings.NewReader(fakeOAuthCreds)); err != nil || ok {
		t.Errorf("IsServiceAccount(installed) = (%v, %v), want (false, nil)", ok, err)
	}

	sa := `{"type":"service_account","client_email":"svc@example.iam.gserviceaccount.com"}`
	if ok, err := IsServiceAccount(strings.NewReader(sa)); err != nil || !ok {
		t.Errorf("IsServiceAccount(service_account) = (%v, %v), want (true, nil)", ok, err)
	}
}

func TestNewServiceAccountAuthenticatorRequiresUser(t *testing.T) {
	sa := `{"type":"service_account"}`
	if _, err := NewServiceAccountAuthenticator(strings.NewReader(sa), ""); err == nil {
		t.Error("expected error when impersonation user is empty")
	}
}
