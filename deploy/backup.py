#!/usr/bin/env python3
"""Consistent Compose backups; restore is intended for a fresh checkout/host."""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[1]
VOLUMES = ("uploads", "app_state", "redis_data", "minio_data", "caddy_data", "caddy_config")


def docker(*arguments: str, **kwargs) -> subprocess.CompletedProcess:
    return subprocess.run(["docker", *arguments], cwd=ROOT, check=True, **kwargs)


def compose(*arguments: str, **kwargs) -> subprocess.CompletedProcess:
    return docker("compose", *arguments, **kwargs)


def configuration() -> dict:
    return json.loads(compose("config", "--format", "json", capture_output=True, text=True).stdout)


def digest(path: Path) -> str:
    result = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            result.update(chunk)
    return result.hexdigest()


def archive_volume(name: str, output: Path, filename: str, restore: bool = False, resume: bool = False) -> None:
    docker("volume", "inspect", name, stdout=subprocess.DEVNULL)
    if restore:
        guard = "" if resume else 'test -z "$(ls -A /target)" || { echo "目标数据卷不是空卷，已拒绝覆盖" >&2; exit 1; }; '
        command = guard + 'tar -xzf "/backup/$1" -C /target'
        target = "/target"
        volume_mount = f"type=volume,source={name},target={target}"
    else:
        command = 'tar -czf "/backup/$1" -C /source .'
        volume_mount = f"type=volume,source={name},target=/source,readonly"
    docker("run", "--rm", "--mount", volume_mount, "--mount", f"type=bind,source={output},target=/backup", "alpine:3.22", "sh", "-ec", command, "backup", filename)


def backup(output: Path) -> None:
    output.mkdir(parents=True, exist_ok=False, mode=0o700)
    output.chmod(0o700)
    config = configuration()
    active = set(compose("ps", "--status", "running", "--services", capture_output=True, text=True).stdout.split())
    if "postgres" not in active:
        raise ValueError("PostgreSQL 尚未运行，无法创建数据库备份。")
    stopped = [name for name in ("caddy", "app", "redis", "minio") if name in active]
    try:
        if stopped:
            compose("stop", *stopped)
        # Binary stdout is written by Python, avoiding PowerShell text encoding.
        with (output / "database.dump").open("wb") as handle:
            compose("exec", "-T", "postgres", "pg_dump", "-U", "postgres", "-d", "getgpt", "-Fc", stdout=handle)
        for relative in (".env", "config.yml", "deploy/secrets", "deploy/runtime"):
            source = ROOT / relative
            destination = output / "private" / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            if source.is_dir():
                shutil.copytree(source, destination)
            else:
                shutil.copy2(source, destination)
        for volume in VOLUMES:
            if volume in config.get("volumes", {}):
                archive_volume(config["volumes"][volume]["name"], output, volume + ".tar.gz")
        manifest = {str(path.relative_to(output)).replace("\\", "/"): digest(path) for path in output.rglob("*") if path.is_file()}
        (output / "checksums.json").write_text(json.dumps(manifest, indent=2), encoding="utf-8")
        print(f"备份完成: {output}。包含数据库、图床及密钥，请加密后保存到异机。")
    finally:
        # Resume only services that were running before the backup.
        for group in (("redis", "minio"), ("app",), ("caddy",)):
            services = [name for name in group if name in stopped]
            if services:
                compose("up", "-d", "--no-deps", "--wait", *services)


def restore(source: Path, confirmed: bool) -> None:
    if not confirmed:
        raise ValueError("恢复会替换目标数据库。仅在新主机/空数据卷中使用，并显式添加 --confirm-replace-database。")
    source = source.resolve()
    manifest = json.loads((source / "checksums.json").read_text(encoding="utf-8"))
    for relative, expected in manifest.items():
        path = (source / relative).resolve()
        try:
            path.relative_to(source)
        except ValueError:
            raise ValueError(f"备份校验失败: {relative}") from None
        if not path.is_file() or digest(path) != expected:
            raise ValueError(f"备份校验失败: {relative}")
    mandatory = ("private/.env", "private/config.yml", "private/deploy/runtime/installation.json", "private/deploy/runtime/Caddyfile", "private/deploy/secrets/postgres_password", "private/deploy/secrets/postgres_admin_password", "private/deploy/secrets/redis_password", "database.dump")
    if any(relative not in manifest for relative in mandatory):
        raise ValueError("备份缺少必要数据库/私有配置文件，已拒绝恢复。")
    marker_path = ROOT / "deploy/runtime/restore-state.json"
    fingerprint = digest(source / "checksums.json")
    if marker_path.is_file():
        progress = json.loads(marker_path.read_text(encoding="utf-8"))
        if progress.get("backup_fingerprint") != fingerprint:
            raise ValueError("当前目录有另一次备份的恢复状态，已拒绝混用备份。")
        print("继续同一备份的未完成恢复。")
    else:
        if any((ROOT / item).exists() for item in (".env", "config.yml", "deploy/secrets", "deploy/runtime")):
            raise ValueError("当前目录已有部署配置。请在全新的 Git checkout 中恢复；不会覆盖现有密钥。")
        project = "getgpt"
        for line in (source / "private/.env").read_text(encoding="utf-8").splitlines():
            if line.startswith("COMPOSE_PROJECT_NAME="):
                project = line.partition("=")[2].strip()
        if not project or any(character not in "abcdefghijklmnopqrstuvwxyz0123456789_-" for character in project):
            raise ValueError("备份的 Compose 项目名无效。")
        expected_volumes = {volume: project + "_" + volume for volume in VOLUMES if (source / (volume + ".tar.gz")).is_file()}
        # Check every destination before copying credentials or unpacking anything.
        for name in expected_volumes.values():
            result = subprocess.run(["docker", "volume", "inspect", name], cwd=ROOT, capture_output=True)
            if result.returncode == 0:
                docker("run", "--rm", "--mount", f"type=volume,source={name},target=/target,readonly", "alpine:3.22", "sh", "-ec", 'test -z "$(ls -A /target)" || { echo "目标数据卷不是空卷，已拒绝覆盖" >&2; exit 1; }')
        progress = {"backup_fingerprint": fingerprint, "expected_volumes": expected_volumes, "credentials_restored": False, "completed_volumes": [], "in_progress": "", "database_restored": False}

    def checkpoint() -> None:
        marker_path.parent.mkdir(parents=True, exist_ok=True)
        temporary = marker_path.with_suffix(".tmp")
        temporary.write_text(json.dumps(progress, indent=2), encoding="utf-8")
        temporary.replace(marker_path)

    checkpoint()
    if not progress["credentials_restored"]:
        for relative in (".env", "config.yml", "deploy/secrets", "deploy/runtime"):
            original = source / "private" / relative
            destination = ROOT / relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            if original.is_dir():
                shutil.copytree(original, destination, dirs_exist_ok=True)
            else:
                shutil.copy2(original, destination)
        progress["credentials_restored"] = True
        checkpoint()
    config = configuration()
    for volume, expected in progress["expected_volumes"].items():
        if config.get("volumes", {}).get(volume, {}).get("name") != expected:
            raise ValueError("目标 Compose 数据卷与备份项目名不一致。检查宿主机 COMPOSE_PROJECT_NAME 环境变量后重试。")
    # Volume archives are restored before starting their owners, into empty volumes.
    for volume in VOLUMES:
        filename = volume + ".tar.gz"
        if (source / filename).is_file() and volume in config.get("volumes", {}):
            if volume in progress["completed_volumes"]:
                continue
            name = config["volumes"][volume]["name"]
            docker("volume", "create", name, stdout=subprocess.DEVNULL)
            continuing = progress["in_progress"] == volume
            progress["in_progress"] = volume
            checkpoint()
            archive_volume(name, source, filename, restore=True, resume=continuing)
            progress["completed_volumes"].append(volume)
            progress["in_progress"] = ""
            checkpoint()
    if not progress["database_restored"]:
        compose("up", "-d", "--wait", "postgres")
        with (source / "database.dump").open("rb") as handle:
            compose("exec", "-T", "postgres", "pg_restore", "-U", "postgres", "-d", "getgpt", "--clean", "--if-exists", "--exit-on-error", "--single-transaction", stdin=handle)
        progress["database_restored"] = True
        checkpoint()
    state = json.loads((ROOT / "deploy/runtime/installation.json").read_text(encoding="utf-8"))
    if state.get("image"):
        compose("pull")
    compose("up", "-d", "--no-build" if state.get("image") else "--build", "--wait", "--wait-timeout", "300")
    marker_path.unlink()
    print("恢复完成。请检查商品、历史订单、图片和支付回调。")


def main() -> int:
    parser = argparse.ArgumentParser(description="备份/恢复本站 Docker Compose 数据。需要 Python 3.8+。")
    subparsers = parser.add_subparsers(dest="operation", required=True)
    save = subparsers.add_parser("backup")
    save.add_argument("--output", type=Path, default=ROOT / "backups" / datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ"))
    load = subparsers.add_parser("restore")
    load.add_argument("--source", required=True, type=Path)
    load.add_argument("--confirm-replace-database", action="store_true")
    args = parser.parse_args()
    os.umask(0o077)
    try:
        if not shutil.which("docker"):
            raise ValueError("需要 Docker CLI 和 Compose v2。")
        docker("info", "--format", "{{.ServerVersion}}", stdout=subprocess.DEVNULL)
        if args.operation == "backup":
            backup(args.output.resolve())
        else:
            restore(args.source.resolve(), args.confirm_replace_database)
        return 0
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"操作未完成: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
