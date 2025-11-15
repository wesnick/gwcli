# Quick Reference: Key Code Changes

This document provides quick reference snippets for the most important changes.

## File Locations Summary

```
OLD                              NEW
---                              ---
~/.cmdg/cmdg.conf               ~/.config/gwcli/credentials.json
                                ~/.config/gwcli/token.json
                                ~/.config/gwcli/config.jsonnet (optional)

pkg/cmdg/                       pkg/gwcli/
pkg/cmdg/configure.go           pkg/gwcli/auth.go (rewritten)
```

## Import Path Change

**Search and replace across entire codebase:**

```bash
# Find all occurrences
grep -r "github.com/wesnick/cmdg/pkg/cmdg" .

# Replace all
find . -type f -name "*.go" -exec sed -i 's|github.com/wesnick/cmdg/pkg/cmdg|github.com/wesnick/cmdg/pkg/gwcli|g' {} +
```

## OAuth Scopes Change

**Old (pkg/cmdg/connection.go):**
```go
scope = "https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/contacts https://www.googleapis.com/auth/drive.appdata"
```

**New (pkg/gwcli/connection.go):**
```go
scope = "https://www.googleapis.com/auth/gmail.modify https://www.googleapis.com/auth/gmail.settings.basic https://www.googleapis.com/auth/gmail.labels"
```

## Config Path Change

**Old (cmd/gwcli/config.go):**
```go
func getConfigPath(configFlag string) (string, error) {
    if configFlag == "" {
        configFlag = "~/.cmdg/cmdg.conf"
    }
    // ...
}
```

**New (pkg/gwcli/auth.go):**
```go
const DefaultConfigDir = "~/.config/gwcli"

func GetConfigPaths(configDir string) (*ConfigPaths, error) {
    if configDir == "" {
        configDir = DefaultConfigDir
    }
    // Returns paths to credentials.json, token.json, config.jsonnet
}
```

## Connection Creation Change

**Old (pkg/cmdg/connection.go):**
```go
func New(configPath string) (*CmdG, error) {
    conf, err := readConf(configPath)  // Reads cmdg.conf
    if err != nil {
        return nil, err
    }

    ocfg := oauth2.Config{
        ClientID:     conf.OAuth.ClientID,
        ClientSecret: conf.OAuth.ClientSecret,
        // ... manual OAuth setup
    }
    // ...
}
```

**New (pkg/gwcli/connection.go):**
```go
func New(configDir string) (*CmdG, error) {
    paths, err := GetConfigPaths(configDir)
    if err != nil {
        return nil, err
    }

    ctx := context.Background()
    gmailSvc, err := InitializeAuth(ctx, paths)  // Uses gmailctl auth
    if err != nil {
        return nil, err
    }
    // ...
}
```

## Label Loading Change

**Old:**
```go
func (c *CmdG) LoadLabels(ctx context.Context) error {
    // Always loads from Gmail API
    resp, err := c.gmail.Users.Labels.List("me").Context(ctx).Do()
    // ...
}
```

**New:**
```go
func (c *CmdG) LoadLabels(ctx context.Context) error {
    // Try config first, then API fallback
    paths, _ := GetConfigPaths("")

    // Try ~/.config/gwcli/config.jsonnet
    if err := c.LoadLabelsFromConfig(paths.Config); err == nil {
        return c.resolveLabelIDs(ctx)
    }

    // Try ~/.gmailctl/config.jsonnet
    gmailctlConfig := filepath.Join(home, ".gmailctl", "config.jsonnet")
    if err := c.LoadLabelsFromConfig(gmailctlConfig); err == nil {
        return c.resolveLabelIDs(ctx)
    }

    // Fallback to API
    return c.loadLabelsFromAPI(ctx)
}
```

## Command Removal

**Delete from cmd/gwcli/main.go:**
```go
Labels struct {
    Create struct {
        Name  string `arg:"" help:"Label name"`
        Color string `help:"Label color (hex)"`
    } `cmd:"" help:"Create a new label"`  // ❌ DELETE

    Delete struct {
        Label string `arg:"" help:"Label name or ID"`
        Force bool   `help:"Confirm deletion"`
    } `cmd:"" help:"Delete a label"`  // ❌ DELETE

    // Keep List, Apply, Remove
}
```

**Delete from cmd/gwcli/labels.go:**
- `runLabelsCreate()` function
- `runLabelsDelete()` function

**Delete from command switch:**
```go
switch ctx.Command() {
case "labels create":   // ❌ DELETE THIS CASE
case "labels delete":   // ❌ DELETE THIS CASE
}
```

## Configure Command Change

**Old behavior:**
```bash
$ gwcli configure
ClientID: <paste>
ClientSecret: <paste>
# Opens browser, saves to ~/.cmdg/cmdg.conf
```

**New behavior:**
```bash
$ gwcli configure
Configuring OAuth authentication...
Config directory: ~/.config/gwcli

credentials not found at ~/.config/gwcli/credentials.json

To set up authentication:
1. Go to https://console.developers.google.com
2. Create OAuth 2.0 Client ID (Desktop app)
3. Download credentials.json
4. Save to: ~/.config/gwcli/credentials.json
5. Run 'gwcli configure' again
```

Then after adding credentials.json:
```bash
$ gwcli configure
Go to the following link in your browser:
https://accounts.google.com/o/oauth2/auth?...

After authorizing, paste the code here: <user pastes>
Token saved to: ~/.config/gwcli/token.json
Configuration complete!
```

## Vendored Package Structure

```
pkg/gwcli/gmailctl/
├── auth.go       // OAuth2 authentication
├── config.go     // Config type definitions
└── reader.go     // Jsonnet parser

// Usage:
import "github.com/wesnick/cmdg/pkg/gwcli/gmailctl"

auth, err := gmailctl.NewAuthenticator(credFile)
config, err := gmailctl.ReadFile(configPath, "")
```

## License Headers for Vendored Files

Add to top of each vendored file:

```go
// Copyright (c) 2017 Michele Bertasi
// Licensed under the MIT License
// Vendored from github.com/mbrt/gmailctl

package gmailctl
```

## Testing Commands

```bash
# Build
go build -o gwcli ./cmd/gwcli

# Configure (first time)
./gwcli configure

# List labels (should read from config or API)
./gwcli labels list

# Apply label (should validate against config)
./gwcli labels apply work --message <message-id>

# List messages (should still work)
./gwcli messages list

# Read message (should still work)
./gwcli messages read <message-id>
```

## File Permission Check

```bash
# Config directory should be 0700 (owner only)
ls -la ~/.config/gwcli
# Should show: drwx------

# Files should be 0600
ls -la ~/.config/gwcli/credentials.json
ls -la ~/.config/gwcli/token.json
# Should show: -rw-------
```

## Example config.jsonnet

```jsonnet
{
  version: 'v1alpha3',
  labels: [
    { name: 'work' },
    { name: 'personal' },
    { name: 'receipts' },
    { name: 'projects/gwcli' },
    { name: 'projects/other' },
  ],
}
```

## Symlink gmailctl Config

If you already use gmailctl:

```bash
ln -s ~/.gmailctl/config.jsonnet ~/.config/gwcli/config.jsonnet
```

Now both tools share the same label definitions!

## Git Commands

```bash
# Rename package (preserves history)
git mv pkg/cmdg pkg/gwcli

# Commit the vendored code
git add pkg/gwcli/gmailctl/
git commit -m "vendor: Add gmailctl auth and config parsing"

# Commit the refactor
git add .
git commit -m "refactor: Integrate with gmailctl, rename pkg/cmdg to pkg/gwcli"
```

## Rollback Plan

If something goes wrong:

```bash
# Before starting, create a backup branch
git checkout -b backup-before-gmailctl-integration

# After changes, if you need to rollback:
git checkout main
git reset --hard backup-before-gmailctl-integration
```

## Common Issues

**Issue:** `cannot import "pkg/cmdg/gmailctl/internal/..."`
**Fix:** You're trying to import internal packages. Only vendor the three files specified.

**Issue:** `undefined: errors.WithDetails`
**Fix:** Replace gmailctl's custom errors with standard library `fmt.Errorf(..., %w, err)`

**Issue:** `undefined: gmail.Category`
**Fix:** In config.go, replace with `type Category string` at package level

**Issue:** Labels not showing from config
**Fix:** Check config.jsonnet syntax with: `jsonnet ~/.config/gwcli/config.jsonnet`

**Issue:** OAuth scope errors
**Fix:** Delete token.json and re-run `gwcli configure` with updated scopes
