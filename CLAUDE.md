# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

tag-finder is a fast terminal UI tool written in Go that finds which Docker image tags are associated with a specific digest using the Docker Registry API v2. It uses concurrent HTTP requests and the Bubble Tea framework for an interactive terminal interface.

## Build and Test Commands

### Building
```bash
# Build the binary
go build -o tag-finder tag-finder.go

# Build with version information (for releases)
go build -ldflags "-X main.version=v1.0.0" -o tag-finder tag-finder.go
```

### Testing
```bash
# Run all tests
go test -v ./...

# Run tests with race detection
go test -v -race ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

# View coverage report in browser
go tool cover -html=coverage.out
```

### Linting
```bash
# Run golangci-lint (v2.7.2 as configured in CI)
golangci-lint run --timeout=5m

# Auto-fix issues where possible
golangci-lint run --fix --timeout=5m

# Check formatting
gofmt -l .
```

### Running the Tool
```bash
# Basic usage (interactive mode)
./tag-finder <image> <digest>

# With custom worker count
./tag-finder -workers 20 ghcr.io/example/image sha256:abc123...

# Plain mode - piped output (automatic TTY detection)
./tag-finder nginx sha256:abc123... | grep latest

# Plain mode - quiet (suppress stderr progress messages)
./tag-finder -quiet nginx sha256:abc123... > tags.txt

# Check version
./tag-finder --version
```

## Code Architecture

### Single-File Structure
The entire application is contained in `tag-finder.go` (with tests in `tag-finder_test.go`). This is intentional for simplicity and ease of distribution.

### Key Components

**RegistryClient** - HTTP client for Docker Registry API v2
- Manages bearer token authentication with caching (`token` field + `tokenMutex`)
- Implements worker pool pattern for concurrent manifest fetches
- Handles automatic pagination for registries with 1000+ tags
- Default timeout: 30 seconds, configurable connection pooling

**Output Mode Detection** - Automatic TTY detection
- Uses `github.com/mattn/go-isatty` to detect if stdout is a TTY
- Branches to `runTUIMode()` for interactive terminals or `runPlainMode()` for pipes/redirects
- Plain mode outputs only tags to stdout, progress to stderr (unless `-quiet` flag is used)
- Exit codes: 0 for matches found, 1 for no matches or errors

**Bubble Tea Model** - Terminal UI state management (TUI mode only)
- `model` struct contains all UI state (progress, spinner, tags, results)
- Uses channels (`resultsChan`) to receive results from concurrent workers
- Context-based cancellation (`ctx`, `cancel`) for clean shutdown on Ctrl+C/q
- Message types: `tagsMsg` (tag list fetched), `checkMsg` (digest comparison result)

**Plain Mode Functions** - Simple text output (plain mode only)
- `runPlainMode()` orchestrates plain mode execution with context cancellation
- `checkDigestsPlain()` polls results channel and outputs matches to stdout
- `setupSignalHandler()` enables graceful Ctrl+C cancellation via context
- Progress throttled to every 100 tags to avoid stderr spam

**Authentication Flow**
1. Initial request returns 401 with `WWW-Authenticate` header
2. Parse header to extract realm, service, scope
3. Request anonymous bearer token from auth endpoint
4. Cache token in `RegistryClient` for reuse across requests
5. Token is shared across all worker goroutines via mutex-protected field

**Worker Pool Pattern** (shared by both TUI and plain modes)
1. Create buffered jobs channel with all tags
2. Spawn N worker goroutines (default 10, configurable via `-workers`)
3. Each worker fetches manifest digest via `fetchManifestDigest()`
4. Results sent to buffered `resultsChan` (capacity: workers*2)
5. TUI mode: Main UI loop receives results via `waitForNextResult()` and updates progress
6. Plain mode: `checkDigestsPlain()` polls channel and outputs matches to stdout
7. Context cancellation stops all workers on user interrupt (Ctrl+C)

**Pagination Support**
- Fetches tags in pages of 1000 via `fetchTagsPage()`
- Parses `Link` header for next page URL
- Loops until no more pages (`parseLinkHeader()` returns empty string)
- Accumulates all tags before starting digest comparison

### Registry Parsing
`parseImageReference()` handles multiple registry formats:
- Docker Hub: `nginx` → `https://registry-1.docker.io/library/nginx`
- Docker Hub explicit: `docker.io/myorg/image` → `https://registry-1.docker.io/myorg/image`
- GHCR: `ghcr.io/user/repo` → `https://ghcr.io/user/repo`
- Quay: `quay.io/org/image` → `https://quay.io/org/image`
- Custom: `registry.example.com/image` → `https://registry.example.com/image`

Note: Docker Hub requires `library/` prefix for official images (e.g., `nginx` → `library/nginx`)

### Manifest Digest Fetching
- Uses `Docker-Content-Digest` header from manifest endpoint
- Accepts multiple manifest types via `Accept` header (Docker v2, OCI v1, manifest lists)
- Does NOT download manifest body - only reads digest from response headers
- This is why it's much faster than tools that download full manifests

## Important Implementation Details

### Concurrency Considerations
- All HTTP requests use `context.Background()` (workers use model's `ctx` for cancellation)
- Token cache (`rc.token`) protected by `tokenMutex` - multiple goroutines share auth
- Results channel is buffered to prevent workers from blocking
- Workers check `ctx.Done()` before each request for graceful shutdown

### Error Handling
- HTTP errors during manifest fetch are sent via `TagInfo.Err` but don't stop the scan
- Authentication failures bubble up and stop execution (can't proceed without auth)
- 401 responses trigger automatic token acquisition and retry

### Testing Patterns
Tests in `tag-finder_test.go` use:
- Table-driven tests for parsing logic
- Mock HTTP servers for registry API testing
- Race detector enabled in CI (`-race` flag)
- Plain mode tests verify correct output and exit codes
  - `TestCheckDigestsPlain` - Verifies matching tags are found and counted correctly
  - `TestCheckDigestsPlainNoMatches` - Verifies behavior when no tags match
  - `TestCheckDigestsPlainCancellation` - Verifies context cancellation works in plain mode

## CI/CD

### CI Workflow (Pull Requests)
Located in `.github/workflows/ci.yml`:
1. Run golangci-lint v2.7.2 with 5-minute timeout
2. Run tests with race detection and coverage
3. Build binary and verify it runs
4. Upload coverage to Codecov

### Linting Configuration
`.golangci.yml` (v2 schema) enables:
- Core linters: errcheck, govet, ineffassign, staticcheck, unused
- Additional: misspell, revive, unconvert, unparam, bodyclose, goconst, noctx, rowserrcheck
- Formatters: gofmt, goimports

## Dependencies

Uses Bubble Tea ecosystem for terminal UI:
- `github.com/charmbracelet/bubbletea` - TUI framework (Elm architecture)
- `github.com/charmbracelet/bubbles` - Progress bar and spinner components
- `github.com/charmbracelet/lipgloss` - Terminal styling

No external dependencies for Docker Registry API - uses standard library `net/http` and `encoding/json`.

## Version Information

The `version` variable at the top of `tag-finder.go` is set to "dev" by default and overridden during release builds via `-ldflags "-X main.version=vX.Y.Z"`.
