package gwcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/oauth2"
)

func TestNewKeepAuthenticatorRejectsOAuthCredentials(t *testing.T) {
	oauthCreds := []byte(`{"installed":{"client_id":"x","client_secret":"y"}}`)
	_, err := newKeepAuthenticator(oauthCreds, "user@example.com")
	if !errors.Is(err, ErrKeepRequiresServiceAccount) {
		t.Fatalf("err = %v, want ErrKeepRequiresServiceAccount", err)
	}
}

func TestNewKeepAuthenticatorRequiresUser(t *testing.T) {
	saCreds := []byte(`{"type":"service_account"}`)
	if _, err := newKeepAuthenticator(saCreds, ""); err == nil {
		t.Fatal("expected error when no impersonation user is given")
	}
	a, err := newKeepAuthenticator(saCreds, "user@example.com")
	if err != nil {
		t.Fatalf("newKeepAuthenticator: %v", err)
	}
	if a.userEmail != "user@example.com" {
		t.Errorf("userEmail = %q", a.userEmail)
	}
}

func TestNewKeepServiceMissingCredentials(t *testing.T) {
	_, err := NewKeepService(context.Background(), t.TempDir(), "user@example.com", true)
	if err == nil {
		t.Fatal("expected error for missing credentials.json")
	}
}

func TestNewKeepServiceOAuthCredentials(t *testing.T) {
	dir := t.TempDir()
	creds := `{"installed":{"client_id":"x","client_secret":"y"}}`
	if err := os.WriteFile(filepath.Join(dir, "credentials.json"), []byte(creds), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := NewKeepService(context.Background(), dir, "user@example.com", true)
	if !errors.Is(err, ErrKeepRequiresServiceAccount) {
		t.Fatalf("err = %v, want ErrKeepRequiresServiceAccount", err)
	}
}

func TestIsUnauthorizedClient(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"jwt body", &oauth2.RetrieveError{Body: []byte(`{"error":"unauthorized_client"}`)}, true},
		{"wrapped", fmt.Errorf("x: %w", &oauth2.RetrieveError{Body: []byte(`{"error":"unauthorized_client"}`)}), true},
		{"error code", &oauth2.RetrieveError{ErrorCode: "unauthorized_client"}, true},
		{"other code", &oauth2.RetrieveError{Body: []byte(`{"error":"invalid_grant"}`)}, false},
		{"not retrieve error", errors.New("unauthorized_client"), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		if got := IsUnauthorizedClient(c.err); got != c.want {
			t.Errorf("%s: IsUnauthorizedClient = %v, want %v", c.name, got, c.want)
		}
	}
}

type stubTokenSource struct {
	tok   *oauth2.Token
	err   error
	calls int
}

func (s *stubTokenSource) Token() (*oauth2.Token, error) {
	s.calls++
	return s.tok, s.err
}

func TestFallbackTokenSourceSwitchesOnUnauthorizedClient(t *testing.T) {
	primary := &stubTokenSource{err: &oauth2.RetrieveError{Body: []byte(`{"error":"unauthorized_client"}`)}}
	secondary := &stubTokenSource{tok: &oauth2.Token{AccessToken: "ro"}}
	fb := &fallbackTokenSource{primary: primary, secondary: secondary}

	for i := 0; i < 2; i++ {
		tok, err := fb.Token()
		if err != nil || tok.AccessToken != "ro" {
			t.Fatalf("Token() = %v, %v", tok, err)
		}
	}
	if primary.calls != 1 {
		t.Errorf("primary called %d times; fallback must stick after first rejection", primary.calls)
	}
}

func TestFallbackTokenSourceKeepsOtherErrors(t *testing.T) {
	primary := &stubTokenSource{err: errors.New("network down")}
	secondary := &stubTokenSource{tok: &oauth2.Token{AccessToken: "ro"}}
	fb := &fallbackTokenSource{primary: primary, secondary: secondary}

	if _, err := fb.Token(); err == nil || err.Error() != "network down" {
		t.Fatalf("err = %v, want primary's error", err)
	}
	if secondary.calls != 0 {
		t.Error("secondary must not be used for non-DWD errors")
	}
}

func TestFallbackTokenSourcePrefersPrimary(t *testing.T) {
	primary := &stubTokenSource{tok: &oauth2.Token{AccessToken: "full"}}
	secondary := &stubTokenSource{tok: &oauth2.Token{AccessToken: "ro"}}
	fb := &fallbackTokenSource{primary: primary, secondary: secondary}

	tok, err := fb.Token()
	if err != nil || tok.AccessToken != "full" {
		t.Fatalf("Token() = %v, %v", tok, err)
	}
}
