#!/bin/sh
set -eu

if ! command -v python3 >/dev/null 2>&1; then
    echo '需要 Python 3.8+，请先安装后再运行。' >&2
    exit 1
fi
if ! python3 -c 'import sys; sys.exit(0 if sys.version_info >= (3, 8) else 1)'; then
    echo '需要 Python 3.8+，请升级 Python 后再运行。' >&2
    exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
    echo '需要 curl，请先安装后再运行。' >&2
    exit 1
fi

installer_tmp="$(mktemp "${TMPDIR:-/tmp}/xioen-install.XXXXXXXX")"
# Only the file created by this invocation is removed; the working directory
# and the user's deployment files are never cleanup targets.
trap 'rm -f -- "$installer_tmp"' 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
    'https://raw.githubusercontent.com/zxgad3wdasaq1e-ai/faka-58/main/deploy/install.py' \
    --output "$installer_tmp"
python3 "$installer_tmp" "$@"
