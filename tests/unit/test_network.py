"""Unit tests for network module."""

import pytest
from src.network import is_private_network, check_vpn_requirement, check_network_requirement


class TestIsPrivateNetwork:
    """Tests for is_private_network function."""

    def test_detects_class_c_private_ip(self):
        """Detects 192.168.x.x as private."""
        assert is_private_network("192.168.1.100") is True

    def test_detects_class_a_private_ip(self):
        """Detects 10.x.x.x as private."""
        assert is_private_network("10.0.0.1") is True

    def test_detects_class_b_private_ip(self):
        """Detects 172.16-31.x.x as private."""
        assert is_private_network("172.16.0.1") is True
        assert is_private_network("172.31.255.254") is True

    def test_rejects_public_ips(self):
        """Rejects public IPs as not private."""
        assert is_private_network("8.8.8.8") is False
        assert is_private_network("1.1.1.1") is False

    def test_rejects_invalid_hostname(self):
        """Returns False for non-IP strings."""
        assert is_private_network("example.com") is False
        assert is_private_network("invalid") is False

    def test_rejects_empty_string(self):
        """Returns False for empty string."""
        assert is_private_network("") is False


class TestCheckVpnRequirement:
    """Tests for check_vpn_requirement function."""

    def test_detects_vpn_requirement_when_flag_true(self):
        """Returns True when argocd_use_socks5_proxy is true."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "vars": {
                            "argocd_use_socks5_proxy": True
                        }
                    }
                }
            }
        }
        result = check_vpn_requirement(inv_data, "k3s_cluster", "testhost")
        assert result is True

    def test_returns_false_when_flag_missing(self):
        """Returns False when argocd_use_socks5_proxy not set."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "vars": {}
                    }
                }
            }
        }
        result = check_vpn_requirement(inv_data, "k3s_cluster", "testhost")
        assert result is False

    def test_returns_false_when_flag_false(self):
        """Returns False when argocd_use_socks5_proxy is false."""
        inv_data = {
            "all": {
                "children": {
                    "k3s_cluster": {
                        "vars": {
                            "argocd_use_socks5_proxy": False
                        }
                    }
                }
            }
        }
        result = check_vpn_requirement(inv_data, "k3s_cluster", "testhost")
        assert result is False

    def test_returns_false_for_invalid_inventory(self):
        """Returns False for malformed inventory data."""
        assert check_vpn_requirement({}, "group", "host") is False
        assert check_vpn_requirement({"all": {}}, "group", "host") is False
        assert check_vpn_requirement(None, "group", "host") is False


class TestCheckNetworkRequirement:
    """Tests for check_network_requirement function."""

    def test_detects_sshuttle_for_private_ip(self):
        """Returns sshuttle type and network for private IPs."""
        host_info = {
            "config": {
                "ansible_host": "192.168.90.100"
            }
        }
        network_type, network_range = check_network_requirement("testhost", host_info)

        assert network_type == "sshuttle"
        assert "192.168.90.0/24" in network_range

    def test_returns_none_for_public_ip(self):
        """Returns None for public IPs."""
        host_info = {
            "config": {
                "ansible_host": "8.8.8.8"
            }
        }
        network_type, network_range = check_network_requirement("testhost", host_info)

        assert network_type is None
        assert network_range is None

    def test_returns_none_when_ansible_host_missing(self):
        """Returns None when ansible_host not in config."""
        host_info = {
            "config": {}
        }
        network_type, network_range = check_network_requirement("testhost", host_info)

        assert network_type is None
        assert network_range is None

    def test_returns_none_for_empty_config(self):
        """Returns None when config dict is empty."""
        host_info = {}
        network_type, network_range = check_network_requirement("testhost", host_info)

        assert network_type is None
        assert network_range is None

    def test_calculates_correct_network_range(self):
        """Calculates /24 network range correctly for different IPs."""
        test_cases = [
            ("10.0.0.1", "10.0.0.0/24"),
            ("172.16.5.50", "172.16.5.0/24"),
            ("192.168.1.255", "192.168.1.0/24"),
        ]

        for ip, expected_range in test_cases:
            host_info = {"config": {"ansible_host": ip}}
            _, network_range = check_network_requirement("testhost", host_info)
            assert network_range == expected_range, f"Failed for IP {ip}"
