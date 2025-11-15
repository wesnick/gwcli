# gwcli + gmailctl Integration Implementation Plan

## Overview

This document provides a complete, step-by-step plan to integrate gwcli with gmailctl by:

1. **Vendoring gmailctl authentication and config parsing code** (from `./source/`)
2. **Renaming `pkg/cmdg` → `pkg/gwcli`** for better project naming
3. **Moving config location** from `~/.cmdg/` to `~/.config/gwcli/`
4. **Using gmailctl-compatible auth** (credentials.json + token.json)
5. **Reading labels from Jsonnet config** (gmailctl-compatible)
6. **Removing label create/delete** (use gmailctl for label management)

**Breaking Changes:** This is a complete rewrite of authentication and label management. No backward compatibility needed (single user).

---

## Prerequisites

- Go 1.24+
- Access to Google Cloud Console (for OAuth credentials)
- Basic understanding of OAuth2 and Gmail API

---

## Phase 1: Vendor gmailctl Code

### Step 1.1: Create Package Structure

```bash
mkdir -p pkg/gwcli/gmailctl
```

### Step 1.2: Copy Source Files

Copy the following files from `docs/gmailctl-integration/source/` to `pkg/gwcli/gmailctl/`:

**File 1: `auth.go`**
- Source: `docs/gmailctl-integration/source/auth.go`
- Destination: `pkg/gwcli/gmailctl/auth.go`
- **Modifications needed:**
  - Change package from `package api` to `package gmailctl`
  - Add MIT license header (see source/LICENSE)

**File 2: `config.go`**
- Source: `docs/gmailctl-integration/source/config.go`
- Destination: `pkg/gwcli/gmailctl/config.go`
- **Modifications needed:**
  - Change package from `package v1alpha3` to `package gmailctl`
  - Remove references to `github.com/mbrt/gmailctl/internal/engine/gmail`
  - Replace with inline type: `type Category string`
  - Add MIT license header

**File 3: `reader.go`**
- Source: `docs/gmailctl-integration/source/read.go`
- Destination: `pkg/gwcli/gmailctl/reader.go`
- **Modifications needed:**
  - Change package from `package config` to `package gmailctl`
  - Update import: `"github.com/mbrt/gmailctl/internal/engine/config/v1alpha3"` → use local Config type
  - Update references to `v1alpha3.Config` → `Config`
  - Remove `github.com/mbrt/gmailctl/internal/errors` dependency
  - Use standard `errors` package
  - Add MIT license header

### Step 1.3: Add License Headers

Add this header to all three vendored files:

```go
// Copyright (c) 2017 Michele Bertasi
// Licensed under the MIT License
// Vendored from github.com/mbrt/gmailctl
```

### Step 1.4: Update go.mod

Add required dependencies:

```bash
go get github.com/google/go-jsonnet@v0.21.0
```

Dependencies already present (verify):
- `golang.org/x/oauth2`
- `google.golang.org/api`

---

## Phase 2: Rename Package Structure

### Step 2.1: Rename pkg/cmdg to pkg/gwcli

```bash
git mv pkg/cmdg pkg/gwcli
```

This renames the entire directory and preserves git history.

### Step 2.2: Update Package Declarations

Update the package declaration in all files in `pkg/gwcli/`:

**Files to update:**
- `pkg/gwcli/connection.go`
- `pkg/gwcli/message.go`
- `pkg/gwcli/labels.go` (if exists)
- `pkg/gwcli/configure.go` (will be replaced)
- Any other `pkg/gwcli/*.go` files

**Change:**
```go
// Old
package cmdg

// New
package gwcli
```

### Step 2.3: Update All Import Paths

**Search and replace across entire codebase:**

```bash
# Find all files importing pkg/cmdg
grep -r "github.com/wesnick/cmdg/pkg/cmdg" cmd/ pkg/

# Replace in all files
find . -type f -name "*.go" -exec sed -i 's|github.com/wesnick/cmdg/pkg/cmdg|github.com/wesnick/cmdg/pkg/gwcli|g' {} +
```

**Files that will need updates:**
- `cmd/gwcli/*.go` (all command files)
- `cmd/gwcli/main.go`
- `pkg/dialog/*.go` (if imports cmdg)
- `pkg/gpg/*.go` (if imports cmdg)

**Verify:**
```bash
go build ./...
```

---

## Phase 3: Implement New Authentication System

### Step 3.1: Delete Old Configure

```bash
rm pkg/gwcli/configure.go
```

### Step 3.2: Create New auth.go

Create `pkg/gwcli/auth.go`:

```go
package gwcli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wesnick/cmdg/pkg/gwcli/gmailctl"
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

// InitializeAuth creates the authenticator and service
func InitializeAuth(ctx context.Context, paths *ConfigPaths) (*gmail.Service, error) {
	// Check if credentials exist
	credFile, err := os.Open(paths.Credentials)
	if err != nil {
		return nil, fmt.Errorf(`credentials not found at %s

To set up authentication:
1. Go to https://console.developers.google.com
2. Create a new project (or select existing)
3. Enable Gmail API
4. Create OAuth 2.0 Client ID (Desktop app)
5. Download the credentials JSON file
6. Save it to: %s
7. Run 'gwcli configure' again

Scopes needed:
- https://www.googleapis.com/auth/gmail.modify
- https://www.googleapis.com/auth/gmail.settings.basic
- https://www.googleapis.com/auth/gmail.labels
`, paths.Credentials, paths.Credentials)
	}
	defer credFile.Close()

	// Create authenticator
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
```

### Step 3.3: Update connection.go

Modify `pkg/gwcli/connection.go`:

**Changes needed:**
1. Remove `readConf()` function (reads old cmdg.conf format)
2. Update `New()` to use new auth system
3. Update OAuth scopes

**Key modifications:**

```go
// Remove old scope
const (
	// OLD - DELETE THIS:
	// scope = "https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/contacts https://www.googleapis.com/auth/drive.appdata"

	// NEW:
	scope = "https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/gmail.settings.basic https://www.googleapis.com/auth/gmail.labels"
)

// Update New() function to use new auth
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

	// ... rest of initialization
	c := &CmdG{
		gmail:        gmailSvc,
		messageCache: make(map[string]*Message),
		labelCache:   make(map[string]*Label),
	}

	return c, nil
}
```

### Step 3.4: Update cmd/gwcli/config.go

Replace the entire file with:

```go
package main

import (
	"context"
	"fmt"

	"github.com/wesnick/cmdg/pkg/gwcli"
)

// runConfigure runs the OAuth configuration flow
func runConfigure(configDir string) error {
	paths, err := gwcli.GetConfigPaths(configDir)
	if err != nil {
		return err
	}

	fmt.Printf("Configuring OAuth authentication...\n")
	fmt.Printf("Config directory: %s\n\n", paths.Dir)
	fmt.Printf("Required files:\n")
	fmt.Printf("  - %s (OAuth credentials from Google Console)\n", paths.Credentials)
	fmt.Printf("  - %s (will be auto-generated)\n\n", paths.Token)

	ctx := context.Background()
	if err := gwcli.ConfigureAuth(ctx, paths, 8080); err != nil {
		return fmt.Errorf("configuration failed: %w", err)
	}

	fmt.Printf("\nConfiguration complete!\n")
	fmt.Printf("You can now use gwcli commands.\n")
	return nil
}

// getConnection creates a gwcli connection
func getConnection(configDir string) (*gwcli.CmdG, error) {
	conn, err := gwcli.New(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	// Load labels
	ctx := context.Background()
	if err := conn.LoadLabels(ctx); err != nil {
		return nil, fmt.Errorf("failed to load labels: %w", err)
	}

	return conn, nil
}
```

---

## Phase 4: Implement Label Loading from Config

### Step 4.1: Add Label Loading to connection.go

Add to `pkg/gwcli/connection.go`:

```go
// LoadLabelsFromConfig loads labels from gmailctl Jsonnet config
func (c *CmdG) LoadLabelsFromConfig(configPath string) error {
	// Try to read config
	cfg, err := gmailctl.ReadFile(configPath, "")
	if err != nil {
		// Config doesn't exist or can't be parsed - fall back to API
		return nil
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
	paths, _ := GetConfigPaths("")
	if paths != nil {
		if err := c.LoadLabelsFromConfig(paths.Config); err == nil {
			// Successfully loaded from config
			return c.resolveLabelIDs(ctx)
		}

		// Try gmailctl location as fallback
		home, _ := os.UserHomeDir()
		gmailctlConfig := filepath.Join(home, ".gmailctl", "config.jsonnet")
		if err := c.LoadLabelsFromConfig(gmailctlConfig); err == nil {
			return c.resolveLabelIDs(ctx)
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
		return err
	}

	// Create name → ID mapping
	nameToID := make(map[string]string)
	for _, label := range resp.Labels {
		nameToID[label.Name] = label.Id
	}

	// Update our cache with actual IDs
	c.m.Lock()
	defer c.m.Unlock()

	for name, label := range c.labelCache {
		if id, ok := nameToID[name]; ok {
			label.ID = id
		}
	}

	return nil
}

// loadLabelsFromAPI loads labels directly from Gmail API (fallback)
func (c *CmdG) loadLabelsFromAPI(ctx context.Context) error {
	resp, err := c.gmail.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return err
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
```

---

## Phase 5: Update Label Commands

### Step 5.1: Remove Label Create/Delete from main.go

Edit `cmd/gwcli/main.go` and remove:

```go
Labels struct {
	// DELETE THESE:
	Create struct {
		Name  string `arg:"" help:"Label name"`
		Color string `help:"Label color (hex)"`
	} `cmd:"" help:"Create a new label"`

	Delete struct {
		Label string `arg:"" help:"Label name or ID"`
		Force bool   `help:"Confirm deletion"`
	} `cmd:"" help:"Delete a label"`

	// KEEP THESE:
	List struct {
		SystemOnly bool `help:"Show only system labels"`
		UserOnly   bool `help:"Show only user labels"`
	} `cmd:"" help:"List all labels"`

	Apply struct {
		Label   string `arg:"" help:"Label name or ID"`
		Message string `help:"Message ID"`
		Stdin   bool   `help:"Read message IDs from stdin"`
		Verbose bool   `help:"Show progress"`
	} `cmd:"" help:"Apply label to messages"`

	Remove struct {
		Label   string `arg:"" help:"Label name or ID"`
		Message string `help:"Message ID"`
		Stdin   bool   `help:"Read message IDs from stdin"`
		Verbose bool   `help:"Show progress"`
	} `cmd:"" help:"Remove label from messages"`
} `cmd:"" help:"Label operations"`
```

### Step 5.2: Update labels.go Command Routing

In `cmd/gwcli/main.go`, update the switch statement that handles label commands:

```go
case "labels list":
	return runLabelsList(ctx, conn, cli.Labels.List.SystemOnly, cli.Labels.List.UserOnly, out)

// DELETE THESE CASES:
// case "labels create":
// case "labels delete":

case "labels apply":
	return runLabelsApply(ctx, conn, cli.Labels.Apply.Label, cli.Labels.Apply.Message,
		cli.Labels.Apply.Stdin, cli.Labels.Apply.Verbose, out)

case "labels remove":
	return runLabelsRemove(ctx, conn, cli.Labels.Remove.Label, cli.Labels.Remove.Message,
		cli.Labels.Remove.Stdin, cli.Labels.Remove.Verbose, out)
```

### Step 5.3: Delete Label Create/Delete Functions

In `cmd/gwcli/labels.go`, **DELETE** these functions:
- `runLabelsCreate()`
- `runLabelsDelete()`

### Step 5.4: Add Validation to Apply/Remove

Update `runLabelsApply()` and `runLabelsRemove()` to validate against config:

```go
// Add at the start of runLabelsApply and runLabelsRemove:
func runLabelsApply(ctx context.Context, conn *gwcli.CmdG, labelID, messageID string, stdin bool, verbose bool, out *outputWriter) error {
	// Validate label exists
	labels := conn.Labels()
	found := false
	for _, l := range labels {
		if strings.EqualFold(l.Label, labelID) || l.ID == labelID {
			found = true
			break
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Warning: label '%s' not found in config\n", labelID)
		fmt.Fprintf(os.Stderr, "If you need to create labels, use gmailctl\n")
	}

	// ... rest of function
}
```

---

## Phase 6: Update Configuration Paths

### Step 6.1: Update Global Config Flag

In `cmd/gwcli/main.go`, update the global config flag:

```go
type CLI struct {
	Config string `type:"path" default:"~/.config/gwcli" help:"Config directory path"`
	JSON   bool   `help:"Output as JSON"`

	Configure struct {} `cmd:"" help:"Configure OAuth authentication"`

	Messages struct {
		// ... existing message commands
	} `cmd:"" help:"Message operations"`

	Labels struct {
		// ... updated label commands (without create/delete)
	} `cmd:"" help:"Label operations"`
}
```

### Step 6.2: Pass Config Dir to Commands

Ensure all commands receive the config directory:

```go
func main() {
	cli := &CLI{}
	ctx := kong.Parse(cli)

	// ...

	switch ctx.Command() {
	case "configure":
		err = runConfigure(cli.Config)

	case "messages list":
		conn, err := getConnection(cli.Config)
		// ...

	// All other commands use cli.Config
	}
}
```

---

## Phase 7: Update Documentation

### Step 7.1: Update CLAUDE.md

Replace the configuration section:

```markdown
## Configuration File

Location: `~/.config/gwcli/`

Required files:
- `credentials.json` - OAuth client credentials from Google Cloud Console
- `token.json` - Auto-generated OAuth access/refresh tokens

Optional files:
- `config.jsonnet` - gmailctl-compatible label definitions (Jsonnet format)

### Authentication Setup

1. Go to https://console.developers.google.com
2. Create a new project
3. Enable Gmail API
4. Create OAuth 2.0 Client ID (Desktop app)
5. Download credentials as JSON
6. Save to `~/.config/gwcli/credentials.json`
7. Run `gwcli configure`

Required OAuth scopes:
- `https://www.googleapis.com/auth/gmail.modify`
- `https://www.googleapis.com/auth/gmail.settings.basic`
- `https://www.googleapis.com/auth/gmail.labels`

### Label Management

**Label Discovery:**
gwcli reads label definitions from Jsonnet config files in this order:
1. `~/.config/gwcli/config.jsonnet`
2. `~/.gmailctl/config.jsonnet` (if gmailctl is installed)
3. Gmail API (fallback)

**Label Operations:**
- `gwcli labels list` - List all labels
- `gwcli labels apply <label> --message <id>` - Apply label to message
- `gwcli labels remove <label> --message <id>` - Remove label from message

**Creating/Deleting Labels:**
Use gmailctl for label management:
```bash
# Install gmailctl
go install github.com/mbrt/gmailctl/cmd/gmailctl@latest

# Edit label config
gmailctl edit

# See: https://github.com/mbrt/gmailctl for full documentation
```

### gmailctl Integration

gwcli is compatible with gmailctl's label definitions. If you use gmailctl
for filter management, gwcli will automatically discover your labels.

Example `~/.config/gwcli/config.jsonnet`:

```jsonnet
{
  version: 'v1alpha3',
  labels: [
    { name: 'work' },
    { name: 'personal' },
    { name: 'receipts' },
  ],
}
```

You can also symlink to gmailctl's config:
```bash
ln -s ~/.gmailctl/config.jsonnet ~/.config/gwcli/config.jsonnet
```
```

### Step 7.2: Create Migration Guide

Create `docs/gmailctl-integration/MIGRATION.md`:

```markdown
# Migration from Old gwcli

## What Changed

**Old config location:** `~/.cmdg/cmdg.conf`
**New config location:** `~/.config/gwcli/`

**New file structure:**
- `credentials.json` - OAuth client credentials
- `token.json` - OAuth tokens
- `config.jsonnet` - Optional label config

## Migration Steps

### 1. Clean Up Old Config

```bash
# Backup old config (optional)
mv ~/.cmdg ~/.cmdg.backup

# Or just delete
rm -rf ~/.cmdg
```

### 2. Set Up New Authentication

```bash
# Create config directory
mkdir -p ~/.config/gwcli

# Download credentials from Google Cloud Console
# (See CLAUDE.md for instructions)
# Save to ~/.config/gwcli/credentials.json

# Run configure to authorize
gwcli configure
```

### 3. Optional: Set Up Label Config

If you want declarative label management:

```bash
# Install gmailctl
go install github.com/mbrt/gmailctl/cmd/gmailctl@latest

# Initialize gmailctl
gmailctl init

# Symlink config to gwcli
ln -s ~/.gmailctl/config.jsonnet ~/.config/gwcli/config.jsonnet
```

## Command Changes

**Removed commands:**
- `gwcli labels create` → Use gmailctl
- `gwcli labels delete` → Use gmailctl

**Unchanged commands:**
- All message operations (list, read, send, delete, etc.)
- Label operations: list, apply, remove

## Troubleshooting

**Error: "credentials not found"**
- Download credentials.json from Google Cloud Console
- Place in `~/.config/gwcli/credentials.json`

**Error: "token not found"**
- Run `gwcli configure` to authorize

**Labels not showing up:**
- Check `~/.config/gwcli/config.jsonnet` exists
- Or labels will be loaded from Gmail API
```

---

## Phase 8: Testing

### Step 8.1: Build and Test

```bash
# Build
go build -o gwcli ./cmd/gwcli

# Test configure (without credentials - should show error)
./gwcli configure

# After adding credentials.json:
./gwcli configure

# Test label list
./gwcli labels list

# Test message operations
./gwcli messages list
```

### Step 8.2: Verify Integration

**Test checklist:**
- [ ] `gwcli configure` creates `~/.config/gwcli/token.json`
- [ ] `gwcli labels list` shows labels
- [ ] `gwcli labels apply` works with message IDs
- [ ] `gwcli messages list` works
- [ ] `gwcli messages read <id>` works
- [ ] If `config.jsonnet` exists, labels are read from it
- [ ] If `config.jsonnet` doesn't exist, labels are read from API

---

## Implementation Checklist

Use this checklist to track progress:

### Phase 1: Vendor gmailctl Code
- [ ] Create `pkg/gwcli/gmailctl/` directory
- [ ] Copy and modify `auth.go`
- [ ] Copy and modify `config.go`
- [ ] Copy and modify `reader.go`
- [ ] Add MIT license headers
- [ ] Update `go.mod` with dependencies
- [ ] Verify: `go build ./pkg/gwcli/gmailctl`

### Phase 2: Rename Package
- [ ] Run `git mv pkg/cmdg pkg/gwcli`
- [ ] Update package declarations in all `pkg/gwcli/*.go` files
- [ ] Update all import paths in `cmd/gwcli/*.go`
- [ ] Update imports in `pkg/dialog/` and `pkg/gpg/` if needed
- [ ] Verify: `go build ./...`

### Phase 3: Authentication
- [ ] Delete `pkg/gwcli/configure.go`
- [ ] Create `pkg/gwcli/auth.go`
- [ ] Update `pkg/gwcli/connection.go` (remove old auth, add new)
- [ ] Update `cmd/gwcli/config.go`
- [ ] Verify: `go build ./cmd/gwcli`

### Phase 4: Label Loading
- [ ] Add `LoadLabelsFromConfig()` to `connection.go`
- [ ] Add `resolveLabelIDs()` to `connection.go`
- [ ] Add `loadLabelsFromAPI()` to `connection.go`
- [ ] Update `LoadLabels()` to try config first
- [ ] Verify: Labels load from config or API

### Phase 5: Update Commands
- [ ] Remove `Create` and `Delete` from `Labels` struct in `main.go`
- [ ] Remove `case "labels create"` and `case "labels delete"`
- [ ] Delete `runLabelsCreate()` from `labels.go`
- [ ] Delete `runLabelsDelete()` from `labels.go`
- [ ] Add validation to `runLabelsApply()`
- [ ] Add validation to `runLabelsRemove()`
- [ ] Verify: `go build ./cmd/gwcli`

### Phase 6: Configuration Paths
- [ ] Update default config path to `~/.config/gwcli`
- [ ] Update all commands to use `cli.Config`
- [ ] Verify: Commands use new path

### Phase 7: Documentation
- [ ] Update CLAUDE.md with new config structure
- [ ] Create MIGRATION.md
- [ ] Document gmailctl integration
- [ ] Update README if needed

### Phase 8: Testing
- [ ] Build: `go build -o gwcli ./cmd/gwcli`
- [ ] Test `gwcli configure`
- [ ] Test `gwcli labels list`
- [ ] Test `gwcli labels apply`
- [ ] Test `gwcli messages list`
- [ ] Test with and without config.jsonnet
- [ ] Clean up old `~/.cmdg/` directory

---

## File Reference

### Source Files to Vendor

All source files are in `docs/gmailctl-integration/source/`:

1. **auth.go** (106 lines)
   - OAuth2 authentication
   - Service creation
   - Token caching

2. **config.go** (163 lines)
   - Config, Label, Rule types
   - Jsonnet config structure

3. **read.go** (134 lines)
   - Jsonnet parsing
   - Config file reading

### License

gmailctl is licensed under MIT License. See `docs/gmailctl-integration/LICENSE`.

When vendoring, add this header to each file:

```go
// Copyright (c) 2017 Michele Bertasi
// Licensed under the MIT License
// Vendored from github.com/mbrt/gmailctl
```

---

## Expected Final Structure

```
gwcli/
├── cmd/
│   └── gwcli/
│       ├── main.go (updated)
│       ├── config.go (rewritten)
│       ├── labels.go (create/delete removed)
│       ├── messages.go
│       ├── attachments.go
│       ├── batch.go
│       ├── output.go
│       └── format.go
├── pkg/
│   ├── gwcli/ (renamed from cmdg)
│   │   ├── connection.go (updated)
│   │   ├── message.go
│   │   ├── auth.go (new)
│   │   └── gmailctl/ (vendored)
│   │       ├── auth.go
│   │       ├── config.go
│   │       └── reader.go
│   ├── dialog/
│   └── gpg/
├── docs/
│   └── gmailctl-integration/
│       ├── IMPLEMENTATION_PLAN.md (this file)
│       ├── MIGRATION.md
│       ├── LICENSE (gmailctl MIT license)
│       └── source/
│           ├── auth.go
│           ├── config.go
│           └── read.go
├── CLAUDE.md (updated)
├── go.mod (updated with jsonnet dependency)
└── README.md

Configuration location:
~/.config/gwcli/
├── credentials.json (OAuth client from Google Console)
├── token.json (auto-generated)
└── config.jsonnet (optional, gmailctl-compatible)
```

---

## Next Steps

After completing this implementation:

1. **Test thoroughly** with your Gmail account
2. **Set up gmailctl** if you want declarative filter management
3. **Create labels** using gmailctl's Jsonnet config
4. **Use gwcli** for message operations and label application

For gmailctl setup, see: https://github.com/mbrt/gmailctl

---

## Questions or Issues

If you encounter issues during implementation:

1. Check that all imports are updated (`pkg/cmdg` → `pkg/gwcli`)
2. Verify dependencies: `go mod tidy`
3. Check file permissions on `~/.config/gwcli/` (should be 0700)
4. Verify OAuth scopes in Google Cloud Console
5. Test with `go build -v ./...` to see detailed build output

Good luck with the implementation!
