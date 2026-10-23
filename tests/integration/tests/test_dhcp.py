from conftest import CLIENTS, UI_URL, wait_for


def test_all_clients_get_a_lease(dhcp_results):
    """Ensure allowed clients complete DHCP and retain their configured MAC identity."""
    for name, info in CLIENTS.items():
        if info["blocked"]:
            continue
        res = dhcp_results[name]
        assert res["leased"], f"{name} did not get a lease"
        assert res["mac"] == info["mac"]


def test_reserved_ips(dhcp_results):
    """Protect static reservations from regressions that assign a different address."""
    for name, info in CLIENTS.items():
        if info["ip"]:
            assert dhcp_results[name]["ip"] == info["ip"]


def test_dynamic_clients_get_ip_from_pool(dhcp_results):
    """Verify clients without reservations receive usable addresses from the DHCP pool."""
    for name in ("dynamic1", "dynamic2"):
        last = int(dhcp_results[name]["ip"].split(".")[-1])
        assert dhcp_results[name]["ip"].startswith("172.28.0.")
        assert 100 <= last <= 120


def test_dhcp_options(dhcp_results):
    """Check that clients receive the router, DNS, domain, and NTP settings they need."""
    res = dhcp_results["dynamic1"]
    assert res["router"] == "172.28.0.1"
    assert res["dns"] == "172.28.0.2"  # 0.0.0.0 means "the dnsmasq server itself"
    assert res["domain"] == "lan"
    assert res["ntp"] == "10.0.0.123"


def test_blocked_mac_gets_no_lease(dhcp_results):
    """Ensure the MAC blocklist prevents a denied device from joining the network."""
    blocked_client = next(info for info in CLIENTS.values() if info["blocked"])
    assert dhcp_results["blocked-guy"]["leased"] is False
    assert blocked_client["mac"] == dhcp_results["blocked-guy"]["mac"]


def test_lease_file_written_by_real_dnsmasq():
    """Confirm dnsmasq records successful leases and excludes blocked clients."""
    with open("/addon-data/dnsmasq.leases", encoding="utf-8") as f:
        lines = [line.split() for line in f if line.strip()]
    macs = {line[1] for line in lines}
    assert {c["mac"] for c in CLIENTS.values() if not c["blocked"]} <= macs
    assert all(c["mac"] not in macs for c in CLIENTS.values() if c["blocked"])


def test_tracker_db_has_history(tracker_db):
    """Protect persistent client history and hostname tracking from DHCP event regressions."""
    def rows():
        return {r["mac_addr"]: r for r in tracker_db.execute("SELECT * FROM dhcp_clients")}

    allowed_clients = {name: info for name, info in CLIENTS.items() if not info["blocked"]}
    found = wait_for(lambda: len(rows()) >= len(allowed_clients))
    assert found
    by_mac = rows()
    for name, info in allowed_clients.items():
        assert info["mac"] in by_mac, f"{name} missing from trackerdb"
        assert by_mac[info["mac"]]["hostname"] == name
    assert all(info["mac"] not in by_mac for info in CLIENTS.values() if info["blocked"])
