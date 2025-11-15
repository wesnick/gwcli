# START HERE: gwcli + gmailctl Integration Guide

Welcome! This directory contains everything you need to integrate gwcli with gmailctl.

## What This Integration Does

**Before:**
- gwcli uses custom config format at `~/.cmdg/cmdg.conf`
- Package name: `pkg/cmdg`
- Labels managed via gwcli commands (create/delete)
- No integration with declarative tools

**After:**
- gwcli uses gmailctl-compatible auth: `~/.config/gwcli/credentials.json` + `token.json`
- Package name: `pkg/gwcli` (cleaner naming)
- Labels read from Jsonnet config (gmailctl-compatible)
- Can share label definitions with gmailctl
- No label create/delete in gwcli (use gmailctl for that)

## Quick Overview

```
┌─────────────────────────────────────────────────────────────┐
│                         gmailctl                             │
│  (Declarative filter & label management via Jsonnet)        │
│                                                              │
│  ~/.gmailctl/config.jsonnet                                 │
│  {                                                           │
│    version: 'v1alpha3',                                     │
│    labels: [...],                                           │
│    rules: [...]  ← Filters, auto-applied                   │
│  }                                                           │
└─────────────────────────────────────────────────────────────┘
                              ↓
                    Shares label definitions
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                          gwcli                               │
│    (CLI tool for reading, searching, sending emails)        │
│                                                              │
│  ~/.config/gwcli/                                           │
│    ├── credentials.json  ← OAuth client from Google        │
│    ├── token.json        ← Auto-generated                  │
│    └── config.jsonnet    ← Optional: label definitions     │
│                             (or symlink to gmailctl's)      │
│                                                              │
│  Commands:                                                   │
│    gwcli messages list                                      │
│    gwcli messages read <id>                                 │
│    gwcli labels list       ← Reads from config             │
│    gwcli labels apply      ← Validates against config      │
└─────────────────────────────────────────────────────────────┘
```

## Document Guide

### 1. **README.md** (Read First)
   - Overview of what's in this directory
   - Quick orientation

### 2. **IMPLEMENTATION_PLAN.md** (Main Guide)
   - **START HERE for implementation**
   - Complete step-by-step instructions
   - Organized into 8 phases
   - Includes a checklist at the end
   - ~1000 lines of detailed guidance

### 3. **QUICK_REFERENCE.md** (During Implementation)
   - Code snippets for common changes
   - Before/after comparisons
   - Testing commands
   - Troubleshooting tips

### 4. **DEPENDENCIES.md**
   - What to add to go.mod
   - Dependency management

### 5. **source/** (Vendor These Files)
   - `auth.go` - OAuth2 authentication (105 lines)
   - `config.go` - Config types (162 lines)
   - `read.go` - Jsonnet parser (133 lines)
   - Total: ~400 lines to vendor

### 6. **LICENSE**
   - gmailctl's MIT License
   - Required attribution

## Implementation Workflow

```bash
# 1. Read the plan
cat docs/gmailctl-integration/IMPLEMENTATION_PLAN.md

# 2. Start implementation
# Follow phases 1-8 in IMPLEMENTATION_PLAN.md

# 3. Reference as needed
# Keep QUICK_REFERENCE.md open for code snippets

# 4. Track progress
# Use checklist at end of IMPLEMENTATION_PLAN.md

# 5. Test
# Verify each phase before moving to next
```

## Time Estimate

- **Reading documentation:** 30 minutes
- **Phase 1 (Vendor code):** 30 minutes
- **Phase 2 (Rename package):** 15 minutes
- **Phase 3 (Auth system):** 45 minutes
- **Phase 4 (Label loading):** 30 minutes
- **Phase 5 (Update commands):** 20 minutes
- **Phase 6 (Config paths):** 10 minutes
- **Phase 7 (Documentation):** 15 minutes
- **Phase 8 (Testing):** 30 minutes

**Total: ~3.5 hours** (including breaks and testing)

## What You'll Need

1. **Development environment**
   - Go 1.24+
   - Text editor
   - Git

2. **Google Cloud Console access**
   - Create OAuth 2.0 Client ID
   - Download credentials.json

3. **Gmail account**
   - For testing

4. **Optional: gmailctl installed**
   - To use shared label config
   - `go install github.com/mbrt/gmailctl/cmd/gmailctl@latest`

## Breaking Changes

⚠️ **This is a complete rewrite of authentication and config**

- Old config `~/.cmdg/cmdg.conf` will not work
- All users need to re-authorize OAuth
- Label create/delete commands removed

**Good news:** Only you use this tool, so no migration complexity!

## After Implementation

### Set Up gwcli

```bash
# 1. Download OAuth credentials from Google Cloud Console
# Save to ~/.config/gwcli/credentials.json

# 2. Authorize
gwcli configure

# 3. Use it!
gwcli messages list
gwcli labels list
```

### Optional: Set Up gmailctl Integration

```bash
# 1. Install gmailctl
go install github.com/mbrt/gmailctl/cmd/gmailctl@latest

# 2. Initialize
gmailctl init

# 3. Share config with gwcli
ln -s ~/.gmailctl/config.jsonnet ~/.config/gwcli/config.jsonnet

# 4. Manage labels via gmailctl
gmailctl edit

# 5. gwcli automatically picks up the labels!
gwcli labels list
```

## Benefits of This Integration

### For Label Management

**Before (gwcli only):**
```bash
gwcli labels create "work"
gwcli labels create "personal"
gwcli labels create "receipts"
# Imperative, manual, error-prone
```

**After (with gmailctl):**
```jsonnet
// config.jsonnet
{
  version: 'v1alpha3',
  labels: [
    { name: 'work' },
    { name: 'personal' },
    { name: 'receipts' },
    { name: 'projects/gwcli' },
    { name: 'projects/other' },
  ],
  rules: [
    // Automatic filter rules!
    {
      filter: { from: 'boss@company.com' },
      actions: { labels: ['work'] },
    },
  ],
}
```

```bash
gmailctl apply
# Declarative, versioned, reviewable
# Plus automatic filters!
```

### For Message Operations

```bash
# gwcli remains the best tool for CLI message operations
gwcli messages search "subject:invoice"
gwcli messages read <id> | grep "Total:"
gwcli messages list --label work | jq '.[].subject'

# Label operations work with gmailctl-defined labels
gwcli labels apply work --message <id>
```

### Best of Both Worlds

- **gmailctl:** Declarative label + filter management
- **gwcli:** Fast CLI message operations and scripting

Both share the same label definitions!

## Support

### If You Get Stuck

1. Check QUICK_REFERENCE.md → "Common Issues" section
2. Verify all import paths updated
3. Run `go mod tidy`
4. Check file permissions (credentials should be 0600)
5. Test each phase independently

### Rollback Plan

Before starting:
```bash
git checkout -b backup-before-integration
git commit -am "Backup before gmailctl integration"
```

If something goes wrong:
```bash
git checkout main
git reset --hard backup-before-integration
```

## Next Steps

**Ready to begin?**

1. ✅ Read IMPLEMENTATION_PLAN.md (all the way through)
2. ✅ Open QUICK_REFERENCE.md in another window
3. ✅ Start with Phase 1: Vendor gmailctl Code
4. ✅ Work through phases sequentially
5. ✅ Check off items in the checklist
6. ✅ Test after each phase

**Let's build this! 🚀**

---

## Files Overview

```
docs/gmailctl-integration/
│
├── START_HERE.md              ← You are here!
├── README.md                  ← Quick overview
├── IMPLEMENTATION_PLAN.md     ← Main implementation guide ⭐
├── QUICK_REFERENCE.md         ← Code snippets & troubleshooting
├── DEPENDENCIES.md            ← go.mod changes
├── LICENSE                    ← gmailctl MIT license
│
└── source/                    ← Files to vendor
    ├── auth.go
    ├── config.go
    └── read.go
```

**Total documentation:** ~1,900 lines
**Total vendored code:** ~400 lines
**Total reading time:** ~45 minutes
**Total implementation time:** ~3.5 hours

Good luck! 🎯
