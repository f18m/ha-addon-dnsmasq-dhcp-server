import json
import os
import sqlite3
import time

import pytest
import requests
import websocket

ADDON_HOST = os.environ.get("ADDON_HOST", "172.28.0.2")
UI_URL = f"http://{ADDON_HOST}:8976"
INGRESS_URL = f"http://{ADDON_HOST}:8100"
INGRESS_HEADERS = {"X-Ingress-Path": "/api/hassio_ingress/testtoken"}

# must be kept in sync with docker-compose.yml and options.json
CLIENTS = {
    "first-reserved": {"mac": "aa:bb:cc:dd:ee:00", "ip": "172.28.0.10"},
    "second-reserved": {"mac": "aa:bb:cc:dd:ee:01", "ip": "172.28.0.105"},
    "dynamic1": {"mac": "aa:bb:cc:dd:ee:02", "ip": None},
    "hostname4": {"mac": "aa:bb:cc:dd:ee:03", "ip": None},
}
BLOCKED_CLIENT = {"name": "blocked-guy", "mac": "de:ad:be:ef:ca:fe"}


@pytest.fixture(scope="session")
def dhcp_results():
    """Lease information collected by each DHCP client container, by client name."""
    results = {}
    for name in [*CLIENTS, BLOCKED_CLIENT["name"]]:
        with open(f"/results/{name}.json", encoding="utf-8") as f:
            results[name] = json.load(f)
    return results


def _read_ws_message():
    ws = websocket.create_connection(f"ws://{ADDON_HOST}:8976/ws", timeout=10)
    try:
        return json.loads(ws.recv())
    finally:
        ws.close()


@pytest.fixture
def ws_message():
    return _read_ws_message()


@pytest.fixture
def read_ws_message():
    return _read_ws_message


@pytest.fixture(scope="session")
def tracker_db():
    # the DB is owned by the addon container but the shared volume is readable here
    conn = sqlite3.connect("file:/addon-data/trackerdb.sqlite3?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row
    yield conn
    conn.close()


def wait_for(predicate, timeout=20, interval=0.5):
    deadline = time.time() + timeout
    while time.time() < deadline:
        value = predicate()
        if value:
            return value
        time.sleep(interval)
    return predicate()


@pytest.fixture
def http():
    s = requests.Session()
    s.headers.update(INGRESS_HEADERS)
    return s
