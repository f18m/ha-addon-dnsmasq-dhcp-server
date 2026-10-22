from conftest import CLIENTS, INGRESS_URL, UI_URL, wait_for


def test_index_page(http):
    r = http.get(UI_URL + "/", timeout=10)
    assert r.status_code == 200
    assert "<html" in r.text.lower()


def test_index_page_requires_ingress_header():
    import requests

    assert requests.get(UI_URL + "/", timeout=10).status_code == 400


def test_nginx_ingress_proxy(http):
    # the nginx-init script consumed the data provided by the mocked Supervisor:
    r = http.get(INGRESS_URL + "/", timeout=10)
    assert r.status_code == 200


def test_websocket_current_clients(dhcp_results, read_ws_message):
    msg = wait_for(lambda: (m := read_ws_message()) if len(read_ws_message()["current_clients"]) >= len(CLIENTS) else None)
    assert msg
    by_mac = {c["lease"]["mac_addr"]: c for c in msg["current_clients"]}
    assert len(by_mac) == len(CLIENTS)

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

    unknown = by_mac[CLIENTS["hostname4"]["mac"]]
    assert unknown["friendly_name"] == ""
    assert unknown["tags"] in (None, [])
