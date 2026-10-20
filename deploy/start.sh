#!/bin/sh
# Start/retry a Docker-only installation without sourcing its private .env.
set -eu

usage() { printf '%s\n' '用法: sh deploy/start.sh [--seed]' '沿用已有配置，拉取并启动服务；--seed 幂等初始化品牌及商品。'; }
die() { printf '%s\n' "启动未完成: $*" >&2; exit 1; }
for argument do case "$argument" in -h|--help) usage; exit 0 ;; esac; done
seed=false
while [ "$#" -gt 0 ]; do
    case "$1" in --seed) seed=true; shift ;; *) die "未知参数 $1。" ;; esac
done

root=$(CDPATH='' cd "$(dirname "$0")/.." && pwd -P) || die '无法定位安装目录。'
case "$root" in *:*) die '安装路径不能包含冒号。' ;; esac
cd "$root"
for file in .env config.yml deploy/runtime/installation.json; do
    [ -f "$file" ] && [ ! -L "$file" ] || die "缺少正常配置文件 $file；请先完成初始化。"
done
command -v docker >/dev/null 2>&1 || die '需要 Docker Engine 和 Compose v2。'
command -v curl >/dev/null 2>&1 || die '需要 curl。'

# Read this one scalar only. Never execute .env as shell code.
image=$(sed -n 's/^APP_IMAGE=//p' .env)
case "$image" in ''|*[!A-Za-z0-9._/@:-]*) die '.env 的 APP_IMAGE 非法，拒绝执行。' ;; esac
[ "${#image}" -le 255 ] || die 'APP_IMAGE 过长。'
printf '%s\n' "$image" | grep -Eq '^([a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]{1,5})?/)?[a-z0-9]+(([._]|__|-+)[a-z0-9]+)*(/[a-z0-9]+(([._]|__|-+)[a-z0-9]+)*)*(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}|@sha256:[a-f0-9]{64})$' || die 'APP_IMAGE 不是合法 OCI 引用。'
case "$image" in *@*) image_repository=${image%@*} ;; *) image_repository=${image%:*} ;; esac
image_registry=${image_repository%%/*}
case "$image_registry" in
    *:*) image_port=${image_registry##*:}; [ "$image_port" -ge 1 ] && [ "$image_port" -le 65535 ] || die 'APP_IMAGE 仓库端口无效。' ;;
esac
docker compose version >/dev/null 2>&1 || die 'Docker Compose v2 不可用。'
docker compose pull || die '镜像拉取失败，请检查网络或 GHCR public/login 设置。'
docker compose up -d --no-build --wait --wait-timeout 300 || die '服务未通过健康检查；使用 docker compose logs 排查后重试本脚本。'
origin=$(docker run --rm --network none --user "$(id -u):$(id -g)" --volume "$root:/workspace:ro" --entrypoint /app/storectl "$image" info --root /workspace --field origin) || die '无法读取安装状态中的站点地址。'
case "$origin" in
    http://localhost:1888|http://127.0.0.1:1888|http://localhost:8080|http://127.0.0.1:8080)
        public_port=${origin##*:}
        health_hint="HTTP 站点未就绪；请检查 Docker/Caddy 日志、服务器 $public_port 端口及防火墙后重试。" ;;
    https://*)
        hostname=${origin#https://}
        case "$hostname" in ''|*[!A-Za-z0-9.-]*) die '安装状态中的公开域名无效。' ;; esac
        health_hint='HTTPS 站点未就绪；请检查 DNS、80/443 和 Caddy 证书日志后重试。' ;;
    *) die '安装状态中的地址必须是 HTTPS 域名或本机 HTTP :1888（兼容旧版 :8080）。' ;;
esac

printf '%s\n' '等待公开站点健康检查就绪（最多 120 秒）…'
deadline=$(($(date +%s) + 120))
while :; do
    remaining=$((deadline - $(date +%s)))
    [ "$remaining" -gt 0 ] || die "$health_hint"
    request_timeout=5
    [ "$remaining" -ge 5 ] || request_timeout=$remaining
    status=$(curl --fail --silent --show-error --proto '=http,https' --max-time "$request_timeout" --output /dev/null --write-out '%{http_code}' "$origin/health") || status=
    [ "$status" = 200 ] && break
    remaining=$((deadline - $(date +%s)))
    [ "$remaining" -gt 0 ] || die "$health_hint"
    pause=2
    [ "$remaining" -ge 2 ] || pause=$remaining
    sleep "$pause"
done
if [ "$seed" = true ]; then
    docker compose exec -T app /app/storectl seed --root /app --base-url http://127.0.0.1:8080 || die '商品初始化未完成；修复后使用 --seed 重试。'
fi
printf '%s\n' "店面: $origin" "后台地址与初始凭据保存在: $root/deploy/secrets/initial-admin.txt"
case "$origin" in
    http://localhost:1888|http://127.0.0.1:1888)
        printf '%s\n' '远程访问: http://服务器IP:1888（请放行服务器 1888/TCP 端口）。' ;;
esac
