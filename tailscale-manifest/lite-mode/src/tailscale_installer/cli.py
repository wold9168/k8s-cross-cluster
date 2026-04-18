"""Command-line interface for Tailscale installer."""

import argparse
import sys
from collections.abc import Sequence

from .config import InstallerConfig
from .installer import TailscaleInstaller, TailscaleUninstaller


def create_parser() -> argparse.ArgumentParser:
    """Create argument parser."""
    parser = argparse.ArgumentParser(
        prog="tailscale-install",
        description="Install Tailscale manifests to a Kubernetes cluster",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s --authkey tskey-1234567890 --cluster-name my-cluster
  %(prog)s --authkey tskey-1234567890 --cluster-name my-cluster --extra-args "--login-server https://login.example.com"
  %(prog)s --authkey tskey-1234567890 --cluster-name my-cluster --extra-args "--login-server https://login.example.com --operator admin" --context my-cluster-context -v
  %(prog)s --authkey tskey-1234567890 --cluster-name my-cluster --force
  %(prog)s --uninstall --context my-cluster-context
        """,
    )

    parser.add_argument(
        "--authkey",
        help="Tailscale auth key (required for install)",
    )
    parser.add_argument(
        "--cluster-name",
        help="Cluster name for identification (required for install)",
    )
    parser.add_argument(
        "--extra-args",
        help="Extra arguments for Tailscale, space-separated (optional, e.g., '--login-server https://login.example.com --operator admin')",
    )
    parser.add_argument(
        "--context",
        help="Kubernetes cluster context (optional, uses current if not specified)",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Enable verbose output for debugging",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="Force installation even if resources already exist",
    )
    parser.add_argument(
        "--uninstall",
        action="store_true",
        help="Uninstall Tailscale instead of installing",
    )

    return parser


def parse_args(argv: Sequence[str] | None = None) -> argparse.Namespace:
    """Parse command-line arguments."""

    return create_parser().parse_args(argv)


def _get_missing_install_args(args: argparse.Namespace) -> list[str]:
    """Return install-only arguments that are missing."""

    required_args = {
        "--authkey": args.authkey,
        "--cluster-name": args.cluster_name,
    }
    return [flag for flag, value in required_args.items() if not value]


def validate_args(args: argparse.Namespace) -> None:
    """Validate arguments based on the selected mode."""

    if args.uninstall:
        if not args.context:
            raise ValueError("--context is required for uninstall")
        return

    missing_install_args = _get_missing_install_args(args)
    if missing_install_args:
        missing_args = ", ".join(missing_install_args)
        raise ValueError(f"install requires: {missing_args}")


def build_installer_config(args: argparse.Namespace) -> InstallerConfig:
    """Build installer configuration from parsed CLI arguments."""

    return InstallerConfig(
        auth_key=args.authkey,
        cluster_name=args.cluster_name,
        extra_args=args.extra_args,
        context=args.context,
        verbose=args.verbose,
        force=args.force,
    )


def run_install(args: argparse.Namespace) -> None:
    """Run installer mode."""

    installer = TailscaleInstaller(build_installer_config(args))
    installer.install()


def run_uninstall(args: argparse.Namespace) -> None:
    """Run uninstall mode."""

    uninstaller = TailscaleUninstaller(
        context=args.context,
        verbose=args.verbose,
    )
    uninstaller.uninstall()


def main(argv: Sequence[str] | None = None) -> int:
    """Main entry point.

    Returns:
        Exit code (0 for success, non-zero for failure)
    """

    try:
        args = parse_args(argv)
        validate_args(args)

        if args.uninstall:
            run_uninstall(args)
        else:
            run_install(args)

        return 0

    except ValueError as e:
        print(f"Configuration error: {e}")
        return 1
    except RuntimeError as e:
        print(f"Runtime error: {e}")
        return 1
    except KeyboardInterrupt:
        print("\nInstallation cancelled by user.")
        return 130
    except Exception as e:
        print(f"Unexpected error: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
