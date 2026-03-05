"""Tests for Tailscale installer."""

import pytest
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
            login_server="https://login.example.com",
            context="test-context",
            verbose=True,
            force=True,
        )
        assert config.login_server == "https://login.example.com"
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

    def test_ts_extra_args_with_login_server(self):
        """Test TS_EXTRA_ARGS with login server."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
            login_server="https://login.example.com",
        )
        assert config.ts_extra_args == "--login-server=https://login.example.com"

    def test_ts_extra_args_without_login_server(self):
        """Test TS_EXTRA_ARGS without login server."""
        config = InstallerConfig(
            auth_key="tskey-test123",
            cluster_name="test-cluster",
        )
        assert config.ts_extra_args == ""


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
