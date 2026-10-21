#!/usr/bin/env python3
"""Installer and restore mode regression tests; no Docker daemon is required."""
from __future__ import annotations

import argparse
from contextlib import redirect_stdout
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


REPOSITORY = Path(__file__).resolve().parents[2]


def load_module(name: str, relative: str):
    spec = importlib.util.spec_from_file_location(name, REPOSITORY / relative)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


setup = load_module("store_setup", "scripts/setup.py")
backup = load_module("store_backup", "deploy/backup.py")
IMAGE = "ghcr.io/example/store:v1.2.3"


def options(**overrides) -> argparse.Namespace:
    values = {"domain": "example.com", "local": False, "admin_user": "admin", "subnet": "172.29.240.0/24", "storage": "local", "image": None}
    values.update(overrides)
    return argparse.Namespace(**values)


class SetupTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        (self.root / "deploy").mkdir()
        (self.root / "deploy/Caddyfile").write_text((REPOSITORY / "deploy/Caddyfile").read_text(encoding="utf-8"), encoding="utf-8")
        self.root_patch = patch.object(setup, "ROOT", self.root)
        self.root_patch.start()
        self.addCleanup(self.root_patch.stop)

    def initialize(self, **overrides) -> dict:
        with redirect_stdout(io.StringIO()):
            return setup.initialize(options(**overrides))

    def private_snapshot(self) -> dict[str, bytes]:
        return {str(path.relative_to(self.root)): path.read_bytes() for path in self.root.rglob("*") if path.is_file()}

    def test_source_default_and_image_state_match_environment(self) -> None:
        state = self.initialize()
        self.assertEqual(state["image"], "")
        self.assertIn("APP_IMAGE=faka-58:local\n", (self.root / ".env").read_text(encoding="utf-8"))
        config = json.loads((self.root / "config.yml").read_text(encoding="utf-8"))
        self.assertEqual(config["app"]["totp_issuer"], "faka-58")
        self.assertIn("user=faka-58 ", config["database"]["dsn"])
        self.assertIn("dbname=faka-58 ", config["database"]["dsn"])
        self.assertNotIn("password=", config["database"]["dsn"])
        self.assertEqual(config["database"]["password_file"], "/run/secrets/postgres_password")
        database_password = (self.root / "deploy/secrets/postgres_password").read_text(encoding="utf-8").strip()
        self.assertNotIn(database_password, (self.root / "config.yml").read_text(encoding="utf-8"))
        self.assertEqual(config["redis"]["prefix"], "faka-58")
        self.assertIn("POSTGRES_DB=faka-58\nPOSTGRES_APP_USER=faka-58\n", (self.root / ".env").read_text(encoding="utf-8"))
        self.assertFalse(config["email"]["enabled"])
        self.assertEqual(config["upload"]["storage"], "local")
        self.assertEqual(config["server"]["trusted_proxies"], ["172.29.240.10/32"])
        self.assertIn("CADDY_PROXY_MODE=direct\n", (self.root / ".env").read_text(encoding="utf-8"))
        self.assertEqual((self.root / "deploy/runtime/Caddyfile").read_text(encoding="utf-8"), (self.root / "deploy/Caddyfile").read_text(encoding="utf-8"))
        secrets = [config["app"]["secret_key"], config["jwt"]["secret"], config["user_jwt"]["secret"]]
        self.assertEqual(len(set(secrets)), 3)
        self.assertTrue(all(len(value) >= 32 for value in secrets))

    def test_minio_preserves_proxy_trust_and_application_ip_normalization(self) -> None:
        self.initialize(storage="minio")
        caddy = (self.root / "deploy/runtime/Caddyfile").read_text(encoding="utf-8")
        self.assertIn("handle_path /media/*", caddy)
        self.assertIn("reverse_proxy minio:9000", caddy)
        self.assertIn("    handle {\n        import storefront_proxy\n    }", caddy)
        self.assertEqual(caddy.count("reverse_proxy app:8080"), 1)
        self.assertEqual(caddy.count("import storefront_proxy"), 1)
        self.assertIn("header_up X-Forwarded-For {client_ip}", caddy)
        self.assertIn("header_up -CF-Connecting-IP", caddy)
        self.assertIn("import {$CADDY_PROXY_MODE:direct}", caddy)

    def test_image_installation_is_consistent(self) -> None:
        state = self.initialize(image=IMAGE)
        self.assertEqual(state["image"], IMAGE)
        self.assertEqual(json.loads((self.root / "deploy/runtime/installation.json").read_text(encoding="utf-8")), state)
        self.assertIn(f"APP_IMAGE={IMAGE}\n", (self.root / ".env").read_text(encoding="utf-8"))
        self.assertEqual(state["storage"], "local")

    def test_valid_references(self) -> None:
        for reference in (IMAGE, "ghcr.io/example/store@sha256:" + "a1" * 32, "example/store:v1", "store:local", "localhost:5000/team/store:v1", "registry:5000/team/store:v1"):
            with self.subTest(reference=reference):
                self.assertEqual(setup.validate_image(reference), reference)

    def test_invalid_images_do_not_write_private_files(self) -> None:
        invalid = ("", " ghcr.io/example/store:v1", "ghcr.io/example/store:v1 ", "ghcr.io/example/store:v1\nCOMPOSE_PROFILES=tools", "ghcr.io/example/store:$TAG", "${APP_IMAGE}", "https://ghcr.io/example/store:v1", "ghcr.io/example/store", "ghcr.io/example/store@sha256:" + "a" * 63, "ghcr.io/example/store@sha256:" + "g" * 64, "ghcr.io/Example/store:v1", "ghcr.io/example//store:v1", "store:v1;command", "registry:0/store:v1", "registry:99999/store:v1")
        for reference in invalid:
            with self.subTest(reference=reference):
                with self.assertRaises(ValueError):
                    self.initialize(image=reference)
                self.assertFalse((self.root / "config.yml").exists())
                self.assertFalse((self.root / ".env").exists())
                self.assertFalse((self.root / "deploy/secrets").exists())
                self.assertFalse((self.root / "deploy/runtime").exists())

    def test_image_minio_rejected_before_private_writes(self) -> None:
        with self.assertRaisesRegex(ValueError, "MinIO"):
            self.initialize(image=IMAGE, storage="minio")
        self.assertFalse((self.root / ".env").exists())
        self.assertFalse((self.root / "deploy/secrets").exists())

    def test_existing_configuration_and_secrets_never_overwritten(self) -> None:
        state = self.initialize(image=IMAGE)
        before = self.private_snapshot()
        self.assertEqual(self.initialize(image=IMAGE, local=True, domain="different.example", admin_user="another", storage="minio"), state)
        self.assertEqual(self.initialize(), state)
        self.assertEqual(self.private_snapshot(), before)
        with self.assertRaisesRegex(ValueError, "不一致"):
            self.initialize(image="ghcr.io/example/store:v2")
        self.assertEqual(self.private_snapshot(), before)

    def test_legacy_source_state_cannot_silently_switch_to_image(self) -> None:
        state = self.initialize()
        del state["image"]
        state_path = self.root / "deploy/runtime/installation.json"
        state_path.write_text(json.dumps(state), encoding="utf-8")
        before = self.private_snapshot()
        self.assertEqual(self.initialize(), state)
        with self.assertRaisesRegex(ValueError, "不一致"):
            self.initialize(image=IMAGE)
        self.assertEqual(self.private_snapshot(), before)

    def test_legacy_names_and_database_are_preserved_on_retry(self) -> None:
        state = self.initialize()
        env = self.root / ".env"
        env.write_text(env.read_text(encoding="utf-8").replace("COMPOSE_PROJECT_NAME=faka-58", "COMPOSE_PROJECT_NAME=getgpt").replace("POSTGRES_DB=faka-58\nPOSTGRES_APP_USER=faka-58\n", ""), encoding="utf-8")
        path = self.root / "config.yml"
        config = json.loads(path.read_text(encoding="utf-8"))
        config["database"]["dsn"] = "host=postgres user=getgpt password=fixture dbname=getgpt"
        path.write_text(json.dumps(config), encoding="utf-8")
        before = self.private_snapshot()
        self.assertEqual(self.initialize(), state)
        self.assertEqual(self.private_snapshot(), before)

    def test_incomplete_existing_configuration_is_not_overwritten(self) -> None:
        (self.root / ".env").write_text("existing-private-data", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "不完整"):
            self.initialize(image=IMAGE)
        self.assertEqual((self.root / ".env").read_text(encoding="utf-8"), "existing-private-data")
        self.assertFalse((self.root / "config.yml").exists())

    def test_start_commands_for_source_and_image(self) -> None:
        for image in (None, "", IMAGE):
            with self.subTest(image=image), patch.object(setup.shutil, "which", return_value="docker"), patch.object(setup, "run") as run:
                state = {} if image is None else {"image": image}
                setup.start(state, False)
                commands = [call.args for call in run.call_args_list]
                expected = [("docker", "compose", "version")]
                if image:
                    expected.append(("docker", "compose", "pull"))
                expected.append(("docker", "compose", "up", "-d", "--no-build" if image else "--build", "--wait", "--wait-timeout", "300"))
                self.assertEqual(commands, expected)

    def test_failed_pull_never_falls_back_to_build(self) -> None:
        def command(*arguments):
            if arguments[-1] == "pull":
                raise subprocess.CalledProcessError(1, arguments)
        with patch.object(setup.shutil, "which", return_value="docker"), patch.object(setup, "run", side_effect=command) as run:
            with self.assertRaises(subprocess.CalledProcessError):
                setup.start({"image": IMAGE}, False)
            self.assertEqual(len(run.call_args_list), 2)
            self.assertFalse(any("up" in call.args for call in run.call_args_list))


class RestoreTests(unittest.TestCase):
    def prepare_backup(self, parent: Path, image) -> tuple[Path, Path]:
        root, source = parent / "checkout", parent / "backup"
        root.mkdir()
        private = source / "private"
        files = {
            ".env": "COMPOSE_PROJECT_NAME=faka-58\nAPP_IMAGE=" + (image or "faka-58:local") + "\n",
            "config.yml": "{}",
            "deploy/runtime/installation.json": json.dumps({} if image is None else {"image": image}),
            "deploy/runtime/Caddyfile": "localhost {\n reverse_proxy app:8080\n}",
            "deploy/secrets/postgres_password": "fake-test-password",
            "deploy/secrets/postgres_admin_password": "fake-test-admin-password",
            "deploy/secrets/redis_password": "fake-test-redis-password",
        }
        for relative, contents in files.items():
            destination = private / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_text(contents, encoding="utf-8")
        (source / "database.dump").write_bytes(b"test-dump")
        manifest = {str(path.relative_to(source)).replace("\\", "/"): backup.digest(path) for path in source.rglob("*") if path.is_file()}
        (source / "checksums.json").write_text(json.dumps(manifest), encoding="utf-8")
        return root, source.resolve()

    def test_restore_uses_restored_installation_mode(self) -> None:
        for image in (None, "", IMAGE):
            with self.subTest(image=image), tempfile.TemporaryDirectory() as temporary:
                root, source = self.prepare_backup(Path(temporary), image)
                with patch.object(backup, "ROOT", root), patch.object(backup, "configuration", return_value={"volumes": {}}), patch.object(backup, "compose") as compose, redirect_stdout(io.StringIO()):
                    backup.restore(source, True)
                commands = [call.args for call in compose.call_args_list]
                final = ("up", "-d", "--no-build" if image else "--build", "--wait", "--wait-timeout", "300")
                self.assertEqual(commands[-1], final)
                self.assertEqual(("pull",) in commands, bool(image))
                if image:
                    self.assertEqual(commands[-2], ("pull",))
                self.assertFalse((root / "deploy/runtime/restore-state.json").exists())

    def test_restore_failed_pull_does_not_start_or_build_app(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            root, source = self.prepare_backup(Path(temporary), IMAGE)
            def command(*arguments, **kwargs):
                if arguments == ("pull",):
                    raise subprocess.CalledProcessError(1, arguments)
            with patch.object(backup, "ROOT", root), patch.object(backup, "configuration", return_value={"volumes": {}}), patch.object(backup, "compose", side_effect=command) as compose, redirect_stdout(io.StringIO()):
                with self.assertRaises(subprocess.CalledProcessError):
                    backup.restore(source, True)
            self.assertFalse(any("--build" in call.args or "--no-build" in call.args for call in compose.call_args_list))
            self.assertTrue((root / "deploy/runtime/restore-state.json").is_file())

    def test_restore_rejects_resolved_manifest_paths_outside_backup(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            root, source = self.prepare_backup(parent, IMAGE)
            outside = parent / "outside.dat"
            outside.write_bytes(b"existing user file")
            sibling = parent / "backup-evil"
            sibling.mkdir()
            (sibling / "outside.dat").write_bytes(b"existing user file")
            manifest_path = source / "checksums.json"
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            for relative in ("../outside.dat", "../backup-evil/outside.dat", "private/../../outside.dat", str(outside)):
                with self.subTest(relative=relative):
                    manifest_path.write_text(json.dumps({**manifest, relative: backup.digest(outside)}), encoding="utf-8")
                    with patch.object(backup, "ROOT", root), patch.object(backup, "compose") as compose, patch.object(backup, "docker") as docker:
                        with self.assertRaisesRegex(ValueError, "备份校验失败"):
                            backup.restore(source, True)
                    compose.assert_not_called()
                    docker.assert_not_called()
                    self.assertEqual(list(root.iterdir()), [])
                    self.assertEqual(outside.read_bytes(), b"existing user file")

    def test_restore_rejects_symlink_outside_backup(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary)
            root, source = self.prepare_backup(parent, IMAGE)
            outside = parent / "backup-evil"
            outside.mkdir()
            sentinel = outside / "outside.dat"
            sentinel.write_bytes(b"existing user file")
            try:
                (source / "linked").symlink_to(outside, target_is_directory=True)
            except (OSError, NotImplementedError):
                self.skipTest("Directory symlinks are unavailable for this account")
            manifest_path = source / "checksums.json"
            manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
            manifest["linked/outside.dat"] = backup.digest(sentinel)
            manifest_path.write_text(json.dumps(manifest), encoding="utf-8")
            with patch.object(backup, "ROOT", root), patch.object(backup, "compose") as compose, patch.object(backup, "docker") as docker:
                with self.assertRaisesRegex(ValueError, "备份校验失败"):
                    backup.restore(source, True)
            compose.assert_not_called()
            docker.assert_not_called()
            self.assertEqual(list(root.iterdir()), [])
            self.assertEqual(sentinel.read_bytes(), b"existing user file")


if __name__ == "__main__":
    unittest.main()
