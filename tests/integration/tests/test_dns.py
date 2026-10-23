import subprocess

from conftest import ADDON_HOST


def dig(name, rtype="A"):
    """Query the addon DNS server directly and return its short-form answers."""
    out = subprocess.run(
        ["dig", "+short", "+time=2", "+tries=1", f"@{ADDON_HOST}", name, rtype],
        capture_output=True, text=True, check=True,
    ).stdout
    return out.split()


def test_custom_host_ipv4():
    """Verify configured IPv4 host records are served by the addon DNS server."""
    assert dig("test1ipv4.lan") == ["192.168.1.10"]


def test_custom_host_ipv6():
    """Verify configured IPv6 records survive DNS configuration and query handling."""
    assert dig("test2ipv6.lan", "AAAA") == ["fd00::1234"]


def test_reserved_client_name(dhcp_results):
    """Ensure reserved clients are discoverable by their configured DNS hostname."""
    assert dig("first-reserved.lan") == ["172.28.0.10"]


def test_dhcp_hostname_registered_in_dns(dhcp_results):
    """Ensure a dynamic DHCP lease immediately contributes its hostname to DNS."""
    assert dig("dynamic1.lan") == [dhcp_results["dynamic1"]["ip"]]


def test_cname_alias(dhcp_results):
    """Protect DNS alias resolution by checking both the CNAME and its target address."""
    answers = dig("my-device.lan")
    assert "dynamic1.lan." in answers
    assert dhcp_results["dynamic1"]["ip"] in answers
