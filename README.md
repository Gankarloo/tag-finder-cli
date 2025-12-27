# Docker Tag Finder

[![CI](https://github.com/USERNAME/skopeo-tag-finder/workflows/CI/badge.svg)](https://github.com/USERNAME/skopeo-tag-finder/actions/workflows/ci.yml)
[![Release](https://github.com/USERNAME/skopeo-tag-finder/workflows/Release/badge.svg)](https://github.com/USERNAME/skopeo-tag-finder/releases/latest)

A fast terminal UI tool that finds which tags are associated with a specific Docker image digest using the Docker Registry API v2.

## Features

- 🎨 Beautiful terminal UI with spinner and progress bar
- 📊 Real-time progress tracking
- ✨ Shows matching tags as they're found
- 🚀 60x+ faster than CLI tools with concurrent HTTP requests
- 🔄 Automatic pagination support for registries with 1000+ tags
- 🌐 Works with Docker Hub, GitHub Container Registry, Quay.io, and custom registries
- 🔧 Configurable worker pool (default: 10 concurrent requests)
- 🚫 No external dependencies - pure Go implementation

## Installation

### Pre-built Binaries (Recommended)

Download the latest release for your platform from the [releases page](https://github.com/USERNAME/skopeo-tag-finder/releases/latest):

**Linux (amd64):**
```bash
curl -L https://github.com/USERNAME/skopeo-tag-finder/releases/latest/download/tag-finder-VERSION-linux-amd64.tar.gz | tar xz
sudo mv tag-finder /usr/local/bin/
```

**Linux (arm64):**
```bash
curl -L https://github.com/USERNAME/skopeo-tag-finder/releases/latest/download/tag-finder-VERSION-linux-arm64.tar.gz | tar xz
sudo mv tag-finder /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -L https://github.com/USERNAME/skopeo-tag-finder/releases/latest/download/tag-finder-VERSION-darwin-amd64.tar.gz | tar xz
sudo mv tag-finder /usr/local/bin/
```

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/USERNAME/skopeo-tag-finder/releases/latest/download/tag-finder-VERSION-darwin-arm64.tar.gz | tar xz
sudo mv tag-finder /usr/local/bin/
```

**Windows:**
Download the `.zip` file from the releases page and extract `tag-finder.exe` to a directory in your PATH.

### Build from Source

Requirements: Go 1.24 or later

```bash
# Clone the repository
git clone https://github.com/USERNAME/skopeo-tag-finder.git
cd skopeo-tag-finder

# Download dependencies
go mod download

# Build the binary
go build -o tag-finder tag-finder.go

# Optionally install to /usr/local/bin
sudo mv tag-finder /usr/local/bin/
```

## Usage

### Basic Usage

```bash
tag-finder [flags] <image> <digest>
```

### Flags

- `-workers <N>` - Number of concurrent HTTP requests (default: 10)
- `-version` - Print version information

### Examples

```bash
# Find tags for a GitHub Container Registry image
tag-finder ghcr.io/ublue-os/bluefin-dx-nvidia-open sha256:569a4c3f0ef68ae8103e85d3e0a7409f3065895f005ab189f10f57c3cc387a8d

# Find tags for an nginx image from Docker Hub
tag-finder docker.io/library/nginx sha256:abc123def456...

# Without sha256: prefix (it will be added automatically)
tag-finder nginx abc123def456...

# Use more workers for faster processing
tag-finder -workers 20 ghcr.io/example/image sha256:abc123...

# Check version
tag-finder --version
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

The program will display:
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
