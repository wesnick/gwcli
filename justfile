# justfile for gwcli

# Default recipe - show available commands
default:
    @just --list

# Build gwcli binary
build:
    mkdir -p dist
    go build -o dist/gwcli .

# Build with version info
build-version VERSION:
    mkdir -p dist
    go build -ldflags "-X main.version={{VERSION}}" -o dist/gwcli .

# Install gwcli to GOPATH/bin
install:
    go install .

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
    go run . {{ARGS}}

# Build and run gwcli
build-run *ARGS: build
    ./dist/gwcli {{ARGS}}

# Show version
version:
    @go run . version

# Run configure
configure:
    go run . configure

# Gmailctl helpers
gmailctl-download *ARGS:
    go run . gmailctl download {{ARGS}}

gmailctl-apply *ARGS:
    go run . gmailctl apply {{ARGS}}

gmailctl-diff *ARGS:
    go run . gmailctl diff {{ARGS}}

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
