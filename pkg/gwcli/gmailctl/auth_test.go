// Copyright (c) 2017 Michele Bertasi
// Licensed under the MIT License
// Vendored from github.com/mbrt/gmailctl

package gmailctl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientFromCredentialsIncludesTasksScope(t *testing.T) {
	// Sample OAuth2 credentials JSON (fake client ID/secret)
	credJSON := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token",
			"redirect_uris": ["http://localhost"]
		}
	}`

	cfg, err := clientFromCredentials(strings.NewReader(credJSON))
	if err != nil {
		t.Fatalf("clientFromCredentials() error = %v", err)
	}

	// Check that Tasks scope is included
	hasTasksScope := false
	for _, scope := range cfg.Scopes {
		if scope == "https://www.googleapis.com/auth/tasks" {
			hasTasksScope = true
			break
		}
	}

	assert.True(t, hasTasksScope, "clientFromCredentials() missing tasks scope, got scopes: %v", cfg.Scopes)
}

func TestClientFromCredentialsIncludesCalendarScope(t *testing.T) {
	credJSON := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token",
			"redirect_uris": ["http://localhost"]
		}
	}`

	cfg, err := clientFromCredentials(strings.NewReader(credJSON))
	if err != nil {
		t.Fatalf("clientFromCredentials() error = %v", err)
	}

	hasCalendarScope := false
	for _, scope := range cfg.Scopes {
		if scope == "https://www.googleapis.com/auth/calendar" {
			hasCalendarScope = true
			break
		}
	}

	if !hasCalendarScope {
		t.Errorf("clientFromCredentials() missing calendar scope, got scopes: %v", cfg.Scopes)
	}
}
