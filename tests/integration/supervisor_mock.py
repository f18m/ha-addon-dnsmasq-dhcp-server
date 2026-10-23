#!/usr/bin/env python3
"""Minimal fake of the Home Assistant Supervisor API, as used by bashio inside the addon.

Only the endpoints needed by the startup scripts are implemented; every request is logged
on stdout so that missing endpoints can be spotted easily. Unknown GET paths get an empty
successful answer, so that bashio never has to wait for a timeout.
"""
import json
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

OPTIONS_FILE = sys.argv[1] if len(sys.argv) > 1 else "/options.json"

with open(OPTIONS_FILE, encoding="utf-8") as f:
    OPTIONS = json.load(f)

ADDON_INFO = {
    "name": "dnsmasq-dhcp",
    "slug": "self",
    "ingress": True,
    "ingress_port": 8100,
    "ingress_entry": "/api/hassio_ingress/testtoken",
    "ip_address": "0.0.0.0",
    "options": OPTIONS,
}


def ok(data):
    return {"result": "ok", "data": data}


class Handler(BaseHTTPRequestHandler):
    def _reply(self, payload, status=200):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def _handle(self):
        path = self.path.split("?")[0].rstrip("/")
        print(f"supervisor-mock: {self.command} {self.path}", flush=True)
        if path == "/addons/self/info":
            self._reply(ok(ADDON_INFO))
        elif path == "/addons/self/options/config":
            self._reply(ok(OPTIONS))
        elif path == "/supervisor/ping":
            self._reply(ok({}))
        else:
            self._reply(ok({}))

    do_GET = do_POST = do_PUT = _handle

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 80), Handler).serve_forever()
