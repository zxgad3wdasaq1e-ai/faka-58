#!/usr/bin/env python3
"""Exercise the shipped proxy config with Caddy; set CADDY_BINARY to opt in."""
from __future__ import annotations

import argparse
from contextlib import redirect_stdout
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import tempfile
import threading
import time
import unittest
from unittest.mock import patch
import urllib.request


REPOSITORY = Path(__file__).resolve().parents[2]
CADDY = os.environ.get("CADDY_BINARY") or shutil.which("caddy")


class HeaderEcho(BaseHTTPRequestHandler):
    def do_GET(self):
        body = json.dumps(dict(self.headers.items())).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *arguments):
        pass


@unittest.skipUnless(CADDY, "Caddy is optional; set CADDY_BINARY to run proxy integration checks")
class CaddyProxyTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.backend = ThreadingHTTPServer(("127.0.0.1", 0), HeaderEcho)
        threading.Thread(target=self.backend.serve_forever, daemon=True).start()
        self.addCleanup(self.backend.server_close)
        self.addCleanup(self.backend.shutdown)

    def start_proxy(self, mode, simulated_trusted_peer=False):
        source = (REPOSITORY / "deploy/Caddyfile").read_text(encoding="utf-8")
        source = source.replace("app:8080", f"127.0.0.1:{self.backend.server_port}")
        source = source.replace("\n{\n    servers {", "\n{\n    admin off\n    servers {")
        if simulated_trusted_peer:
            # Replace the trusted *peer* only in this temporary test fixture.
            lines = source.splitlines()
            source = "\n".join("    trusted_proxies static 127.0.0.1/32" if line.strip().startswith("trusted_proxies static ") else line for line in lines) + "\n"
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            port = listener.getsockname()[1]
        path = self.root / "Caddyfile"
        path.write_text(source, encoding="utf-8")
        environment = {**os.environ, "SITE_ADDRESS": f"http://127.0.0.1:{port}", "CADDY_PROXY_MODE": mode}
        validation = subprocess.run([CADDY, "validate", "--config", str(path), "--adapter", "caddyfile"], env=environment, capture_output=True, text=True)
        self.assertEqual(validation.returncode, 0, validation.stderr)
        process = subprocess.Popen([CADDY, "run", "--config", str(path), "--adapter", "caddyfile"], env=environment, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
        def stop():
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
        self.addCleanup(stop)
        url = f"http://127.0.0.1:{port}/headers"
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            try:
                with urllib.request.urlopen(url, timeout=0.3):
                    return url
            except OSError:
                time.sleep(0.05)
        self.fail("Caddy did not become ready")

    def forwarded_headers(self, url, visitor="203.0.113.7"):
        request = urllib.request.Request(url, headers={
            "CF-Connecting-IP": visitor,
            "X-Forwarded-For": "198.51.100.8, 198.51.100.9",
            "X-Real-IP": "198.51.100.10",
        })
        with urllib.request.urlopen(request, timeout=2) as response:
            return {key.lower(): value for key, value in json.load(response).items()}

    def test_direct_mode_rejects_all_spoofed_client_ip_headers(self):
        headers = self.forwarded_headers(self.start_proxy("direct"))
        self.assertEqual(headers["x-forwarded-for"], "127.0.0.1")
        self.assertEqual(headers["x-real-ip"], "127.0.0.1")
        self.assertNotIn("cf-connecting-ip", headers)

    def test_cloudflare_mode_rejects_headers_from_a_non_cloudflare_peer(self):
        headers = self.forwarded_headers(self.start_proxy("cloudflare"))
        self.assertEqual(headers["x-forwarded-for"], "127.0.0.1")
        self.assertEqual(headers["x-real-ip"], "127.0.0.1")
        self.assertNotIn("cf-connecting-ip", headers)

    def test_verified_proxy_normalizes_ipv4_ipv6_and_rejects_invalid_ip(self):
        url = self.start_proxy("cloudflare", simulated_trusted_peer=True)
        for visitor, expected in (("203.0.113.7", "203.0.113.7"), ("2001:db8:1:2::1234", "2001:db8:1:2::1234"), ("invalid-client-ip", "127.0.0.1")):
            with self.subTest(visitor=visitor):
                headers = self.forwarded_headers(url, visitor)
                self.assertEqual(headers["x-forwarded-for"], expected)
                self.assertEqual(headers["x-real-ip"], expected)
                self.assertNotIn("cf-connecting-ip", headers)

    def test_minio_generated_config_validates_in_both_proxy_modes(self):
        spec = importlib.util.spec_from_file_location("caddy_test_setup", REPOSITORY / "scripts/setup.py")
        setup = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(setup)
        (self.root / "deploy").mkdir()
        shutil.copyfile(REPOSITORY / "deploy/Caddyfile", self.root / "deploy/Caddyfile")
        options = argparse.Namespace(domain="store.example", local=False, admin_user="admin", subnet="172.29.240.0/24", storage="minio", image=None)
        with patch.object(setup, "ROOT", self.root), redirect_stdout(io.StringIO()):
            setup.initialize(options)
        for mode in ("direct", "cloudflare"):
            with self.subTest(mode=mode):
                validation = subprocess.run([CADDY, "validate", "--config", str(self.root / "deploy/runtime/Caddyfile"), "--adapter", "caddyfile"], env={**os.environ, "SITE_ADDRESS": "http://localhost", "CADDY_PROXY_MODE": mode}, capture_output=True, text=True)
                self.assertEqual(validation.returncode, 0, validation.stderr)


if __name__ == "__main__":
    unittest.main()
