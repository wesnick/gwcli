package gwcli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wesnick/gwcli/pkg/gwcli/gmailctl"
	"google.golang.org/api/gmail/v1"
)

const (
	// DefaultConfigDir is the default location for gwcli configuration
	DefaultConfigDir = "~/.config/gwcli"

	credentialsFile = "credentials.json"
	tokenFile       = "token.json"
	configFile      = "config.jsonnet"
)

// ConfigPaths holds paths to all config files
type ConfigPaths struct {
	Dir         string
	Credentials string
	Token       string
	Config      string
}

// GetConfigPaths returns the config paths, expanding ~ if needed
func GetConfigPaths(configDir string) (*ConfigPaths, error) {
	if configDir == "" {
		configDir = DefaultConfigDir
	}

	// Expand ~
	if len(configDir) > 0 && configDir[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		configDir = filepath.Join(home, configDir[1:])
	}

	return &ConfigPaths{
		Dir:         configDir,
		Credentials: filepath.Join(configDir, credentialsFile),
		Token:       filepath.Join(configDir, tokenFile),
		Config:      filepath.Join(configDir, configFile),
	}, nil
}

// InitializeAuth creates the authenticator and service.
// userEmail is only required when using service account authentication.
func InitializeAuth(ctx context.Context, paths *ConfigPaths, userEmail string) (*gmail.Service, error) {
	// Check if credentials exist
	credFile, err := os.Open(paths.Credentials)
	if err != nil {
		return nil, fmt.Errorf(`credentials not found at %s

To set up authentication:

For OAuth (regular Gmail accounts):
1. Go to https://console.developers.google.com
2. Create a new project (or select existing)
3. Enable Gmail API
4. Create OAuth 2.0 Client ID (Desktop app)
5. Download the credentials JSON file
6. Save it to: %s
7. Run 'gwcli configure' to authorize

For Service Accounts (Google Workspace):
1. Go to https://console.developers.google.com
2. Create a new project (or select existing)
3. Enable Gmail API
4. Create Service Account with domain-wide delegation
5. Download the credentials JSON file
6. Save it to: %s
7. Use --user flag to specify which user to impersonate

Scopes needed:
- https://www.googleapis.com/auth/gmail.modify
- https://www.googleapis.com/auth/gmail.settings.basic
- https://www.googleapis.com/auth/gmail.labels
`, paths.Credentials, paths.Credentials, paths.Credentials)
	}
	defer credFile.Close()

	// Detect if this is a service account
	isServiceAcct, err := gmailctl.IsServiceAccount(credFile)
	if err != nil {
		return nil, fmt.Errorf("detecting credential type: %w", err)
	}

	// Reset file pointer after reading for detection
	if _, err := credFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("resetting credentials file: %w", err)
	}

	// Use service account authentication
	if isServiceAcct {
		if userEmail == "" {
			return nil, fmt.Errorf("service account authentication requires --user flag to specify which user to impersonate")
		}

		auth, err := gmailctl.NewServiceAccountAuthenticator(credFile, userEmail)
		if err != nil {
			return nil, fmt.Errorf("creating service account authenticator: %w", err)
		}

		return auth.Service(ctx)
	}

	// Use OAuth authentication
	auth, err := gmailctl.NewAuthenticator(credFile)
	if err != nil {
		return nil, fmt.Errorf("creating authenticator: %w", err)
	}

	// Try to load existing token
	tokenFile, err := os.Open(paths.Token)
	if err == nil {
		defer tokenFile.Close()
		return auth.Service(ctx, tokenFile)
	}

	// Token doesn't exist - need to authorize
	return nil, fmt.Errorf("token not found - run 'gwcli configure' to authorize")
}

// ConfigureAuth performs the OAuth flow and saves the token
func ConfigureAuth(ctx context.Context, paths *ConfigPaths, port int) error {
	// Ensure config directory exists
	if err := os.MkdirAll(paths.Dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Open credentials
	credFile, err := os.Open(paths.Credentials)
	if err != nil {
		return fmt.Errorf("credentials not found at %s - see 'gwcli configure --help'", paths.Credentials)
	}
	defer credFile.Close()

	// Create authenticator
	auth, err := gmailctl.NewAuthenticator(credFile)
	if err != nil {
		return fmt.Errorf("creating authenticator: %w", err)
	}

	// Start local OAuth server
	localAddr := fmt.Sprintf("http://localhost:%d", port)
	if port == 0 {
		localAddr = "http://localhost:8080"
	}

	authURL := auth.AuthURL(localAddr)
	fmt.Printf("\nGo to the following link in your browser:\n\n%s\n\n", authURL)
	fmt.Printf("After authorizing, paste the code here: ")

	var authCode string
	if _, err := fmt.Scanln(&authCode); err != nil {
		return fmt.Errorf("reading auth code: %w", err)
	}

	// Create token file
	tokenOut, err := os.OpenFile(paths.Token, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating token file: %w", err)
	}
	defer tokenOut.Close()

	// Exchange code for token and save
	if err := auth.CacheToken(ctx, authCode, tokenOut); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	fmt.Printf("\nToken saved to: %s\n", paths.Token)
	return nil
}
