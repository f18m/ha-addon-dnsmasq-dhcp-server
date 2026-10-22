from conftest import BLOCKED_CLIENT, CLIENTS, UI_URL, wait_for


def test_all_clients_get_a_lease(dhcp_results):
    for name, info in CLIENTS.items():
        res = dhcp_results[name]
        assert res["leased"], f"{name} did not get a lease"
        assert res["mac"] == info["mac"]


def test_reserved_ips(dhcp_results):
    for name, info in CLIENTS.items():
        if info["ip"]:
            assert dhcp_results[name]["ip"] == info["ip"]


def test_dynamic_clients_get_ip_from_pool(dhcp_results):
    for name in ("dynamic1", "hostname4"):
        last = int(dhcp_results[name]["ip"].split(".")[-1])
        assert dhcp_results[name]["ip"].startswith("172.28.0.")
        assert 100 <= last <= 120


def test_dhcp_options(dhcp_results):
    res = dhcp_results["dynamic1"]
    assert res["router"] == "172.28.0.1"
    assert res["dns"] == "172.28.0.2"  # 0.0.0.0 means "the dnsmasq server itself"
    assert res["domain"] == "lan"
    assert res["ntp"] == "10.0.0.123"


def test_blocked_mac_gets_no_lease(dhcp_results):
    assert dhcp_results[BLOCKED_CLIENT["name"]]["leased"] is False


def test_lease_file_written_by_real_dnsmasq():
    with open("/addon-data/dnsmasq.leases", encoding="utf-8") as f:
        lines = [line.split() for line in f if line.strip()]
    macs = {line[1] for line in lines}
    assert {c["mac"] for c in CLIENTS.values()} <= macs
    assert BLOCKED_CLIENT["mac"] not in macs


def test_tracker_db_has_history(tracker_db):
    def rows():
        return {r["mac_addr"]: r for r in tracker_db.execute("SELECT * FROM dhcp_clients")}

    found = wait_for(lambda: len(rows()) >= len(CLIENTS))
    assert found
    by_mac = rows()
    for name, info in CLIENTS.items():
        assert info["mac"] in by_mac, f"{name} missing from trackerdb"
        assert by_mac[info["mac"]]["hostname"] == name
    assert BLOCKED_CLIENT["mac"] not in by_mac
