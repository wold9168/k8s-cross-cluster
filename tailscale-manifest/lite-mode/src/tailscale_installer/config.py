"""Configuration data classes for the installer."""

from dataclasses import dataclass
from typing import Optional


@dataclass
class InstallerConfig:
    """Configuration for Tailscale installer.

    Attributes:
        auth_key: Tailscale authentication key (required)
        extra_args: Extra arguments for Tailscale (optional)
        cluster_name: Cluster name for identification (required)
        context: Kubernetes cluster context (optional, uses current if not specified)
        verbose: Enable verbose/debug output
        force: Force installation even if resources exist
    """

    auth_key: str
    cluster_name: str
    extra_args: Optional[str] = None
    context: Optional[str] = None
    verbose: bool = False
    force: bool = False

    def validate(self) -> None:
        """Validate required configuration fields.

        Raises:
            ValueError: If required fields are missing
        """
        if not self.auth_key:
            raise ValueError("auth_key is required")
        if not self.cluster_name:
            raise ValueError("cluster_name is required")

    @property
    def ts_hostname(self) -> str:
        """Generate Tailscale hostname from cluster name."""
        return f"{self.cluster_name}-tsgateway"

    @property
    def ts_extra_args(self) -> str:
        """Generate TS_EXTRA_ARGS value from extra_args."""
        if self.extra_args:
            return self.extra_args
        return ""
