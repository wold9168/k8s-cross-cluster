"""Kubectl utility for running kubectl commands."""

import subprocess
import shutil
from typing import Optional, List
from dataclasses import dataclass


@dataclass
class KubectlResult:
    """Result of a kubectl command."""

    success: bool
    stdout: str
    stderr: str
    returncode: int


class Kubectl:
    """Utility class for running kubectl commands."""

    def __init__(self, context: Optional[str] = None, verbose: bool = False):
        """Initialize kubectl utility.

        Args:
            context: Kubernetes context to use
            verbose: Enable verbose output
        """
        self.context = context
        self.verbose = verbose

    def _get_base_args(self) -> List[str]:
        """Get base kubectl arguments including context if specified."""
        args = []
        if self.context:
            args.extend(["--context", self.context])
        return args

    def _log(self, message: str) -> None:
        """Log message if verbose mode is enabled."""
        if self.verbose:
            print(f"[DEBUG] {message}")

    def check_available(self) -> bool:
        """Check if kubectl is available in PATH.

        Returns:
            True if kubectl is available

        Raises:
            RuntimeError: If kubectl is not found
        """
        if not shutil.which("kubectl"):
            raise RuntimeError("kubectl is not installed or not in PATH")
        self._log("kubectl is available")
        return True

    def get_current_context(self) -> str:
        """Get current kubectl context.

        Returns:
            Current context name

        Raises:
            RuntimeError: If no current context is set
        """
        result = self._run(["config", "current-context"], capture_output=True)
        if not result.success or not result.stdout.strip():
            raise RuntimeError("No current kubectl context is set")
        context = result.stdout.strip()
        self._log(f"Current context: {context}")
        return context

    def context_exists(self, context: str) -> bool:
        """Check if a context exists.

        Args:
            context: Context name to check

        Returns:
            True if context exists
        """
        result = self._run(["config", "get-contexts", context], capture_output=True)
        exists = result.success
        if exists:
            self._log(f"Context '{context}' exists")
        return exists

    def resource_exists(self, resource: str) -> bool:
        """Check if a Kubernetes resource exists.

        Args:
            resource: Resource specifier (e.g., "serviceaccount/tailscale")

        Returns:
            True if resource exists
        """
        args = ["get", resource]
        args.extend(self._get_base_args())
        result = self._run(args, capture_output=True)
        return result.success

    def apply(self, file: str, dry_run: bool = False) -> KubectlResult:
        """Apply a manifest file.

        Args:
            file: Path to manifest file
            dry_run: If True, only show what would be applied

        Returns:
            KubectlResult with command output
        """
        args = ["apply", "-f", file]
        if dry_run:
            args.append("--dry-run=client")
        args.extend(self._get_base_args())
        return self._run(args)

    def delete(self, file: str) -> KubectlResult:
        """Delete resources from a manifest file.

        Args:
            file: Path to manifest file

        Returns:
            KubectlResult with command output
        """
        args = ["delete", "-f", file]
        args.extend(self._get_base_args())
        return self._run(args)

    def delete_labelled(self, label: str) -> KubectlResult:
        """Delete resources with a specific label.

        Args:
            label: Label selector (e.g., "name=k8s-cross-cluster")

        Returns:
            KubectlResult with command output
        """
        args = ["delete", "all", "-l", label]
        args.extend(self._get_base_args())
        return self._run(args)

    def _run(
        self, args: List[str], capture_output: bool = True
    ) -> KubectlResult:
        """Run kubectl command.

        Args:
            args: Command arguments
            capture_output: Whether to capture output

        Returns:
            KubectlResult with command output
        """
        cmd = ["kubectl"] + args
        self._log(f"Running: {' '.join(cmd)}")

        try:
            result = subprocess.run(
                cmd,
                capture_output=capture_output,
                text=True,
                check=False,
            )
            return KubectlResult(
                success=result.returncode == 0,
                stdout=result.stdout,
                stderr=result.stderr,
                returncode=result.returncode,
            )
        except Exception as e:
            return KubectlResult(
                success=False,
                stdout="",
                stderr=str(e),
                returncode=1,
            )
