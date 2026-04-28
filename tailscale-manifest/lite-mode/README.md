# Tailscale Manifest Installer

Python-based installer for Tailscale Kubernetes manifests, managed with `uv`.
Build orchestration uses [just](https://github.com/casey/just).

## Quick Start

```bash
# Set up the environment
just setup

# Install Tailscale (single cluster)
just install --authkey tskey-xxx --cluster-name my-cluster \
  --login-server https://headscale.example.com \
  --headscale-api-key hskey-xxx

# Or use uv directly
uv run tailscale-install --authkey tskey-xxx --cluster-name my-cluster \
  --login-server https://headscale.example.com \
  --headscale-api-key hskey-xxx
```

## Requirements

- Python 3.10+
- `uv` package manager
- `just` command runner (`brew install just` / `cargo install just` / etc.)
- `kubectl` configured with cluster access

## Installation

### Using uv (recommended)

```bash
# Install dependencies
uv sync

# Run the installer
uv run tailscale-install --help
```

### Using just

```bash
# Setup
just setup

# Install (single cluster)
just install --authkey tskey-xxx --cluster-name my-cluster \
  --login-server https://headscale.example.com \
  --headscale-api-key hskey-xxx

# Uninstall
just uninstall ctx=my-context
```

## Multi-Cluster Batch Operations

Set environment variables once, then operate on all clusters:

```bash
export TS_AUTHKEY="tskey-..."
export TS_LOGIN_SERVER="http://headscale.example.com"
export HEADSCALE_API_KEY="hskey-..."

# Install to multiple clusters (context:cluster-name pairs)
just install-all "ctx1:cluster1 ctx2:cluster2 ctx3:cluster3"

# Uninstall from multiple clusters
just uninstall-all "ctx1:cluster1 ctx2:cluster2 ctx3:cluster3"
```

## Usage

```bash
# Basic installation
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --login-server <HEADSCALE_LOGIN_SERVER> \
  --headscale-api-key <HEADSCALE_API_KEY>

# With specific context
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --login-server https://my-login-server.example.com \
  --headscale-api-key <HEADSCALE_API_KEY> \
  --context my-cluster-context

# Verbose output
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --login-server <HEADSCALE_LOGIN_SERVER> \
  --headscale-api-key <HEADSCALE_API_KEY> \
  --verbose

# Force reinstall
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --login-server <HEADSCALE_LOGIN_SERVER> \
  --headscale-api-key <HEADSCALE_API_KEY> \
  --force

# Additional Tailscale flags
uv run tailscale-install \
  --authkey <TS_AUTHKEY> \
  --cluster-name <CLUSTER_NAME> \
  --login-server <HEADSCALE_LOGIN_SERVER> \
  --headscale-api-key <HEADSCALE_API_KEY> \
  --extra-args "--operator admin --advertise-routes=10.96.0.0/12"

# Uninstall
uv run tailscale-install --uninstall --context my-cluster-context
```

## Command Line Options

| Option | Description | Required |
|--------|-------------|----------|
| `--authkey` | Tailscale authentication key | Yes (install) |
| `--cluster-name` | Cluster name for identification | Yes (install) |
| `--login-server` | Headscale login server URL | Yes (install) |
| `--headscale-api-key` | Headscale API key for duplicate checks | Yes (install) |
| `--extra-args` | Additional Tailscale flags except `--login-server` | No |
| `--context` | Kubernetes cluster context | No (uses current) |
| `-v, --verbose` | Enable verbose output | No |
| `--force` | Force installation even if resources exist | No |
| `--uninstall` | Uninstall instead of install | No |

> **注意**：`--login-server` 是 installer 的顶层参数，不要通过 `--extra-args` 传入。若直接将 `--login-server` 塞进 `--extra-args`，Python 安装器会报 `install requires: --login-server`。

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
just setup-dev

# Run tests
just test

# Format code
just format

# Lint code
just lint
```

## Migration from Shell Script

The Python installer (`tailscale-install`) replaces `apply-tailscale.sh` with the same interface:

```bash
# Old shell script
./apply-tailscale.sh --authkey tskey-xxx --cluster-name my-cluster

# New Python installer
uv run tailscale-install --authkey tskey-xxx --cluster-name my-cluster \
  --login-server https://headscale.example.com \
  --headscale-api-key hskey-xxx
```

All options and behaviors are preserved.
