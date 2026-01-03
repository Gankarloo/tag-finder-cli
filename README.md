# Docker Tag Finder

[![CI](https://github.com/Gankarloo/tag-finder-cli/workflows/CI/badge.svg)](https://github.com/Gankarloo/tag-finder-cli/actions/workflows/ci.yml)
[![Release](https://github.com/Gankarloo/tag-finder-cli/workflows/Release/badge.svg)](https://github.com/Gankarloo/tag-finder-cli/releases/latest)
[![codecov](https://codecov.io/gh/Gankarloo/tag-finder-cli/branch/main/graph/badge.svg)](https://codecov.io/gh/Gankarloo/tag-finder-cli)

A fast terminal UI tool that finds which tags are associated with a specific Docker image digest using the Docker Registry API v2.

## Features

- 🎨 Beautiful terminal UI with spinner and progress bar
- 📜 Automatic plain text mode for piping/scripting
- 📊 Real-time progress tracking
- ✨ Shows matching tags as they're found
- 🚀 60x+ faster than CLI tools with concurrent HTTP requests
- 🔄 Automatic pagination support for registries with 1000+ tags
- 🌐 Works with Docker Hub, GitHub Container Registry, Quay.io, and custom registries
- 🔧 Configurable worker pool (default: 10 concurrent requests)
- 🚫 No external dependencies - pure Go implementation

## Installation

### Pre-built Binaries (Recommended)

Download the latest release for your platform from the [releases page](https://github.com/Gankarloo/tag-finder-cli/releases/latest):

**Linux (amd64):**
```bash
curl -L https://github.com/Gankarloo/tag-finder-cli/releases/latest/download/oci-tag-finder-VERSION-linux-amd64.tar.gz | tar xz
sudo mv oci-tag-finder /usr/local/bin/
```

**Linux (arm64):**
```bash
curl -L https://github.com/Gankarloo/tag-finder-cli/releases/latest/download/oci-tag-finder-VERSION-linux-arm64.tar.gz | tar xz
sudo mv oci-tag-finder /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -L https://github.com/Gankarloo/tag-finder-cli/releases/latest/download/oci-tag-finder-VERSION-darwin-amd64.tar.gz | tar xz
sudo mv oci-tag-finder /usr/local/bin/
```

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/Gankarloo/tag-finder-cli/releases/latest/download/oci-tag-finder-VERSION-darwin-arm64.tar.gz | tar xz
sudo mv oci-tag-finder /usr/local/bin/
```

**Windows:**
Download the `.zip` file from the releases page and extract `oci-tag-finder.exe` to a directory in your PATH.

### Build from Source

Requirements: Go 1.24 or later

```bash
# Clone the repository
git clone https://github.com/Gankarloo/tag-finder-cli.git
cd tag-finder-cli

# Download dependencies
go mod download

# Build the binary
go build -o oci-tag-finder oci-tag-finder.go

# Optionally install to /usr/local/bin
sudo mv oci-tag-finder /usr/local/bin/
```

## Usage

### Basic Usage

```bash
oci-tag-finder[flags] <image> <digest>
```

### Flags

- `-workers <N>` - Number of concurrent HTTP requests (default: 10)
- `-quiet` - Suppress progress messages (plain mode only)
- `-version` - Print version information

### Output Modes

oci-tag-finderautomatically detects the output mode based on your environment:

**Interactive Mode (TTY detected)**
- Shows full terminal UI with progress bar, spinner, and colors
- Real-time updates as tags are checked
- Best for interactive terminal usage

**Plain Mode (piped/redirected output)**
- Outputs only matching tags to stdout (one per line)
- Progress messages go to stderr by default
- Exit code 0 if matches found, 1 if no matches or error
- Perfect for scripting and automation

The tool automatically switches to plain mode when:
- Output is piped to another command (e.g., `oci-tag-finder... | grep latest`)
- Output is redirected to a file (e.g., `oci-tag-finder... > tags.txt`)
- Running in a non-interactive environment (e.g., CI/CD)

### Examples

**Interactive Mode:**
```bash
# Find tags for a GitHub Container Registry image
oci-tag-finderghcr.io/ublue-os/bluefin-dx-nvidia-open sha256:569a4c3f0ef68ae8103e85d3e0a7409f3065895f005ab189f10f57c3cc387a8d

# Find tags for an nginx image from Docker Hub
oci-tag-finderdocker.io/library/nginx sha256:abc123def456...

# Without sha256: prefix (it will be added automatically)
oci-tag-findernginx abc123def456...

# Use more workers for faster processing
oci-tag-finder-workers 20 ghcr.io/example/image sha256:abc123...

# Check version
oci-tag-finder--version
```

**Plain Mode (Scripting):**
```bash
# Save matching tags to a file
oci-tag-findernginx sha256:abc123... > matching-tags.txt

# Pipe output to other commands
oci-tag-findernginx sha256:abc123... | grep latest

# Assign to a bash variable
TAGS=$(oci-tag-findernginx sha256:abc123...)

# Get only the first matching tag
FIRST_TAG=$(oci-tag-findernginx sha256:abc123... | head -1)

# Quiet mode - suppress all progress messages
oci-tag-finder-quiet nginx sha256:abc123... > tags.txt

# Check if any tags match (using exit code)
if oci-tag-finder-quiet nginx sha256:abc123... > /dev/null; then
  echo "Tag found!"
else
  echo "No matching tags"
fi

# Count matching tags
oci-tag-finder-quiet nginx sha256:abc123... | wc -l
```

### Supported Registries

- Docker Hub (`docker.io` or just the image name)
- GitHub Container Registry (`ghcr.io`)
- Quay.io (`quay.io`)
- Any custom Docker Registry API v2 compatible registry

## How It Works

1. Connects directly to the Docker Registry API v2 endpoint
2. Fetches all available tags with automatic pagination support (handles 1000+ tags)
3. Uses a configurable worker pool to concurrently check each tag's manifest digest
4. Compares each manifest digest with the target digest
5. Displays matching tags in real-time with a progress bar and spinner
6. No external tools required - pure Go HTTP implementation with bearer token authentication

## Controls

- `q` or `Ctrl+C` - Quit the program

## Performance

With the concurrent HTTP request implementation:
- **~90 seconds** to check 1,400+ tags (10 workers)
- **60x+ faster** than sequential CLI tools
- Configurable concurrency via `-workers` flag
- Automatic pagination handles registries with thousands of tags

## Output

### Interactive Mode

When running in a terminal, the program displays:
- A spinner while working
- Current progress (X/Y tags checked)
- A progress bar showing completion percentage
- Real-time results as matching tags are found
- Final summary with all matching tags when complete

Example output:
```
✓ Scan complete!

Found 4 matching tag(s):
  • 41-20241227
  • 41
  • latest
  • stable
```

### Plain Mode

When output is piped or redirected, the program outputs:
- **stdout**: Only matching tags, one per line (perfect for piping)
- **stderr**: Progress messages (unless `-quiet` is used)
- **Exit code**: 0 if matches found, 1 if no matches or error

Example plain mode output:
```bash
$ oci-tag-findernginx sha256:abc123... 2>/dev/null
41-20241227
41
latest
stable

$ echo $?
0
```
