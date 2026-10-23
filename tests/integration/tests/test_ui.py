from conftest import CLIENTS, INGRESS_URL, UI_URL, wait_for


def test_index_page(http):
    """Verify the main web UI is served as HTML through its expected ingress headers."""
    r = http.get(UI_URL + "/", timeout=10)
    assert r.status_code == 200
    assert "<html" in r.text.lower()


def test_index_page_requires_ingress_header():
    """Ensure direct UI requests cannot bypass the required Home Assistant ingress path."""
    import requests

    assert requests.get(UI_URL + "/", timeout=10).status_code == 400


def test_nginx_ingress_proxy(http):
    """Verify nginx routes the Supervisor-configured ingress URL to the web backend."""
    # the nginx-init script consumed the data provided by the mocked Supervisor:
    r = http.get(INGRESS_URL + "/", timeout=10)
    assert r.status_code == 200


def test_websocket_current_clients(dhcp_results, read_ws_message):
    """Check live client data includes leases and metadata needed by the UI."""
    expected_clients = sum(not info["blocked"] for info in CLIENTS.values())
    msg = wait_for(lambda: (m := read_ws_message()) if len(read_ws_message()["current_clients"]) >= expected_clients else None)
    assert msg
    by_mac = {c["lease"]["mac_addr"]: c for c in msg["current_clients"]}
    assert len(by_mac) == expected_clients

    first = by_mac[CLIENTS["first-reserved"]["mac"]]
    assert first["lease"]["ip_addr"] == "172.28.0.10"
    assert first["has_static_ip"] is True
    assert first["friendly_name"] == "first-reserved"
    assert "important" in first["tags"]
    assert first["evaluated_link"] == "https://172.28.0.10/important-host-land-page"

    second = by_mac[CLIENTS["second-reserved"]["mac"]]
    assert second["has_static_ip"] is True
    assert second["is_inside_dhcp_pool"] is True

    dyn = by_mac[CLIENTS["dynamic1"]["mac"]]
    assert dyn["has_static_ip"] is False
    assert dyn["friendly_name"] == "dynamic1"
    assert dyn["lease"]["hostname"] == "dynamic1"

    unknown = by_mac[CLIENTS["dynamic2"]["mac"]]
    assert unknown["friendly_name"] == ""
    assert unknown["tags"] in (None, [])
