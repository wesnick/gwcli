package gwcli

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc makes it easy to stub HTTP responses in tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetTokenInfoHandlesStringExpiresIn(t *testing.T) {
	const tokenInfoJSON = `{
		"email": "user@example.com",
		"email_verified": true,
		"expires_in": "3599",
		"scope": "scope.one scope.two",
		"user_id": "1234567890",
		"aud": "client-id.apps.googleusercontent.com",
		"issued_to": "client-id.apps.googleusercontent.com",
		"app_name": "gwcli-dev"
	}`

	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if want, got := "https://oauth2.googleapis.com/tokeninfo", req.URL.String(); want != got {
				t.Fatalf("unexpected URL: want %q, got %q", want, got)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(tokenInfoJSON)),
			}, nil
		}),
	}

	conn := &CmdG{authedClient: client}

	info, err := conn.GetTokenInfo(context.Background())
	if err != nil {
		t.Fatalf("GetTokenInfo() error = %v", err)
	}
	if info == nil {
		t.Fatalf("GetTokenInfo() returned nil info")
	}
	if info.ExpiresIn != 3599 {
		t.Fatalf("ExpiresIn = %d, want 3599", info.ExpiresIn)
	}
	if got, want := info.Scopes, []string{"scope.one", "scope.two"}; len(got) != len(want) {
		t.Fatalf("Scopes length = %d, want %d. Scopes = %v", len(got), len(want), got)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Scopes[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	}
}
