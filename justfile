# justfile for gmailcli

# Default recipe - show available commands
default:
    @just --list

# Build gmailcli binary
build:
    go build -o gmailcli ./cmd/gmailcli

# Build with version info
build-version VERSION:
    go build -ldflags "-X main.version={{VERSION}}" -o gmailcli ./cmd/gmailcli

# Install gmailcli to GOPATH/bin
install:
    go install ./cmd/gmailcli

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
    rm -f gmailcli

# Run gmailcli with arguments
run *ARGS:
    go run ./cmd/gmailcli {{ARGS}}

# Build and run gmailcli
build-run *ARGS: build
    ./gmailcli {{ARGS}}

# Show version
version:
    @go run ./cmd/gmailcli version

# Run configure
configure:
    go run ./cmd/gmailcli configure

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
