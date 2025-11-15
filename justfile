# justfile for gwcli

# Default recipe - show available commands
default:
    @just --list

# Build gwcli binary
build:
    go build -o gwcli ./cmd/gwcli

# Build with version info
build-version VERSION:
    go build -ldflags "-X main.version={{VERSION}}" -o gwcli ./cmd/gwcli

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
    rm -f gwcli

# Run gwcli with arguments
run *ARGS:
    go run ./cmd/gwcli {{ARGS}}

# Build and run gwcli
build-run *ARGS: build
    ./gwcli {{ARGS}}

# Show version
version:
    @go run ./cmd/gwcli version

# Run configure
configure:
    go run ./cmd/gwcli configure

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
