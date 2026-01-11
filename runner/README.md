# AgentMesh Runner

[![Release](https://img.shields.io/github/v/release/AgentsMesh/AgentsMeshRunner?style=flat-square)](https://github.com/AgentsMesh/AgentsMeshRunner/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/anthropics/agentmesh/runner?style=flat-square)](https://goreportcard.com/report/github.com/anthropics/agentmesh/runner)
[![License](https://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)](LICENSE)

AgentMesh Runner is a lightweight agent that connects to the AgentMesh server and executes AI agent tasks in isolated terminal environments.

## Features

- 🚀 **Multi-mode operation**: CLI, system service, or desktop tray
- 🔒 **Secure execution**: Isolated terminal environments for each task
- 🌐 **Cross-platform**: macOS, Linux, Windows support
- 📊 **Web console**: Built-in status monitoring and log viewer
- 🔄 **Auto-reconnect**: Resilient connection to AgentMesh server

## Installation

### macOS (Homebrew)

```bash
brew install agentsmesh/tap/agentmesh-runner
```

### macOS/Linux (Direct download)

```bash
# Download and install
curl -fsSL https://github.com/AgentsMesh/AgentsMeshRunner/releases/latest/download/agentmesh-runner_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz | tar xz
sudo mv runner /usr/local/bin/
```

### Linux (Debian/Ubuntu)

```bash
# Download the latest .deb package
wget https://github.com/AgentsMesh/AgentsMeshRunner/releases/latest/download/agentmesh-runner_amd64.deb
sudo dpkg -i agentmesh-runner_amd64.deb
```

### Linux (RHEL/CentOS/Fedora)

```bash
# Download the latest .rpm package
wget https://github.com/AgentsMesh/AgentsMeshRunner/releases/latest/download/agentmesh-runner_amd64.rpm
sudo rpm -i agentmesh-runner_amd64.rpm
```

### Windows

Download the latest `.zip` file from [Releases](https://github.com/AgentsMesh/AgentsMeshRunner/releases/latest), extract, and add to your PATH.

Or using Scoop:

```powershell
scoop bucket add agentsmesh https://github.com/AgentsMesh/scoop-bucket
scoop install agentmesh-runner
```

## Quick Start

### 1. Register the runner

Get a registration token from your AgentMesh dashboard, then:

```bash
runner register --server https://api.agentmesh.dev --token YOUR_TOKEN
```

### 2. Start the runner

**CLI mode (foreground):**

```bash
runner run
```

**Desktop mode (with system tray):**

```bash
runner desktop
```

**System service:**

```bash
# Install as service
sudo runner service install

# Start service
sudo runner service start

# Check status
runner service status
```

## Usage

```
AgentMesh Runner

Usage:
  runner <command> [options]

Commands:
  register    Register this runner with the AgentMesh server
  run         Start the runner in CLI mode
  desktop     Start runner in desktop mode with system tray
  service     Manage runner as a system service
  version     Show version information
  help        Show this help message

Use "runner <command> --help" for more information about a command.
```

## Configuration

Configuration is stored in `~/.agentmesh/config.yaml` after registration:

```yaml
server_url: https://api.agentmesh.dev
node_id: my-runner
max_concurrent_pods: 5
workspace_root: /tmp/agentmesh-workspace
default_agent: claude-code
log_level: info
```

## Web Console

When running in desktop mode, a local web console is available at:

```
http://127.0.0.1:19080
```

Features:
- Real-time status monitoring
- Active pods and uptime tracking
- Configuration viewer
- Live log streaming

## Building from Source

```bash
# Clone the repository
git clone https://github.com/anthropics/agentmesh.git
cd agentmesh/runner

# Build CLI version (no CGO required)
make build

# Build with desktop support (requires CGO)
make build-desktop

# Build for all platforms
make build-all
```

## Release

Releases are published to [AgentsMesh/AgentsMeshRunner](https://github.com/AgentsMesh/AgentsMeshRunner).

To create a new release:

```bash
# Tag a new version
git tag v1.0.0
git push origin v1.0.0
```

The GitHub Actions workflow will automatically build and publish to the release repository.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Links

- [AgentMesh](https://agentmesh.dev) - Main product website
- [Documentation](https://agentmesh.dev/docs/runner) - Full documentation
- [Releases](https://github.com/AgentsMesh/AgentsMeshRunner/releases) - Download binaries
