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

## Benefits of the New System

### Before (old gwcli):
- Single monolithic config file
- Manual label creation
- No integration with other tools

### After (gmailctl integration):
- Modular config structure
- Declarative label management via Jsonnet
- Shareable config with gmailctl
- Automatic filter rules (via gmailctl)
- Version-controlled label definitions

### Example Workflow

```bash
# Define labels in config
cat > ~/.config/gwcli/config.jsonnet <<EOF
{
  version: 'v1alpha3',
  labels: [
    { name: 'work' },
    { name: 'personal' },
    { name: 'projects/gwcli' },
  ],
}
EOF

# gwcli automatically picks up these labels
gwcli labels list

# Apply labels to messages
gwcli messages search "project update" | jq -r '.[].id' | gwcli labels apply "projects/gwcli" --stdin

# Use gmailctl for advanced filter management
gmailctl edit  # Opens editor for filter rules
gmailctl apply # Applies filters to Gmail
```

## Rollback

If you need to rollback to the old system:

1. Restore old config:
   ```bash
   mv ~/.cmdg.backup ~/.cmdg
   ```

2. Checkout previous version of gwcli:
   ```bash
   git checkout <previous-commit>
   go build -o gwcli ./cmd/gwcli
   ```

## Questions?

See the full documentation in:
- `CLAUDE.md` - Developer documentation
- `docs/gmailctl-integration/IMPLEMENTATION_PLAN.md` - Implementation details
- https://github.com/mbrt/gmailctl - gmailctl documentation
