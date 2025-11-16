# justfile for gwcli

# Default recipe - show available commands
default:
    @just --list

# Build gwcli binary
build:
    mkdir -p dist
    go build -o dist/gwcli ./cmd/gwcli

# Build with version info
build-version VERSION:
    mkdir -p dist
    go build -ldflags "-X main.version={{VERSION}}" -o dist/gwcli ./cmd/gwcli

# Install gwcli to GOPATH/bin
install:
    go install ./cmd/gwcli

# Run tests
test:
    go test ./...

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Run go vet
vet:
    go vet ./...

# Run all checks (vet + test)
check: vet test

# Clean built binaries
clean:
    rm -rf dist/

# Run gwcli with arguments
run *ARGS:
    go run ./cmd/gwcli {{ARGS}}

# Build and run gwcli
build-run *ARGS: build
    ./dist/gwcli {{ARGS}}

# Show version
version:
    @go run ./cmd/gwcli version

# Run configure
configure:
    go run ./cmd/gwcli configure

# Gmailctl helpers
gmailctl-download *ARGS:
    go run ./cmd/gwcli gmailctl download {{ARGS}}

gmailctl-apply *ARGS:
    go run ./cmd/gwcli gmailctl apply {{ARGS}}

gmailctl-diff *ARGS:
    go run ./cmd/gwcli gmailctl diff {{ARGS}}

# Format Go code
fmt:
    go fmt ./...

# Tidy dependencies
tidy:
    go mod tidy

# Update dependencies
update:
    go get -u ./...
    go mod tidy
