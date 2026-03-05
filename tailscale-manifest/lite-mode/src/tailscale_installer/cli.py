"""Command-line interface for Tailscale installer."""

import argparse
import sys

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
  %(prog)s --authkey tskey-1234567890 --cluster-name my-cluster --login-server https://login.example.com
  %(prog)s --authkey tskey-1234567890 --cluster-name my-cluster --context my-cluster-context -v
  %(prog)s --authkey tskey-1234567890 --cluster-name my-cluster --force
        """,
    )

    parser.add_argument(
        "--authkey",
        required=True,
        help="Tailscale auth key (required)",
    )
    parser.add_argument(
        "--cluster-name",
        required=True,
        help="Cluster name for identification (required)",
    )
    parser.add_argument(
        "--login-server",
        help="Tailscale login server URL (optional)",
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


def main() -> int:
    """Main entry point.

    Returns:
        Exit code (0 for success, non-zero for failure)
    """
    parser = create_parser()
    args = parser.parse_args()

    try:
        if args.uninstall:
            # Uninstall mode
            if not args.context:
                print("Error: --context is required for uninstall")
                return 1

            uninstaller = TailscaleUninstaller(
                context=args.context,
                verbose=args.verbose,
            )
            uninstaller.uninstall()
        else:
            # Install mode
            config = InstallerConfig(
                auth_key=args.authkey,
                cluster_name=args.cluster_name,
                login_server=args.login_server,
                context=args.context,
                verbose=args.verbose,
                force=args.force,
            )

            installer = TailscaleInstaller(config)
            installer.install()

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
