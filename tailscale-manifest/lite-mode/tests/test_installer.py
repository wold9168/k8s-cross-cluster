"""Tests for Tailscale installer."""

import pytest
from unittest.mock import patch
from tailscale_installer import cli
from tailscale_installer.config import InstallerConfig
from tailscale_installer.installer import TailscaleInstaller


class TestInstallerConfig:
    """Tests for InstallerConfig."""

    def test_valid_config(self):
        """Test creating a valid config."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
        )
        assert config.auth_key == "tskey-test123"
        assert config.cluster_name == "test-cluster"

    def test_config_with_optional_fields(self):
        """Test config with optional fields."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
            extra_args="--login-server https://login.example.com --operator admin",
            context="test-context",
            verbose=True,
            force=True,
        )
        assert config.extra_args == "--login-server https://login.example.com --operator admin"
        assert config.context == "test-context"
        assert config.verbose is True
        assert config.force is True

    def test_validate_missing_auth_key(self):
        """Test validation fails with missing auth key."""
        config = InstallerConfig(
            auth_key="",
            cluster_name="test-cluster",
        )
        with pytest.raises(ValueError, match="auth_key is required"):
            config.validate()

    def test_validate_missing_cluster_name(self):
        """Test validation fails with missing cluster name."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="",
        )
        with pytest.raises(ValueError, match="cluster_name is required"):
            config.validate()

    def test_ts_hostname(self):
        """Test TS hostname generation."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="my-cluster",
        )
        assert config.ts_hostname == "my-cluster-tsgateway"

    def test_ts_extra_args_with_extra_args(self):
        """Test TS_EXTRA_ARGS with extra_args."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
            extra_args="--login-server https://login.example.com --operator admin",
        )
        assert config.ts_extra_args == "--login-server https://login.example.com --operator admin"

    def test_ts_extra_args_without_extra_args(self):
        """Test TS_EXTRA_ARGS without extra_args."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
        )
        assert config.ts_extra_args == ""


class TestCli:
    """Tests for command-line handling."""

    def test_parse_install_args(self):
        """Install mode accepts install arguments."""
        args = cli.parse_args([
            "--authkey",
            "tskey-test123",
            "--cluster-name",
            "test-cluster",
            "--context",
            "kind-test",
            "--force",
        ])

        assert args.authkey == "tskey-test123"
        assert args.cluster_name == "test-cluster"
        assert args.context == "kind-test"
        assert args.force is True
        assert args.uninstall is False

    def test_parse_uninstall_args_without_install_fields(self):
        """Uninstall mode parses without install-only arguments."""
        args = cli.parse_args([
            "--uninstall",
            "--context",
            "kind-test",
        ])

        assert args.uninstall is True
        assert args.context == "kind-test"
        assert args.authkey is None
        assert args.cluster_name is None

    def test_validate_install_requires_authkey_and_cluster_name(self):
        """Install mode requires auth key and cluster name."""
        args = cli.parse_args([])

        with pytest.raises(ValueError, match=r"install requires: --authkey, --cluster-name"):
            cli.validate_args(args)

    def test_validate_uninstall_requires_context(self):
        """Uninstall mode requires a Kubernetes context."""
        args = cli.parse_args(["--uninstall"])

        with pytest.raises(ValueError, match="--context is required for uninstall"):
            cli.validate_args(args)

    def test_build_installer_config(self):
        """Installer config is built directly from parsed args."""
        args = cli.parse_args([
            "--authkey",
            "tskey-test123",
            "--cluster-name",
            "test-cluster",
            "--extra-args",
            "--operator admin",
            "--context",
            "kind-test",
            "--verbose",
            "--force",
        ])

        config = cli.build_installer_config(args)

        assert config == InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
            extra_args="--operator admin",
            context="kind-test",
            verbose=True,
            force=True,
        )

    @patch("tailscale_installer.cli.run_install")
    def test_main_runs_install_mode(self, mock_run_install):
        """Main dispatches to install mode by default."""
        exit_code = cli.main([
            "--authkey",
            "tskey-test123",
            "--cluster-name",
            "test-cluster",
        ])

        assert exit_code == 0
        mock_run_install.assert_called_once()

    @patch("tailscale_installer.cli.run_uninstall")
    def test_main_runs_uninstall_mode(self, mock_run_uninstall):
        """Main dispatches to uninstall mode without install-only args."""
        exit_code = cli.main([
            "--uninstall",
            "--context",
            "kind-test",
        ])

        assert exit_code == 0
        mock_run_uninstall.assert_called_once()

    def test_main_returns_error_for_invalid_args(self, capsys):
        """Main reports validation errors consistently."""
        exit_code = cli.main(["--uninstall"])

        captured = capsys.readouterr()
        assert exit_code == 1
        assert "Configuration error: --context is required for uninstall" in captured.out


class TestTailscaleInstaller:
    """Tests for TailscaleInstaller."""

    def test_installer_initialization(self):
        """Test installer initializes correctly."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")
        assert installer.config == config
        assert str(installer.manifest_dir) == "."

    def test_installer_with_custom_manifest_dir(self, tmp_path):
        """Test installer with custom manifest directory."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
        )
        installer = TailscaleInstaller(config, manifest_dir=str(tmp_path))
        assert installer.manifest_dir == tmp_path


class TestGenerateExtraArgsConfigmap:
    """Tests for generate_extra_args_configmap method."""

    @pytest.fixture
    def mock_manifest_content(self):
        """Mock manifest content for extra args ConfigMap."""
        return """apiVersion: v1
kind: ConfigMap
metadata:
  name: tailscale-extra-args
  namespace: default
  labels:
    name: k8s-cross-cluster
data:
  TS_EXTRA_ARGS: ""
"""

    @pytest.fixture
    def mock_manifest_content_with_ts_hostname(self):
        """Mock manifest content with TS_HOSTNAME."""
        return """apiVersion: v1
kind: ConfigMap
metadata:
  name: tailscale-extra-args
  namespace: default
  labels:
    name: k8s-cross-cluster
data:
  TS_EXTRA_ARGS: ""
  TS_HOSTNAME: "old-hostname"
"""

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_generate_extra_args_configmap_with_extra_args(self, mock_read, mock_manifest_content):
        """Test generating ConfigMap with extra_args."""
        mock_read.return_value = mock_manifest_content
        config = InstallerConfig(
            auth_key="tskey-test",
            cluster_name="test-cluster",
            extra_args="--login-server https://login.example.com",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        result = installer.generate_extra_args_configmap()

        assert 'TS_EXTRA_ARGS: "--login-server https://login.example.com"' in result
        assert 'TS_HOSTNAME: "test-cluster-tsgateway"' in result

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_generate_extra_args_configmap_without_extra_args(self, mock_read, mock_manifest_content):
        """Test generating ConfigMap without extra_args."""
        mock_read.return_value = mock_manifest_content
        config = InstallerConfig(
            auth_key="tskey-test",
            cluster_name="test-cluster",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        result = installer.generate_extra_args_configmap()

        assert 'TS_EXTRA_ARGS: ""' in result
        assert 'TS_HOSTNAME: "test-cluster-tsgateway"' in result

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_generate_extra_args_configmap_with_multiple_extra_args(self, mock_read, mock_manifest_content):
        """Test generating ConfigMap with multiple space-separated extra_args."""
        mock_read.return_value = mock_manifest_content
        config = InstallerConfig(
            auth_key="tskey-test",
            cluster_name="test-cluster",
            extra_args="--login-server https://login.example.com --operator admin",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        result = installer.generate_extra_args_configmap()

        assert 'TS_EXTRA_ARGS: "--login-server https://login.example.com --operator admin"' in result

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_generate_extra_args_configmap_replace_existing_ts_hostname(
        self, mock_read, mock_manifest_content_with_ts_hostname
    ):
        """Test generating ConfigMap replaces existing TS_HOSTNAME."""
        mock_read.return_value = mock_manifest_content_with_ts_hostname
        config = InstallerConfig(
            auth_key="tskey-test",
            cluster_name="new-cluster",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        result = installer.generate_extra_args_configmap()

        assert 'TS_HOSTNAME: "new-cluster-tsgateway"' in result
        assert "old-hostname" not in result


class TestUpdateTsHostname:
    """Tests for _update_ts_hostname method."""

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_update_ts_hostname_with_cluster_name(self, mock_read):
        """Test _update_ts_hostname adds TS_HOSTNAME when cluster_name is set."""
        mock_read.return_value = """data:
  TS_EXTRA_ARGS: ""
"""
        config = InstallerConfig(
            auth_key="tskey-test",
            cluster_name="my-cluster",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        content = """data:
  TS_EXTRA_ARGS: ""
"""
        result = installer._update_ts_hostname(content)

        assert 'TS_HOSTNAME: "my-cluster-tsgateway"' in result

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_update_ts_hostname_without_cluster_name(self, mock_read):
        """Test _update_ts_hostname removes TS_HOSTNAME when cluster_name is empty."""
        mock_read.return_value = ""
        config = InstallerConfig(
            auth_key="tskey-test",
            cluster_name="",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        content = """data:
  TS_EXTRA_ARGS: ""
  TS_HOSTNAME: "some-hostname"
"""
        result = installer._update_ts_hostname(content)

        assert "TS_HOSTNAME" not in result

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_update_ts_hostname_replace_existing(self, mock_read):
        """Test _update_ts_hostname replaces existing TS_HOSTNAME value."""
        mock_read.return_value = ""
        config = InstallerConfig(
            auth_key="tskey-test",
            cluster_name="updated-cluster",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        content = """data:
  TS_EXTRA_ARGS: ""
  TS_HOSTNAME: "old-cluster-tsgateway"
"""
        result = installer._update_ts_hostname(content)

        assert 'TS_HOSTNAME: "updated-cluster-tsgateway"' in result
        assert "old-cluster-tsgateway" not in result


class TestGenerateAuthSecret:
    """Tests for generate_auth_secret method."""

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_generate_auth_secret(self, mock_read):
        """Test generating auth secret with auth key."""
        mock_read.return_value = """apiVersion: v1
kind: Secret
metadata:
  name: tailscale
data:
  TS_AUTHKEY: tskey-xxxxxxxxxx
"""
        config = InstallerConfig(
            auth_key="tskey-actual123",
            cluster_name="test-cluster",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        result = installer.generate_auth_secret()

        assert "tskey-actual123" in result
        assert "tskey-xxxxxxxxxx" not in result


class TestGenerateClusterNameConfigmap:
    """Tests for generate_cluster_name_configmap method."""

    @patch.object(TailscaleInstaller, "_read_manifest")
    def test_generate_cluster_name_configmap(self, mock_read):
        """Test generating cluster name ConfigMap."""
        mock_read.return_value = """apiVersion: v1
kind: ConfigMap
metadata:
  name: tailscale-cluster-name
data:
  CLUSTER_NAME: ""
"""
        config = InstallerConfig(
            auth_key="tskey-test",
            cluster_name="my-production-cluster",
        )
        installer = TailscaleInstaller(config, manifest_dir=".")

        result = installer.generate_cluster_name_configmap()

        assert 'CLUSTER_NAME: "my-production-cluster"' in result
