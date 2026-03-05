# Tailscale Manifest Installer

Python-based installer for Tailscale Kubernetes manifests, managed with `uv`.

## Quick Start

```bash
# Set up the environment
make setup

# Install Tailscale
make ARGS="--authkey tskey-xxx --cluster-name my-cluster" install

# Or use uv directly
uv run tailscale-install --authkey tskey-xxx --cluster-name my-cluster
```

## Requirements

- Python 3.10+
- `uv` package manager
- `kubectl` configured with cluster access

## Installation

### Using uv (recommended)

```bash
# Install dependencies
uv sync

# Run the installer
uv run tailscale-install --help
```

### Using Make

```bash
# Setup
make setup

# Install
make ARGS="--authkey tskey-xxx --cluster-name my-cluster" install

# Uninstall
make CONTEXT=my-context uninstall
```

## Usage

```bash
# Basic installation
uv run tailscale-install --authkey <TS_AUTHKEY> --cluster-name <CLUSTER_NAME>

# With custom login server
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --login-server https://my-login-server.example.com

# With specific context
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --context my-cluster-context

# Verbose output
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --verbose

# Force reinstall
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --force

# Uninstall
uv run tailscale-install --uninstall --context my-cluster-context
```

## Command Line Options

| Option | Description | Required |
|--------|-------------|----------|
| `--authkey` | Tailscale authentication key | Yes |
| `--cluster-name` | Cluster name for identification | Yes |
| `--login-server` | Tailscale login server URL | No |
| `--context` | Kubernetes cluster context | No (uses current) |
| `-v, --verbose` | Enable verbose output | No |
| `--force` | Force installation even if resources exist | No |
| `--uninstall` | Uninstall instead of install | No |

## Architecture

The installer is built with object-oriented design:

- **`InstallerConfig`**: Configuration data class
- **`TailscaleInstaller`**: Main installation logic
- **`TailscaleUninstaller`**: Uninstallation logic
- **`Kubectl`**: Kubectl command wrapper
- **CLI**: Command-line interface

## Development

```bash
# Install dev dependencies
make setup-dev

# Run tests
make test

# Format code
make format

# Lint code
make lint
```

## Migration from Shell Script

The Python installer (`tailscale-install`) replaces `apply-tailscale.sh` with the same interface:

```bash
# Old shell script
./apply-tailscale.sh --authkey tskey-xxx --cluster-name my-cluster

# New Python installer
uv run tailscale-install --authkey tskey-xxx --cluster-name my-cluster
```

All options and behaviors are preserved.
