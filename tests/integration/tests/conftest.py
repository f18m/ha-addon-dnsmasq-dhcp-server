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
    "first-reserved": {"mac": "aa:bb:cc:dd:ee:00", "ip": "172.28.0.10", "blocked": False},
    "second-reserved": {"mac": "aa:bb:cc:dd:ee:01", "ip": "172.28.0.105", "blocked": False},
    "dynamic1": {"mac": "aa:bb:cc:dd:ee:02", "ip": None, "blocked": False},
    "dynamic2": {"mac": "aa:bb:cc:dd:ee:03", "ip": None, "blocked": False},
    "blocked-guy": {"mac": "de:ad:be:ef:ca:fe", "ip": None, "blocked": True},
}

# ---------------------------------------------------------------------------- #
#                                   Fixtures                                   #
# ---------------------------------------------------------------------------- #

@pytest.fixture(scope="session")
def dhcp_results():
    """Load per-client JSON results from the shared Compose ``results`` volume.

    Each DHCP client writes ``/results/<name>.json`` (see tests/integration/dhcp-client.sh); 
    the pytest container mounts
    the same volume read-only and this fixture loads one file for each CLIENTS key.
    """
    results = {}
    for name in CLIENTS:
        with open(f"/results/{name}.json", encoding="utf-8") as f:
            results[name] = json.load(f)
    return results


def _read_ws_message():
    """Read one current-client snapshot from the addon's live WebSocket endpoint."""
    ws = websocket.create_connection(f"ws://{ADDON_HOST}:8976/ws", timeout=10)
    try:
        return json.loads(ws.recv())
    finally:
        ws.close()


@pytest.fixture
def ws_message():
    """Provide one WebSocket snapshot for tests that need the current client state."""
    return _read_ws_message()


@pytest.fixture
def read_ws_message():
    """Provide the snapshot reader so tests can poll for asynchronously updated state."""
    return _read_ws_message


@pytest.fixture(scope="session")
def tracker_db():
    """Expose the addon's tracker database read-only for the duration of the test session."""
    # the DB is owned by the addon container but the shared volume is readable here
    conn = sqlite3.connect("file:/addon-data/trackerdb.sqlite3?mode=ro", uri=True)
    conn.row_factory = sqlite3.Row
    yield conn
    conn.close()


@pytest.fixture
def http():
    """Provide an HTTP session with the Home Assistant ingress header already set."""
    s = requests.Session()
    s.headers.update(INGRESS_HEADERS)
    return s

# ---------------------------------------------------------------------------- #
#                                    Helpers                                   #
# ---------------------------------------------------------------------------- #

def wait_for(predicate, timeout=20, interval=0.5):
    """Poll for asynchronous integration state until it appears or the timeout expires."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        value = predicate()
        if value:
            return value
        time.sleep(interval)
    return predicate()