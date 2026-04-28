"""Configuration data classes for the installer."""

import shlex
from dataclasses import dataclass
from typing import Optional


@dataclass
class InstallerConfig:
    """Configuration for Tailscale installer.

    Attributes:
        auth_key: Tailscale authentication key (required)
        cluster_name: Cluster name for identification (required)
        login_server: Headscale login server URL (required for install)
        headscale_api_key: Headscale API key used for duplicate-node checks
        extra_args: Extra arguments for Tailscale beyond login server (optional)
        context: Kubernetes cluster context (optional, uses current if not specified)
        verbose: Enable verbose/debug output
        force: Force installation even if resources exist
    """

    auth_key: str
    cluster_name: str
    login_server: Optional[str] = None
    headscale_api_key: Optional[str] = None
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
        if not self.effective_login_server:
            raise ValueError("login_server is required")
        if not self.headscale_api_key:
            raise ValueError("headscale_api_key is required")

    @property
    def ts_hostname(self) -> str:
        """Generate Tailscale hostname from cluster name."""
        return f"{self.cluster_name}-tsgateway"

    def _extra_args_tokens(self) -> list[str]:
        """Split extra args into shell-style tokens."""

        if not self.extra_args:
            return []

        try:
            return shlex.split(self.extra_args)
        except ValueError as exc:
            raise ValueError(f"extra_args is invalid: {exc}") from exc

    def _extract_login_server_from_extra_args(self) -> tuple[Optional[str], list[str]]:
        """Extract a legacy --login-server flag from extra_args.

        Returns the parsed login server and the remaining extra args.
        """

        login_server = None
        remaining_tokens = []
        tokens = self._extra_args_tokens()
        index = 0

        while index < len(tokens):
            token = tokens[index]
            if token == "--login-server":
                if index + 1 >= len(tokens):
                    raise ValueError("--login-server in extra_args requires a value")
                login_server = tokens[index + 1]
                index += 2
                continue

            if token.startswith("--login-server="):
                _, value = token.split("=", 1)
                if not value:
                    raise ValueError("--login-server in extra_args requires a value")
                login_server = value
                index += 1
                continue

            remaining_tokens.append(token)
            index += 1

        return login_server, remaining_tokens

    @property
    def effective_login_server(self) -> Optional[str]:
        """Return the login server from the explicit flag or legacy extra args."""

        legacy_login_server, _ = self._extract_login_server_from_extra_args()
        return self.login_server or legacy_login_server

    @property
    def ts_extra_args(self) -> str:
        """Generate TS_EXTRA_ARGS value from explicit and legacy config."""

        tokens = []
        login_server = self.effective_login_server
        if login_server:
            tokens.extend(["--login-server", login_server])

        _, remaining_tokens = self._extract_login_server_from_extra_args()
        tokens.extend(remaining_tokens)
        return shlex.join(tokens)
