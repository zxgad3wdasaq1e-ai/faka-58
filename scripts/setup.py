#!/usr/bin/env python3
"""Generate private deployment files, then optionally build/start and seed the store."""
from __future__ import annotations

import argparse
import ipaddress
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request


ROOT = Path(__file__).resolve().parents[1]

# Use an explicit tag/digest so an installer never silently selects ``latest``.
# Repository components follow the OCI/Docker distribution reference grammar;
# shell, Compose interpolation, URL schemes and whitespace are not accepted.
IMAGE_REFERENCE = re.compile(
    r"(?=.{1,255}$)"
    r"(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*(?::[0-9]{1,5})?/)?"
    r"[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*"
    r"(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*"
    r"(?::[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})"
)


def validate_image(image: str) -> str:
    if not isinstance(image, str) or not IMAGE_REFERENCE.fullmatch(image):
        raise ValueError("--image 必须是带显式 tag 或 sha256 摘要的 OCI 镜像引用，例如 ghcr.io/owner/store:v1；不能含空白、协议或变量。")
    registry = image.partition("/")[0]
    if "/" in image and ":" in registry and not 1 <= int(registry.rpartition(":")[2]) <= 65535:
        raise ValueError("--image 的镜像仓库端口需在 1–65535 之间。")
    return image


def private_write(path: Path, contents: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(contents)


def random_secret() -> str:
    return secrets.token_hex(32)


def initialize(args: argparse.Namespace) -> dict:
    requested_image = getattr(args, "image", None)
    if requested_image is not None:
        validate_image(requested_image)
    config_path = ROOT / "config.yml"
    state_path = ROOT / "deploy/runtime/installation.json"
    existing = [config_path, ROOT / ".env", state_path]
    if all(path.is_file() for path in existing):
        state = json.loads(state_path.read_text(encoding="utf-8"))
        if requested_image is not None and requested_image != state.get("image", ""):
            raise ValueError("--image 与既有安装模式或镜像不一致；脚本不会切换源码/镜像模式或覆盖配置。请沿用现有配置。")
        print("沿用现有部署配置和密钥；没有覆盖或轮换。")
        return state
    if any(path.exists() for path in existing):
        raise ValueError("发现不完整的既有部署配置。请先恢复 config.yml、.env 和 deploy/runtime/installation.json；脚本不会覆盖密钥。")

    domain = args.domain.strip().lower().rstrip(".")
    if not args.local and not re.fullmatch(r"(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}", domain):
        raise ValueError("--domain 需要填写域名，不含协议、端口或路径。")
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{2,31}", args.admin_user):
        raise ValueError("管理员用户名需为 3–32 位字母、数字、下划线或连字符。")
    subnet = ipaddress.ip_network(args.subnet)
    if subnet.version != 4 or not subnet.is_private or subnet.prefixlen != 24:
        raise ValueError("--subnet 需要一个私有 IPv4 /24 网段。")
    proxy_ip = str(subnet.network_address + 10)
    storage = args.storage
    if requested_image and storage == "minio":
        raise ValueError("--image 快速部署不支持 --storage minio：MinIO 仍需源码构建。请使用默认持久化本地图床，或省略 --image 从源码构建。")
    address = "http://localhost" if args.local else f"https://{domain}"
    origin = "http://localhost:8080" if args.local else address
    admin_path = "/console-" + secrets.token_hex(6)
    admin_password = "Aa9_" + secrets.token_urlsafe(24)
    pg_password, redis_password = random_secret(), random_secret()
    root_user = "root" + secrets.token_hex(6)
    root_password, upload_password = random_secret(), random_secret()
    upload_user = "upload" + secrets.token_hex(6)

    config = {
        "app": {"secret_key": random_secret(), "totp_issuer": "XIOEN AI"},
        "server": {"host": "0.0.0.0", "port": 8080, "mode": "release", "trusted_proxies": [proxy_ip + "/32"]},
        "database": {
            "driver": "postgres",
            "dsn": f"host=postgres user=getgpt password={pg_password} dbname=getgpt port=5432 sslmode=disable TimeZone=Asia/Shanghai",
            "pool": {"max_open_conns": 20, "max_idle_conns": 5, "conn_max_lifetime_seconds": 1800, "conn_max_idle_time_seconds": 300},
        },
        "jwt": {"secret": random_secret(), "expire_hours": 12},
        "user_jwt": {"secret": random_secret(), "expire_hours": 24, "remember_me_expire_hours": 168},
        "bootstrap": {"default_admin_username": args.admin_user, "default_admin_password": admin_password},
        "redis": {"enabled": True, "host": "redis", "port": 6379, "password": redis_password, "db": 0, "prefix": "xioen"},
        "queue": {"enabled": True, "host": "redis", "port": 6379, "password": redis_password, "db": 1, "concurrency": 10, "queues": {"default": 10, "critical": 5}},
        "upload": {
            "storage": "s3" if storage == "minio" else "local",
            "max_size": 10485760,
            "allowed_types": ["image/jpeg", "image/png", "image/gif", "image/webp"],
            "allowed_extensions": [".jpg", ".jpeg", ".png", ".gif", ".webp"],
            "max_width": 4096, "max_height": 4096,
        },
        "cors": {"allowed_origins": [origin], "allowed_methods": ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"], "allowed_headers": ["Content-Type", "Authorization", "X-Requested-With", "X-CSRF-Token"], "allow_credentials": True, "max_age": 600},
        "email": {"enabled": False, "host": "", "port": 587, "username": "", "password": "", "from": "", "from_name": "XIOEN AI", "use_tls": True, "use_ssl": False},
        "telegram_auth": {"enabled": False},
        "google_auth": {"enabled": False},
        "reseller": {"enabled": False, "main_hosts": ["localhost" if args.local else domain], "trusted_forwarded_host": False},
        "web": {"admin_path": admin_path},
    }
    if storage == "minio":
        config["upload"]["s3"] = {"endpoint": "http://minio:9000", "region": "us-east-1", "bucket": "storefront-media", "access_key_id": upload_user, "secret_access_key": upload_password, "public_base_url": origin + "/media", "key_prefix": "", "timeout_seconds": 30}

    secret_dir = ROOT / "deploy/secrets"
    secret_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
    secret_dir.chmod(0o700)
    for name, value in {
        "postgres_password": pg_password, "postgres_admin_password": random_secret(), "redis_password": redis_password,
        "minio_root_user": root_user, "minio_root_password": root_password,
        "minio_app_user": upload_user, "minio_app_password": upload_password,
    }.items():
        private_write(secret_dir / name, value + "\n")
        if name.startswith("postgres_"):
            # Compose binds local secret files without uid remapping. PostgreSQL
            # initializes as uid 999; the parent directory stays host-private 0700.
            (secret_dir / name).chmod(0o444)

    env = ["COMPOSE_PROJECT_NAME=getgpt", f"SITE_ADDRESS={address}", f"HTTP_PORT={8080 if args.local else 80}", f"HTTPS_PORT={8443 if args.local else 443}", f"APP_SUBNET={subnet}", f"CADDY_IP={proxy_ip}", f"APP_IMAGE={requested_image or 'getgpt-store:local'}", "APP_VERSION=v1.0.0-xioen.1"]
    if storage == "minio":
        env += ["COMPOSE_PATH_SEPARATOR=|", "COMPOSE_FILE=compose.yml|compose.minio.yml", "COMPOSE_PROFILES=storage"]
    else:
        env += ["COMPOSE_FILE=compose.yml"]
    private_write(ROOT / ".env", "\n".join(env) + "\n")
    # JSON is a valid YAML document, so no third-party YAML package is needed.
    private_write(config_path, json.dumps(config, ensure_ascii=False, indent=2) + "\n")
    caddy = (ROOT / "deploy/Caddyfile").read_text(encoding="utf-8")
    if storage == "minio":
        caddy = caddy.replace("    reverse_proxy app:8080", "    handle_path /media/* {\n        @read method GET HEAD\n        handle @read {\n            rewrite * /storefront-media{uri}\n            reverse_proxy minio:9000\n        }\n        respond 405\n    }\n    handle {\n        reverse_proxy app:8080\n    }")
    private_write(ROOT / "deploy/runtime/Caddyfile", caddy)
    state = {"origin": origin, "admin_path": admin_path, "admin_user": args.admin_user, "storage": storage, "local": args.local, "image": requested_image or ""}
    private_write(state_path, json.dumps(state, ensure_ascii=False, indent=2) + "\n")
    private_write(secret_dir / "initial-admin.txt", f"后台: {origin}{admin_path}\n用户名: {args.admin_user}\n首次密码: {admin_password}\n" + (f"MinIO 管理台: http://127.0.0.1:9001\nMinIO 用户名: {root_user}\nMinIO 密码: {root_password}\n" if storage == "minio" else "") + "首次登录后修改密码并启用 2FA。此文件含密钥，不得提交、分享或放入公开目录。\n")
    print("已生成私有配置。初始管理员凭据仅写入 deploy/secrets/initial-admin.txt。")
    return state


def run(*command: str, **kwargs) -> subprocess.CompletedProcess:
    return subprocess.run(command, cwd=ROOT, check=True, **kwargs)


def start(state: dict, seed: bool) -> None:
    if not shutil.which("docker"):
        raise ValueError("未找到 Docker CLI；配置已生成。安装 Docker Engine / Desktop 和 Compose v2 后重新运行 --start。")
    run("docker", "compose", "version")
    if state.get("image"):
        run("docker", "compose", "pull")
    run("docker", "compose", "up", "-d", "--no-build" if state.get("image") else "--build", "--wait", "--wait-timeout", "300")
    if seed:
        script = ROOT / "scripts/seed-catalog.py"
        if not script.is_file():
            raise ValueError("缺少 scripts/seed-catalog.py，请完成代码更新后再运行 --start --seed。")
        print("等待公开站点 HTTPS/HTTP 健康检查就绪（最多 120 秒）…")
        deadline = time.monotonic() + 120
        last_error = ""
        while True:
            try:
                with urllib.request.urlopen(state["origin"] + "/health", timeout=5) as response:
                    if response.status == 200:
                        break
            except (OSError, urllib.error.URLError) as error:
                last_error = str(error)
            if time.monotonic() >= deadline:
                raise ValueError("公开站点尚未就绪；请检查 DNS、80/443 和 Caddy 证书日志后重试 --start --seed。" + last_error)
            time.sleep(2)
        # No bootstrap password is put on a process command line;
        # seed-catalog reads the private config file.
        run(sys.executable, str(script), "--base-url", state["origin"])


def main() -> int:
    parser = argparse.ArgumentParser(description="生成安全配置；可选启动 Docker 全栈服务和导入商品目录。")
    parser.add_argument("--domain", default="xioenai.vip", help="正式域名（默认 xioenai.vip）")
    parser.add_argument("--local", action="store_true", help="本地 HTTP :8080，不申请证书")
    parser.add_argument("--storage", choices=["local", "minio"], default="local", help="图床（默认持久化 uploads）")
    parser.add_argument("--image", help="拉取已构建的 OCI 镜像（显式 tag 或 sha256 摘要）；省略则保持源码构建")
    parser.add_argument("--admin-user", default="admin")
    parser.add_argument("--subnet", default="172.29.240.0/24", help="Docker 私有网段；与宿主机已有网络冲突时更换")
    parser.add_argument("--start", action="store_true", help="构建镜像、启动依赖并等待健康检查")
    parser.add_argument("--seed", action="store_true", help="服务启动后幂等导入品牌和示例商品（需要 --start）")
    args = parser.parse_args()
    if args.seed and not args.start:
        parser.error("--seed 需要同时指定 --start")
    try:
        state = initialize(args)
        if args.start:
            start(state, args.seed)
        print(f"店面: {state['origin']}")
        print(f"后台: {state['origin']}{state['admin_path']}")
        if not args.start:
            print("下一步: python3 scripts/setup.py --start --seed")
        return 0
    except (ValueError, OSError, subprocess.CalledProcessError, json.JSONDecodeError) as error:
        print(f"初始化未完成: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
