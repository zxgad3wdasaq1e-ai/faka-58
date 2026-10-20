#!/usr/bin/env python3
"""Install a prebuilt store image and its matching runtime files without Git."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import urllib.request


ROOT = Path.cwd().resolve()
SOURCE = "https://github.com/zxgad3wdasaq1e-ai/faka-58"
RAW_BASE = "https://raw.githubusercontent.com/zxgad3wdasaq1e-ai/faka-58"
DEFAULT_IMAGE = "ghcr.io/zxgad3wdasaq1e-ai/faka-58:latest"
RUNTIME_FILES = (
    "compose.yml",
    "scripts/setup.py",
    "scripts/seed-catalog.py",
    "deploy/Caddyfile",
    "deploy/postgres-init.sh",
    "deploy/backup.py",
    "LICENSE",
    "README.md",
)
ALLOWED_INSTALLER_FILES = frozenset(("docker-deploy.sh", "install.py"))
MAX_DOWNLOAD_BYTES = 4 * 1024 * 1024
REVISION = re.compile(r"[0-9a-fA-F]{40}")
DIGEST = re.compile(r"sha256:[0-9a-f]{64}")
REPOSITORY = re.compile(
    r"[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*"
    r"(?::[0-9]{1,5})?"
    r"(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*"
)
TAG = re.compile(r"[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}")


class InstallError(RuntimeError):
    pass


def image_repository(image: str) -> str:
    """Validate a tagged/digested reference and return its repository spelling."""
    if not isinstance(image, str) or not image or len(image) > 255 or any(character.isspace() for character in image):
        raise InstallError("--image 需要带 tag 或 sha256 摘要的 OCI 镜像引用。")
    if "@" in image:
        repository, digest = image.split("@", 1)
        if not DIGEST.fullmatch(digest):
            raise InstallError("镜像摘要必须是 sha256 和 64 位小写十六进制字符。")
    else:
        separator = image.rfind(":")
        if separator <= image.rfind("/") or separator < 0 or not TAG.fullmatch(image[separator + 1:]):
            raise InstallError("--image 必须包含显式 tag 或 sha256 摘要。")
        repository = image[:separator]
    if not REPOSITORY.fullmatch(repository):
        raise InstallError("镜像仓库名称无效，不能包含协议、变量、空白或 shell 字符。")
    first = repository.split("/", 1)[0]
    if ":" in first:
        port = int(first.rsplit(":", 1)[1])
        if not 1 <= port <= 65535:
            raise InstallError("镜像仓库端口无效。")
    return repository


def canonical_repository(repository: str) -> str:
    first, separator, remainder = repository.partition("/")
    if not separator:
        return "docker.io/library/" + repository
    if "." not in first and ":" not in first and first != "localhost":
        return "docker.io/" + repository
    if first == "index.docker.io":
        return "docker.io/" + remainder
    return repository


def ensure_empty_target(root: Path) -> None:
    for entry in root.iterdir():
        if entry.name not in ALLOWED_INSTALLER_FILES or not stat.S_ISREG(entry.lstat().st_mode):
            raise InstallError("安装目录非空，已拒绝改动现有文件。请新建空目录再运行；仅允许 docker-deploy.sh/install.py 两个安装脚本。")


def check_docker(root: Path) -> None:
    if not shutil.which("docker"):
        raise InstallError("未找到 Docker。请先安装 Docker Engine 和 Compose v2；安装器不会自动安装系统软件。")
    try:
        subprocess.run(["docker", "compose", "version"], cwd=root, check=True, capture_output=True, text=True)
        subprocess.run(["docker", "info", "--format", "{{.ServerVersion}}"], cwd=root, check=True, capture_output=True, text=True)
    except subprocess.CalledProcessError as error:
        raise InstallError("Docker daemon 或 Compose 不可用。请启动 Docker，并确认当前账号可以执行 docker compose。") from error


def pull_and_inspect(image: str, root: Path) -> tuple[str, str]:
    repository = canonical_repository(image_repository(image))
    try:
        subprocess.run(["docker", "pull", image], cwd=root, check=True)
        result = subprocess.run(
            ["docker", "image", "inspect", image, "--format", "{{json .}}"],
            cwd=root, check=True, capture_output=True, text=True,
        )
    except subprocess.CalledProcessError as error:
        raise InstallError("镜像拉取/读取失败。GHCR 首次发布后需将 Packages 可见性设为 public，或先执行 docker login ghcr.io；同时检查镜像 tag 是否已发布。") from error
    try:
        metadata = json.loads(result.stdout)
        labels = metadata["Config"]["Labels"]
        revision = labels["org.opencontainers.image.revision"]
        source = labels["org.opencontainers.image.source"]
    except (ValueError, TypeError, KeyError) as error:
        raise InstallError("镜像缺少有效的 OCI source/revision 标签，已拒绝部署。") from error
    if source != SOURCE or not isinstance(revision, str) or not REVISION.fullmatch(revision):
        raise InstallError("镜像必须来自指定源码仓库，并包含 40 位提交 SHA 的 OCI revision 标签。")

    candidates = metadata.get("RepoDigests", [])
    if not isinstance(candidates, list):
        raise InstallError("镜像没有可验证的 RepoDigests，已拒绝使用可变 tag 部署。")
    matches = []
    for candidate in candidates:
        if not isinstance(candidate, str):
            continue
        try:
            candidate_repository = image_repository(candidate)
        except InstallError:
            continue
        if "@" in candidate and canonical_repository(candidate_repository) == repository:
            matches.append(repository + "@" + candidate.split("@", 1)[1])
    if "@" in image:
        requested = repository + "@" + image.split("@", 1)[1]
        matches = [candidate for candidate in matches if candidate == requested]
    if not matches:
        raise InstallError("镜像 RepoDigests 中没有与所选仓库一致的 sha256 引用，已拒绝部署。")
    return revision.lower(), matches[0]


def download_runtime_file(revision: str, relative: str, staging: Path) -> None:
    if not REVISION.fullmatch(revision) or relative not in RUNTIME_FILES:
        raise InstallError("拒绝下载不在固定清单中的运行文件或未固定提交的源码。")
    url = f"{RAW_BASE}/{revision}/{relative}"
    request = urllib.request.Request(url, headers={"User-Agent": "xioen-store-installer/1"})
    with urllib.request.urlopen(request, timeout=30) as response:
        if response.geturl() != url or response.status != 200:
            raise InstallError("运行文件下载被重定向或返回异常状态，已拒绝安装。")
        contents = response.read(MAX_DOWNLOAD_BYTES + 1)
    if not contents or len(contents) > MAX_DOWNLOAD_BYTES:
        raise InstallError("运行文件为空或超过下载大小限制。")
    try:
        contents.decode("utf-8")
    except UnicodeDecodeError as error:
        raise InstallError("运行文件不是有效 UTF-8 文本。") from error
    destination = staging / relative
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_bytes(contents)


def publish_runtime(staging: Path, root: Path) -> None:
    """Publish only fully downloaded files, with exclusive creation and rollback."""
    ensure_empty_target(root)
    created_files = []
    created_directories = []
    try:
        for relative in RUNTIME_FILES:
            destination = root / relative
            if not destination.resolve().is_relative_to(root):
                raise InstallError("运行文件路径超出安装目录，已拒绝写入。")
            if not destination.parent.exists():
                destination.parent.mkdir(mode=0o755)
                created_directories.append(destination.parent)
            if destination.parent.is_symlink():
                raise InstallError("运行文件父目录不能是符号链接。")
            flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0)
            descriptor = os.open(destination, flags, 0o644)
            created_files.append(destination)
            with os.fdopen(descriptor, "wb") as handle:
                handle.write((staging / relative).read_bytes())
            # Public runtime scripts must remain readable by PostgreSQL's uid.
            destination.chmod(0o644)
    except BaseException:
        for path in reversed(created_files):
            path.unlink()
        for directory in reversed(created_directories):
            try:
                directory.rmdir()
            except OSError:
                pass  # Never recursively remove user-created files.
        raise


def install(args: argparse.Namespace, root: Path | None = None) -> str:
    root = ROOT if root is None else root.resolve()
    image_repository(args.image)
    if not args.local and not re.fullmatch(r"(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}", args.domain):
        raise InstallError("--domain 需要填写域名，不含协议、端口或路径。")
    check_docker(root)
    ensure_empty_target(root)
    revision, pinned_image = pull_and_inspect(args.image, root)
    with tempfile.TemporaryDirectory(prefix="xioen-runtime-") as temporary:
        staging = Path(temporary)
        for relative in RUNTIME_FILES:
            download_runtime_file(revision, relative, staging)
        publish_runtime(staging, root)
    command = [sys.executable, str(root / "scripts/setup.py")]
    command += ["--local"] if args.local else ["--domain", args.domain]
    command += ["--image", pinned_image, "--start", "--seed"]
    print(f"运行文件已固定到提交 {revision}，使用同仓库 sha256 镜像部署。")
    try:
        subprocess.run(command, cwd=root, check=True)
    except subprocess.CalledProcessError as error:
        raise InstallError("服务初始化未完成，文件已保留。检查 Docker 日志后，在此目录运行 python3 scripts/setup.py --start --seed 继续；不要重新覆盖密钥。") from error
    return pinned_image


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="在当前空目录部署预构建商城镜像；无需 Git、Node.js 或 Go。")
    parser.add_argument("--domain", default="xioenai.vip")
    parser.add_argument("--local", action="store_true", help="使用 http://localhost:8080，不申请证书")
    parser.add_argument("--image", default=DEFAULT_IMAGE, help="预构建镜像 tag 或 digest")
    args = parser.parse_args(argv)
    if sys.version_info < (3, 9):
        print("需要 Python 3.9+。", file=sys.stderr)
        return 1
    try:
        install(args)
        return 0
    except (InstallError, OSError, ValueError) as error:
        print(f"部署未完成: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
