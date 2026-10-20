#!/usr/bin/env python3
"""Installer trust, staging and no-overwrite tests using only the standard library."""
from __future__ import annotations

import argparse
from contextlib import redirect_stdout
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


PROJECT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("store_image_install", PROJECT / "deploy/install.py")
installer = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(installer)
REVISION = "abcd1234" * 5
REPOSITORY = "ghcr.io/zxgad3wdasaq1e-ai/faka-58"
DIGEST = "sha256:" + "1a" * 32
PINNED = REPOSITORY + "@" + DIGEST


class Response(io.BytesIO):
    status = 200

    def __init__(self, url: str, contents: bytes = b"runtime fixture\n"):
        super().__init__(contents)
        self.url = url

    def geturl(self) -> str:
        return self.url


def options(**overrides) -> argparse.Namespace:
    values = {"domain": "xioenai.vip", "local": False, "image": REPOSITORY + ":latest"}
    values.update(overrides)
    return argparse.Namespace(**values)


class InstallTests(unittest.TestCase):
    def setUp(self) -> None:
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.metadata = {
            "Config": {"Labels": {
                "org.opencontainers.image.source": installer.SOURCE,
                "org.opencontainers.image.revision": REVISION,
            }},
            "RepoDigests": [PINNED],
        }
        self.commands = []
        self.urls = []
        self.process_patch = patch.object(installer.subprocess, "run", side_effect=self.run_process)
        self.process = self.process_patch.start()
        self.addCleanup(self.process_patch.stop)
        self.which_patch = patch.object(installer.shutil, "which", return_value="/usr/bin/docker")
        self.which_patch.start()
        self.addCleanup(self.which_patch.stop)
        self.http_patch = patch.object(installer.urllib.request, "urlopen", side_effect=self.download)
        self.http = self.http_patch.start()
        self.addCleanup(self.http_patch.stop)

    def run_process(self, command, **kwargs):
        self.commands.append(command)
        output = json.dumps(self.metadata) if command[:3] == ["docker", "image", "inspect"] else "ready"
        return subprocess.CompletedProcess(command, 0, output, "")

    def download(self, request, **kwargs):
        self.urls.append(request.full_url)
        return Response(request.full_url)

    def install(self, **overrides):
        with redirect_stdout(io.StringIO()):
            return installer.install(options(**overrides), self.root)

    def assert_no_runtime(self) -> None:
        self.assertFalse((self.root / "compose.yml").exists())
        self.assertFalse((self.root / "scripts").exists())
        self.assertFalse((self.root / "deploy").exists())

    def test_pins_image_and_downloads_every_file_from_its_commit(self) -> None:
        self.assertEqual(self.install(), PINNED)
        self.assertEqual(self.urls, [f"{installer.RAW_BASE}/{REVISION}/{name}" for name in installer.RUNTIME_FILES])
        self.assertTrue(all("/main/" not in url for url in self.urls))
        for relative in installer.RUNTIME_FILES:
            self.assertEqual((self.root / relative).read_bytes(), b"runtime fixture\n")
        self.assertEqual(self.commands[0], ["docker", "compose", "version"])
        self.assertEqual(self.commands[1], ["docker", "info", "--format", "{{.ServerVersion}}"])
        self.assertEqual(self.commands[2], ["docker", "pull", REPOSITORY + ":latest"])
        self.assertEqual(self.commands[3], ["docker", "image", "inspect", REPOSITORY + ":latest", "--format", "{{json .}}"])
        self.assertEqual(self.commands[-1], [sys.executable, str(self.root / "scripts/setup.py"), "--domain", "xioenai.vip", "--image", PINNED, "--start", "--seed"])
        self.assertFalse((self.root / "config.yml").exists())

    def test_local_mode_passes_only_local_flag_and_immutable_image(self) -> None:
        self.install(local=True)
        self.assertEqual(self.commands[-1], [sys.executable, str(self.root / "scripts/setup.py"), "--local", "--image", PINNED, "--start", "--seed"])

    def test_existing_private_files_are_unchanged(self) -> None:
        (self.root / "config.yml").write_bytes(b"private-existing-config")
        (self.root / ".env").write_bytes(b"private-existing-env")
        with self.assertRaisesRegex(installer.InstallError, "非空"):
            self.install()
        self.assertEqual((self.root / "config.yml").read_bytes(), b"private-existing-config")
        self.assertEqual((self.root / ".env").read_bytes(), b"private-existing-env")
        self.assertFalse(any(command[:2] == ["docker", "pull"] for command in self.commands))
        self.http.assert_not_called()

    def test_allows_only_installer_scripts_without_overwriting_them(self) -> None:
        for name in installer.ALLOWED_INSTALLER_FILES:
            (self.root / name).write_bytes(b"original installer")
        self.install()
        for name in installer.ALLOWED_INSTALLER_FILES:
            self.assertEqual((self.root / name).read_bytes(), b"original installer")

    def test_installer_named_directory_is_not_allowed(self) -> None:
        (self.root / "install.py").mkdir()
        with self.assertRaises(installer.InstallError):
            self.install()
        self.http.assert_not_called()

    def test_rejects_missing_or_untrusted_oci_source(self) -> None:
        for source in (None, "https://github.com/other/repository", installer.SOURCE + "/"):
            with self.subTest(source=source):
                self.metadata["Config"]["Labels"]["org.opencontainers.image.source"] = source
                with self.assertRaises(installer.InstallError):
                    self.install()
                self.assert_no_runtime()
        self.http.assert_not_called()

    def test_revision_must_be_complete_hex_commit(self) -> None:
        for revision in (None, "main", "a" * 39, "a" * 41, "g" * 40, "../main", "a" * 40 + "\n"):
            with self.subTest(revision=revision):
                self.metadata["Config"]["Labels"]["org.opencontainers.image.revision"] = revision
                with self.assertRaises(installer.InstallError):
                    self.install()
                self.assert_no_runtime()
        self.http.assert_not_called()

    def test_missing_labels_do_not_download_anything(self) -> None:
        self.metadata["Config"]["Labels"] = None
        with self.assertRaises(installer.InstallError):
            self.install()
        self.http.assert_not_called()
        self.assert_no_runtime()

    def test_chooses_digest_only_for_requested_repository(self) -> None:
        self.metadata["RepoDigests"] = ["ghcr.io/other/repository@sha256:" + "22" * 32, PINNED]
        self.assertEqual(self.install(), PINNED)

    def test_rejects_missing_foreign_or_malformed_digests(self) -> None:
        for digests in ([], None, ["ghcr.io/other/repository@" + DIGEST], [REPOSITORY + "@sha256:" + "g" * 64], [REPOSITORY + ":latest"]):
            with self.subTest(digests=digests):
                self.metadata["RepoDigests"] = digests
                with self.assertRaises(installer.InstallError):
                    self.install()
                self.assert_no_runtime()
        self.http.assert_not_called()

    def test_explicit_digest_must_also_match_selected_digest(self) -> None:
        with self.assertRaises(installer.InstallError):
            self.install(image=REPOSITORY + "@sha256:" + "33" * 32)
        self.http.assert_not_called()

    def test_rejects_unknown_download_paths_before_network_access(self) -> None:
        for name in (".env", "config.yml", "../../.env", "deploy/other.py", "https://example.test/a"):
            with self.subTest(name=name):
                with self.assertRaises(installer.InstallError):
                    installer.download_runtime_file(REVISION, name, self.root)
        self.http.assert_not_called()
        self.assertEqual(list(self.root.iterdir()), [])

    def test_failed_download_leaves_target_empty(self) -> None:
        def fail_on_second(request, **kwargs):
            if request.full_url.endswith("scripts/setup.py"):
                raise OSError("simulated network interruption")
            return Response(request.full_url)
        self.http.side_effect = fail_on_second
        with self.assertRaises(OSError):
            self.install()
        self.assertEqual(list(self.root.iterdir()), [])
        self.assertFalse(any(command[0] == sys.executable for command in self.commands))

    def test_redirect_away_from_pinned_source_is_rejected(self) -> None:
        self.http.side_effect = lambda request, **kwargs: Response("https://example.test/runtime.py")
        with self.assertRaises(installer.InstallError):
            self.install()
        self.assertEqual(list(self.root.iterdir()), [])

    def test_target_is_rechecked_after_download(self) -> None:
        def concurrent_file(request, **kwargs):
            if request.full_url.endswith(installer.RUNTIME_FILES[-1]):
                (self.root / "config.yml").write_bytes(b"concurrent private file")
            return Response(request.full_url)
        self.http.side_effect = concurrent_file
        with self.assertRaises(installer.InstallError):
            self.install()
        self.assertEqual((self.root / "config.yml").read_bytes(), b"concurrent private file")
        self.assert_no_runtime()

    def test_invalid_image_is_rejected_without_docker_calls(self) -> None:
        for image in ("--help", "https://ghcr.io/x/y:tag", "ghcr.io/x/y", "ghcr.io/x/y:$TAG", REPOSITORY + ":latest\nSECRET=value"):
            with self.subTest(image=image):
                with self.assertRaises(installer.InstallError):
                    self.install(image=image)
        self.process.assert_not_called()
        self.http.assert_not_called()

    def test_pull_failure_has_ghcr_public_or_login_instructions(self) -> None:
        def fail_pull(command, **kwargs):
            if command[:2] == ["docker", "pull"]:
                raise subprocess.CalledProcessError(1, command)
            return self.run_process(command, **kwargs)
        self.process.side_effect = fail_pull
        with self.assertRaisesRegex(installer.InstallError, "public.*docker login ghcr.io"):
            self.install()
        self.http.assert_not_called()
        self.assertEqual(list(self.root.iterdir()), [])

    def test_setup_failure_keeps_files_for_safe_retry(self) -> None:
        def fail_setup(command, **kwargs):
            if command[0] == sys.executable:
                raise subprocess.CalledProcessError(1, command)
            return self.run_process(command, **kwargs)
        self.process.side_effect = fail_setup
        with self.assertRaisesRegex(installer.InstallError, "setup.py --start --seed"):
            self.install()
        self.assertTrue((self.root / "scripts/setup.py").is_file())
        self.assertTrue((self.root / "compose.yml").is_file())

    def test_missing_docker_does_not_download_or_write_files(self) -> None:
        with patch.object(installer.shutil, "which", return_value=None):
            with self.assertRaisesRegex(installer.InstallError, "未找到 Docker"):
                self.install()
        self.process.assert_not_called()
        self.http.assert_not_called()
        self.assertEqual(list(self.root.iterdir()), [])

    def test_unavailable_daemon_does_not_pull_or_write_files(self) -> None:
        def fail_info(command, **kwargs):
            if command[:2] == ["docker", "info"]:
                raise subprocess.CalledProcessError(1, command)
            return self.run_process(command, **kwargs)
        self.process.side_effect = fail_info
        with self.assertRaisesRegex(installer.InstallError, "daemon"):
            self.install()
        self.http.assert_not_called()
        self.assertFalse(any(command[:2] == ["docker", "pull"] for command in self.commands))
        self.assertEqual(list(self.root.iterdir()), [])

    def test_empty_or_oversized_runtime_is_not_published(self) -> None:
        for contents in (b"", b"x" * (installer.MAX_DOWNLOAD_BYTES + 1)):
            with self.subTest(size=len(contents)):
                self.http.side_effect = lambda request, **kwargs: Response(request.full_url, contents)
                with self.assertRaises(installer.InstallError):
                    self.install()
                self.assertEqual(list(self.root.iterdir()), [])

    def test_publish_collision_rolls_back_only_files_it_created(self) -> None:
        original_open = installer.os.open
        collided = self.root / "deploy/Caddyfile"
        def concurrent_runtime_file(path, flags, mode=0o777, **kwargs):
            if Path(path) == collided:
                collided.write_bytes(b"concurrent user file")
            return original_open(path, flags, mode, **kwargs)
        with patch.object(installer.os, "open", side_effect=concurrent_runtime_file):
            with self.assertRaises(FileExistsError):
                self.install()
        self.assertEqual(collided.read_bytes(), b"concurrent user file")
        self.assertFalse((self.root / "compose.yml").exists())
        self.assertFalse((self.root / "scripts").exists())
        self.assertFalse(any(command[0] == sys.executable for command in self.commands))

    def test_quick_installer_does_not_accept_minio_option(self) -> None:
        with patch.object(sys, "stderr", io.StringIO()):
            with self.assertRaises(SystemExit) as error:
                installer.main(["--storage", "minio"])
        self.assertEqual(error.exception.code, 2)
        self.process.assert_not_called()

    def test_publish_rejects_resolved_paths_outside_target(self) -> None:
        target = self.root / "store"
        target.mkdir()
        staging = self.root / "staging"
        staging.mkdir()
        outside = self.root / "outside.py"
        outside.write_bytes(b"existing user file")
        for relative in ("../outside.py", "../store-evil/escaped.py", "nested/../../outside.py", str(outside)):
            with self.subTest(relative=relative), patch.object(installer, "RUNTIME_FILES", (relative,)):
                with self.assertRaisesRegex(installer.InstallError, "超出安装目录"):
                    installer.publish_runtime(staging, target)
                self.assertEqual(list(target.iterdir()), [])
                self.assertEqual(outside.read_bytes(), b"existing user file")
                self.assertFalse((self.root / "store-evil").exists())

    def test_publish_rejects_symlink_escape_after_target_recheck(self) -> None:
        target, staging, outside = (self.root / name for name in ("store", "staging", "store-evil"))
        for directory in (target, staging, outside):
            directory.mkdir()
        sentinel = outside / "setup.py"
        sentinel.write_bytes(b"existing user file")
        original_check = installer.ensure_empty_target
        def insert_symlink(root):
            original_check(root)
            try:
                (root / "scripts").symlink_to(outside, target_is_directory=True)
            except (OSError, NotImplementedError):
                self.skipTest("Directory symlinks are unavailable for this account")
        with patch.object(installer, "ensure_empty_target", side_effect=insert_symlink), patch.object(installer, "RUNTIME_FILES", ("scripts/setup.py",)):
            with self.assertRaisesRegex(installer.InstallError, "超出安装目录"):
                installer.publish_runtime(staging, target)
        self.assertEqual(sentinel.read_bytes(), b"existing user file")

    def test_python_38_passes_version_guard(self) -> None:
        with patch.object(installer.sys, "version_info", (3, 8, 0)), patch.object(installer, "install", return_value=PINNED) as install:
            self.assertEqual(installer.main(["--local"]), 0)
        install.assert_called_once()

    def test_python_before_38_stops_before_installing(self) -> None:
        with patch.object(installer.sys, "version_info", (3, 7, 17)), patch.object(installer, "install") as install, patch.object(sys, "stderr", io.StringIO()) as stderr:
            self.assertEqual(installer.main(["--local"]), 1)
            self.assertIn("Python 3.8+", stderr.getvalue())
        install.assert_not_called()


if __name__ == "__main__":
    unittest.main()
