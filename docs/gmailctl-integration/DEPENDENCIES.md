# Dependencies for gmailctl Integration

## New Dependencies to Add

### Required

```bash
go get github.com/google/go-jsonnet@v0.21.0
```

This is the only new dependency required for Jsonnet config parsing.

## Existing Dependencies (Verify)

These should already be in your `go.mod`:

```go
require (
    golang.org/x/oauth2 v0.32.0
    google.golang.org/api v0.254.0
    gopkg.in/yaml.v3 v3.0.1  // For YAML frontmatter
)
```

## go.mod After Integration

Your `go.mod` should include:

```go
module github.com/wesnick/cmdg

go 1.24

require (
    github.com/alecthomas/kong v1.6.0
    github.com/JohannesKaufmann/html-to-markdown/v2 v2.2.1
    github.com/google/go-jsonnet v0.21.0  // NEW
    github.com/pkg/errors v0.9.1
    github.com/sirupsen/logrus v1.9.3
    golang.org/x/net v0.46.0
    golang.org/x/oauth2 v0.32.0
    google.golang.org/api v0.254.0
    gopkg.in/yaml.v3 v3.0.1
)
```

## Verify Dependencies

After adding the jsonnet dependency, run:

```bash
go mod tidy
go mod verify
```

## Transitive Dependencies

The jsonnet library will pull in its own dependencies automatically.
You don't need to manually add them.

## Minimal Vendoring Alternative

If you want to avoid the jsonnet dependency (~2MB), you could:

1. Only vendor the config types (`config.go`)
2. Skip Jsonnet parsing
3. Accept labels from Gmail API only

However, this defeats the purpose of gmailctl integration.

**Recommendation:** Accept the jsonnet dependency - it's a one-time cost and enables true integration.
