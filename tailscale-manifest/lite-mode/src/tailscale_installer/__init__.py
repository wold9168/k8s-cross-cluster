"""Tailscale Manifest Installer - Python implementation."""

from .installer import TailscaleInstaller
from .config import InstallerConfig

__all__ = ["TailscaleInstaller", "InstallerConfig"]
__version__ = "1.0.0"
