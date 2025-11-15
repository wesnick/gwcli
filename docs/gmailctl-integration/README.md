# gmailctl Integration Documentation

This directory contains all the documentation and source files needed to integrate gwcli with gmailctl.

## Contents

- **IMPLEMENTATION_PLAN.md** - Complete step-by-step implementation guide
- **LICENSE** - gmailctl's MIT License
- **source/** - Source files to vendor from gmailctl
  - `auth.go` - OAuth2 authentication logic
  - `config.go` - Config type definitions
  - `read.go` - Jsonnet config parser

## Quick Start

1. Read `IMPLEMENTATION_PLAN.md` from start to finish
2. Follow the phases in order
3. Use the checklist at the end to track progress
4. Refer to source files in `source/` directory when vendoring code

## Overview

This integration will:

- ✅ Use gmailctl-compatible authentication (credentials.json + token.json)
- ✅ Read labels from Jsonnet config files
- ✅ Store config in `~/.config/gwcli/`
- ✅ Rename `pkg/cmdg` to `pkg/gwcli`
- ✅ Remove label create/delete commands (use gmailctl instead)
- ✅ Keep label apply/remove commands (with config validation)

## Implementation Time

Estimated: 2-3 hours for complete implementation and testing

## License

The vendored code from gmailctl is licensed under the MIT License.
See `LICENSE` file for full text.

When vendoring, ensure you add proper attribution headers to all files.

## Questions?

See the "Questions or Issues" section at the end of IMPLEMENTATION_PLAN.md
