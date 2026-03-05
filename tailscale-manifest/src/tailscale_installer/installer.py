"""Main installer logic for Tailscale Kubernetes manifests."""

import os
import tempfile
import shutil
from pathlib import Path
from typing import List, Tuple

from .config import InstallerConfig
from .kubectl import Kubectl, KubectlResult


class TailscaleInstaller:
    """Installer for Tailscale Kubernetes manifests.

    This class handles the complete installation lifecycle of Tailscale
    resources into a Kubernetes cluster.

    Attributes:
        config: Installer configuration
        kubectl: Kubectl utility instance
        manifest_dir: Directory containing manifest files
    """

    # Resource paths relative to manifest directory
    RBAC_FILE = "tailscale-rbac.yaml"
    AUTH_SECRET_FILE = "tailscale-auth-secret.yaml"
    EXTRA_ARGS_FILE = "tailscale-extra-args-configmap.yaml"
    CLUSTER_NAME_FILE = "tailscale-cluster-name-configmap.yaml"
    USERSPACE_PROXY_FILE = "tailscale-userspace-proxy.yaml"

    # Resource identifiers for existence checks
    EXISTING_RESOURCES = [
        "serviceaccount/tailscale",
        "clusterrole/tailscale",
        "clusterrolebinding/tailscale",
        "configmap/tailscale-extra-args",
        "configmap/tailscale-cluster-name",
        "secret/tailscale-auth",
        "deployment/tailscale",
    ]

    def __init__(self, config: InstallerConfig, manifest_dir: str = "."):
        """Initialize the installer.

        Args:
            config: Installer configuration
            manifest_dir: Directory containing manifest YAML files
        """
        self.config = config
        self.kubectl = Kubectl(context=config.context, verbose=config.verbose)
        self.manifest_dir = Path(manifest_dir)

    def _log(self, message: str) -> None:
        """Log message if verbose mode is enabled."""
        if self.config.verbose:
            print(f"[DEBUG] {message}")

    def validate_prerequisites(self) -> None:
        """Validate all prerequisites before installation.

        Raises:
            ValueError: If configuration is invalid
            RuntimeError: If kubectl is not available
        """
        self.config.validate()
        self.kubectl.check_available()

    def get_effective_context(self) -> str:
        """Get the effective Kubernetes context.

        Returns:
            The context name being used

        Raises:
            RuntimeError: If context cannot be determined
        """
        if self.config.context:
            if not self.kubectl.context_exists(self.config.context):
                raise RuntimeError(
                    f"Kubernetes context '{self.config.context}' does not exist"
                )
            self._log(f"Using specified context: {self.config.context}")
            return self.config.context
        else:
            context = self.kubectl.get_current_context()
            self._log(f"Using current context: {context}")
            return context

    def check_existing_installation(self) -> List[str]:
        """Check for existing Tailscale resources.

        Returns:
            List of existing resource identifiers
        """
        if self.config.force:
            self._log("Force installation enabled, skipping duplicate check")
            return []

        existing = []
        for resource in self.EXISTING_RESOURCES:
            if self.kubectl.resource_exists(resource):
                existing.append(resource)
                self._log(f"Found existing resource: {resource}")

        return existing

    def _create_temp_file(self, content: str) -> str:
        """Create a temporary file with given content.

        Args:
            content: File content

        Returns:
            Path to temporary file
        """
        fd, path = tempfile.mkstemp(suffix=".yaml")
        try:
            with os.fdopen(fd, "w") as f:
                f.write(content)
        except Exception:
            os.close(fd)
            raise
        self._log(f"Created temporary file: {path}")
        return path

    def _read_manifest(self, filename: str) -> str:
        """Read a manifest file.

        Args:
            filename: Name of manifest file

        Returns:
            File content
        """
        filepath = self.manifest_dir / filename
        self._log(f"Reading manifest: {filepath}")
        with open(filepath, "r") as f:
            return f.read()

    def generate_auth_secret(self) -> str:
        """Generate auth secret manifest with configured auth key.

        Returns:
            Updated manifest content
        """
        content = self._read_manifest(self.AUTH_SECRET_FILE)
        return content.replace(
            "TS_AUTHKEY: tskey-xxxxxxxxxx",
            f"TS_AUTHKEY: {self.config.auth_key}",
        )

    def generate_extra_args_configmap(self) -> str:
        """Generate extra args ConfigMap with configured values.

        Returns:
            Updated manifest content
        """
        content = self._read_manifest(self.EXTRA_ARGS_FILE)

        # Replace TS_EXTRA_ARGS
        content = content.replace(
            'TS_EXTRA_ARGS: ""',
            f'TS_EXTRA_ARGS: "{self.config.ts_extra_args}"',
        )

        # Handle TS_HOSTNAME
        if self.config.cluster_name:
            ts_hostname_line = f'  TS_HOSTNAME: "{self.config.ts_hostname}"'
            if "TS_HOSTNAME:" in content:
                # Replace existing
                import re
                content = re.sub(
                    r"TS_HOSTNAME: .*",
                    ts_hostname_line.strip(),
                    content,
                )
            else:
                # Add after TS_EXTRA_ARGS
                content = content.replace(
                    f'TS_EXTRA_ARGS: "{self.config.ts_extra_args}"',
                    f'TS_EXTRA_ARGS: "{self.config.ts_extra_args}"\n{ts_hostname_line}',
                )
        else:
            # Remove TS_HOSTNAME if cluster name not set
            import re
            content = re.sub(r"\n  TS_HOSTNAME:.*", "", content)

        return content

    def generate_cluster_name_configmap(self) -> str:
        """Generate cluster name ConfigMap with configured cluster name.

        Returns:
            Updated manifest content
        """
        content = self._read_manifest(self.CLUSTER_NAME_FILE)
        return content.replace(
            'CLUSTER_NAME: ""',
            f'CLUSTER_NAME: "{self.config.cluster_name}"',
        )

    def apply_manifest_content(self, content: str, description: str) -> bool:
        """Apply manifest content to cluster.

        Args:
            content: Manifest content
            description: Description for logging

        Returns:
            True if successful

        Raises:
            RuntimeError: If apply fails
        """
        temp_path = self._create_temp_file(content)
        try:
            result = self.kubectl.apply(temp_path)
            if not result.success:
                raise RuntimeError(f"Failed to apply {description}: {result.stderr}")
            return True
        finally:
            os.unlink(temp_path)

    def apply_static_manifest(self, filename: str) -> bool:
        """Apply a static manifest file.

        Args:
            filename: Name of manifest file

        Returns:
            True if successful

        Raises:
            RuntimeError: If apply fails
        """
        filepath = self.manifest_dir / filename
        result = self.kubectl.apply(str(filepath))
        if not result.success:
            raise RuntimeError(f"Failed to apply {filename}: {result.stderr}")
        return True

    def install(self) -> None:
        """Run the complete installation process.

        This method performs the following steps:
        1. Validate prerequisites
        2. Check for existing installation
        3. Apply RBAC resources
        4. Apply auth secret
        5. Apply userspace proxy
        6. Apply cluster name ConfigMap

        Raises:
            ValueError: If configuration is invalid
            RuntimeError: If installation fails
        """
        print("Starting Tailscale installation...")

        # Validate prerequisites
        self.validate_prerequisites()

        # Get and display context
        context = self.get_effective_context()
        print(f"Using Kubernetes context: {context}")

        # Check for existing installation
        existing = self.check_existing_installation()
        if existing:
            print("\nWARNING: Tailscale resources already exist in the cluster!")
            print("Found the following existing resources:")
            for resource in existing:
                print(f"  - {resource}")
            print("\nThis may indicate that Tailscale has already been installed.")

            if not self.config.force:
                response = input("\nDo you want to continue and update existing resources? (y/N): ")
                if response.lower() != "y":
                    print("Installation cancelled by user.")
                    return

        # Apply RBAC
        print("Applying Tailscale RBAC resources...")
        self.apply_static_manifest(self.RBAC_FILE)

        # Apply auth secret
        print("Applying Tailscale auth secret...")
        auth_secret = self.generate_auth_secret()
        self.apply_manifest_content(auth_secret, "auth secret")

        # Apply userspace proxy (includes extra args ConfigMap)
        print("Applying Tailscale userspace proxy...")
        extra_args = self.generate_extra_args_configmap()
        self.apply_manifest_content(extra_args, "extra args ConfigMap")
        self.apply_static_manifest(self.USERSPACE_PROXY_FILE)

        # Apply cluster name ConfigMap
        print(f"Applying Tailscale cluster name ConfigMap (name: {self.config.cluster_name})...")
        cluster_configmap = self.generate_cluster_name_configmap()
        self.apply_manifest_content(cluster_configmap, "cluster name ConfigMap")

        print(f"\nTailscale manifests applied successfully to context: {context}")
        print("Deployment may take a moment to become ready. You can check status with:")
        context_flag = f"--context {context} " if context else ""
        print(f"  kubectl {context_flag}get pods")


class TailscaleUninstaller:
    """Uninstaller for Tailscale Kubernetes manifests."""

    def __init__(self, context: str, verbose: bool = False):
        """Initialize the uninstaller.

        Args:
            context: Kubernetes context to use
            verbose: Enable verbose output
        """
        self.context = context
        self.verbose = verbose
        self.kubectl = Kubectl(context=context, verbose=verbose)
        self.manifest_dir = Path(".")

    def _log(self, message: str) -> None:
        """Log message if verbose mode is enabled."""
        if self.verbose:
            print(f"[DEBUG] {message}")

    def uninstall(self) -> None:
        """Run the complete uninstallation process."""
        print(f"Uninstalling Tailscale from context: {self.context}")

        files = [
            "tailscale-userspace-proxy.yaml",
            "tailscale-rbac.yaml",
            "tailscale-extra-args-configmap.yaml",
            "tailscale-auth-secret.yaml",
            "tailscale-cluster-name-configmap.yaml",
        ]

        for filename in files:
            filepath = self.manifest_dir / filename
            if filepath.exists():
                print(f"Deleting {filename}...")
                self.kubectl.delete(str(filepath))

        print("Deleting labelled resources...")
        self.kubectl.delete_labelled("name=k8s-cross-cluster")

        print("Tailscale uninstallation complete.")
