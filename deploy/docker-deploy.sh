#!/bin/sh
# A Docker-only Ubuntu bootstrap: no Python, jq, Git, Node.js or Go on the host.
set -eu

usage() {
    printf '%s\n' '用法: sh docker-deploy.sh [--domain xioenai.vip | --local] [--image OCI引用]' \
        '       [--admin-user admin] [--subnet 172.29.240.0/24]' \
        '在当前空目录部署预构建镜像；需要 Docker Compose 和 curl。' \
        '默认通过 HTTP :1888 部署，可访问 http://服务器IP:1888；显式 --domain 启用自动 HTTPS。' \
        '仅允许当前目录已有普通文件 docker-deploy.sh，不会覆盖任何既有部署。'
}
die() { printf '%s\n' "部署未完成: $*" >&2; exit 1; }

# Help is available before checking the filesystem, Docker or network tools.
for argument do
    case "$argument" in -h|--help) usage; exit 0 ;; esac
done

domain=xioenai.vip
local_mode=true
image=ghcr.io/zxgad3wdasaq1e-ai/faka-58:latest
admin_user='admin'
subnet=172.29.240.0/24
while [ "$#" -gt 0 ]; do
    case "$1" in
        --local) local_mode=true; shift ;;
        --domain|--image|--admin-user|--subnet)
            [ "$#" -ge 2 ] || die "$1 缺少参数。"
            case "$1" in
                --domain) domain=$2; local_mode=false ;;
                --image) image=$2 ;;
                --admin-user) admin_user=$2 ;;
                --subnet) subnet=$2 ;;
            esac
            shift 2 ;;
        *) die "未知参数 $1；使用 --help 查看用法。" ;;
    esac
done

validate_image() {
    case "$1" in ''|*[!A-Za-z0-9._/@:-]*) return 1 ;; esac
    [ "${#1}" -le 255 ] || return 1
    printf '%s\n' "$1" | grep -Eq '^([a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]{1,5})?/)?[a-z0-9]+(([._]|__|-+)[a-z0-9]+)*(/[a-z0-9]+(([._]|__|-+)[a-z0-9]+)*)*(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})$' || return 1
    case "$1" in *@*) validation_repository=${1%@*} ;; *) validation_repository=${1%:*} ;; esac
    validation_registry=${validation_repository%%/*}
    case "$validation_registry" in
        *:*) validation_port=${validation_registry##*:}; [ "$validation_port" -ge 1 ] && [ "$validation_port" -le 65535 ] || return 1 ;;
    esac
    return 0
}
canonical_repository() {
    case "$1" in
        index.docker.io/*) printf 'docker.io/%s\n' "${1#index.docker.io/}" ;;
        */*)
            first=${1%%/*}
            case "$first" in *.*|*:*|localhost) printf '%s\n' "$1" ;; *) printf 'docker.io/%s\n' "$1" ;; esac ;;
        *) printf 'docker.io/library/%s\n' "$1" ;;
    esac
}
ensure_empty_target() {
    for entry in ./* ./.[!.]* ./..?*; do
        [ -e "$entry" ] || [ -L "$entry" ] || continue
        case "$entry" in
            ./docker-deploy.sh) [ -f "$entry" ] && [ ! -L "$entry" ] || die '安装脚本不能是目录或符号链接。' ;;
            *) die '当前目录非空，已拒绝修改。请在新建的空目录中运行。' ;;
        esac
    done
}

command -v docker >/dev/null 2>&1 || die '需要 Docker Engine 和 Compose v2；安装器不会自动安装系统软件。'
command -v curl >/dev/null 2>&1 || die '需要 curl。'
validate_image "$image" || die '--image 必须是带 tag 或 sha256 摘要的合法 OCI 引用，不能含变量、空白或 shell 字符。'
docker compose version >/dev/null 2>&1 || die 'Docker Compose v2 不可用。'
docker info --format '{{.ServerVersion}}' >/dev/null 2>&1 || die 'Docker daemon 不可用，或当前账号没有 Docker 权限。'
root=$(pwd -P)
case "$root" in *:*) die '安装路径不能包含冒号。' ;; esac
ensure_empty_target

case "$image" in
    *@*) repository=${image%@*}; requested_digest=${image##*@} ;;
    *) repository=${image%:*}; requested_digest= ;;
esac
repository=$(canonical_repository "$repository")
docker pull "$image" || die '镜像拉取失败。GHCR 首次发布后需将 Packages 设为 public，或先执行 docker login ghcr.io；请确认 tag 已发布。'
# %q escapes label newlines before line parsing, preventing a label from
# injecting an apparent revision or digest into another field.
metadata=$(docker image inspect "$image" --format '{{printf "%q\n" (index .Config.Labels "org.opencontainers.image.source")}}{{printf "%q\n" (index .Config.Labels "org.opencontainers.image.revision")}}{{range .RepoDigests}}{{println .}}{{end}}') || die '无法读取镜像 OCI 元数据。'
source_label=$(printf '%s\n' "$metadata" | sed -n '1p')
revision_label=$(printf '%s\n' "$metadata" | sed -n '2p')
[ "$source_label" = '"https://github.com/zxgad3wdasaq1e-ai/faka-58"' ] || die '镜像 OCI source 与指定源码仓库不一致。'
printf '%s\n' "$revision_label" | grep -Eq '^"[0-9a-fA-F]{40}"$' || die '镜像 OCI revision 必须是完整的 40 位提交 SHA。'
revision=$(printf '%s' "$revision_label" | tr -d '"' | tr 'A-F' 'a-f')
digests=$(printf '%s\n' "$metadata" | sed '1,2d')
pinned_image=
while IFS= read -r candidate; do
    case "$candidate" in *@sha256:*) ;; *) continue ;; esac
    validate_image "$candidate" || continue
    candidate_repository=$(canonical_repository "${candidate%@*}")
    [ "$candidate_repository" = "$repository" ] || continue
    candidate_digest=${candidate##*@}
    [ -z "$requested_digest" ] || [ "$candidate_digest" = "$requested_digest" ] || continue
    pinned_image="$repository@$candidate_digest"
    break
done <<EOF
$digests
EOF
[ -n "$pinned_image" ] || die 'RepoDigests 中没有与所选镜像仓库及摘要一致的不可变引用。'

temp_base=$(CDPATH='' cd "${TMPDIR:-/tmp}" && pwd -P) || die '临时目录不可用。'
staging=$(mktemp -d "$temp_base/xioen-deploy.XXXXXXXX") || die '无法创建临时下载目录。'
cleanup() {
    # This path is returned by mktemp under our checked, absolute temp base.
    case "$staging" in "$temp_base"/xioen-deploy.*) rm -rf -- "$staging" ;; esac
}
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir "$staging/deploy"
files='compose.yml deploy/Caddyfile deploy/postgres-init.sh deploy/start.sh deploy/backup.sh LICENSE README.md'
for file in $files; do
    url="https://raw.githubusercontent.com/zxgad3wdasaq1e-ai/faka-58/$revision/$file"
    effective_url=$(curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' --max-time 60 --output "$staging/$file" --write-out '%{url_effective}' "$url") || die "下载 $file 失败，尚未初始化配置。"
    [ "$effective_url" = "$url" ] || die "下载 $file 被重定向到其他地址，已拒绝安装。"
    [ -s "$staging/$file" ] || die "下载的 $file 为空，已拒绝安装。"
done

# Publish only after the complete fixed allowlist has downloaded successfully.
ensure_empty_target
mkdir -m 755 deploy || die '无法创建 deploy 目录；没有覆盖任何已有文件。'
for file in $files; do
    [ ! -e "$root/$file" ] && [ ! -L "$root/$file" ] || die "目标 $file 已存在，拒绝覆盖。"
    (set -C; cat "$staging/$file" >"$root/$file") || die "复制 $file 失败；没有运行初始化，请保留既有私有配置。"
    chmod 644 "$root/$file"
done

set -- docker run --rm --network none --user "$(id -u):$(id -g)" \
    --volume "$root:/workspace" --entrypoint /app/storectl "$pinned_image" \
    init --root /workspace --image "$pinned_image" --admin-user "$admin_user" --subnet "$subnet"
if [ "$local_mode" = true ]; then set -- "$@" --local; else set -- "$@" --domain "$domain"; fi
"$@" || die '私有配置初始化未完成。文件已保留，检查错误后重试；不要覆盖已有密钥。'
sh "$root/deploy/start.sh" --seed || die '服务尚未完成启动。修复错误后运行 sh deploy/start.sh --seed 继续，无需重新安装。'
