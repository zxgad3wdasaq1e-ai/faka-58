#!/bin/sh
# Shell-only backups for the pinned-image deployment with local uploads.
set -eu
umask 077

ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd -P)
VOLUMES='uploads app_state redis_data caddy_data caddy_config app_logs'
ARTIFACTS='backup.meta database.dump private.tar.gz uploads.tar.gz app_state.tar.gz redis_data.tar.gz caddy_data.tar.gz caddy_config.tar.gz app_logs.tar.gz'
STOPPED=''
SUCCESS=''

die() { printf '备份工具: %s\n' "$*" >&2; exit 1; }
usage() {
    cat <<'EOF'
用法:
  sh deploy/backup.sh backup [--output 目录]
  sh deploy/backup.sh restore --source 目录 --confirm-replace-database

仅支持固定镜像摘要、默认 Compose 和本地 uploads 的安装。
旧 Python 格式、源码部署或 MinIO 请使用原 deploy/backup.py。
备份包含数据库、图片、队列、证书和全部密钥；请加密保存到异机。
恢复仅用于已有公开部署文件、没有私有配置/容器/数据的新目录。
恢复失败不会删除数据卷，也不提供自动断点续恢复。
EOF
}

validate_project() {
    case "$PROJECT" in ''|[!a-z0-9]*|*[!a-z0-9_-]*) die 'Compose 项目名无效。';; esac
}
validate_image() {
    case "$IMAGE" in *@sha256:*) ;; *) die '仅支持固定 sha256 镜像的安装；源码/tag 部署请使用原备份工具。';; esac
    repository=${IMAGE%@sha256:*}
    checksum=${IMAGE##*@sha256:}
    case "$repository" in ''|[!a-z0-9]*|*[!a-z0-9./:_-]*) die '镜像仓库名称无效。';; esac
    case "$checksum" in *[!a-f0-9]*) die '镜像 sha256 无效。';; esac
    [ "${#checksum}" -eq 64 ] || die '镜像 sha256 长度无效。'
}

# Parse data, never source/eval it. Only the generated quick-install keys are
# accepted, so restored settings cannot select another Compose file or profile.
parse_env() {
    PROJECT='' IMAGE='' seen=''
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in ''|'#'*) continue;; esac
        case "$line" in *=*) ;; *) die '.env 格式无效。';; esac
        key=${line%%=*}
        value=${line#*=}
        case " $seen " in *" $key "*) die '.env 含重复配置项。';; esac
        seen="$seen $key"
        case "$key" in
            COMPOSE_PROJECT_NAME) PROJECT=$value;;
            APP_IMAGE) IMAGE=$value;;
            COMPOSE_FILE) [ "$value" = compose.yml ] || die '自定义 Compose / MinIO 请使用原备份工具。';;
            SITE_ADDRESS|HTTP_PORT|HTTPS_PORT|APP_SUBNET|CADDY_IP|APP_VERSION) ;;
            *) die '.env 含非快速安装配置项；请使用原备份工具。';;
        esac
        case "$value" in *[!A-Za-z0-9_./:@+-]*) die '.env 含变量、引号或不支持的字符。';; esac
    done
    validate_project
    validate_image
    case " $seen " in *' COMPOSE_FILE '*) ;; *) die '.env 缺少 COMPOSE_FILE。';; esac
}

compose() (
    # Do not allow an inherited shell variable to change the backed-up project,
    # image, mount configuration, or public ports.
    unset COMPOSE_FILE COMPOSE_PROJECT_NAME COMPOSE_PROFILES COMPOSE_PATH_SEPARATOR
    unset APP_IMAGE APP_VERSION SITE_ADDRESS HTTP_PORT HTTPS_PORT APP_SUBNET CADDY_IP
    cd "$ROOT"
    docker compose --project-name "$PROJECT" --env-file "$ROOT/.env" -f "$ROOT/compose.yml" "$@"
)

utility() {
    docker run --rm --network none --read-only --cap-drop ALL --cap-add CHOWN \
        --cap-add FOWNER --cap-add DAC_OVERRIDE --security-opt no-new-privileges \
        --tmpfs /tmp:rw,nosuid,nodev,noexec "$@"
}

# These programs are fixed source code; archive contents are never executed.
# Only directories and ordinary files with simple relative names are accepted.
# This deliberately rejects symlinks, hardlinks, devices, control characters,
# duplicate entries, and unusual filenames instead of weakening extraction.
ARCHIVE_CHECK='
check_archive() (
    archive=$1 kind=$2
    names=$(mktemp) types=$(mktemp) normalized=$(mktemp)
    tar -tzf "$archive" > "$names"
    tar -tvzf "$archive" > "$types"
    test -s "$names"
    while IFS= read -r entry; do
        case "$entry" in ""|/*|-*|*[!A-Za-z0-9_./@=:+-]*) echo "归档含不安全路径" >&2; return 1;; esac
        if test "$kind" = private; then
            case "$entry" in .env|config.yml|deploy/secrets|deploy/secrets/|deploy/secrets/*|deploy/runtime|deploy/runtime/|deploy/runtime/*) ;;
                *) echo "私有归档含非配置路径" >&2; return 1;; esac
        fi
        name=${entry#./}; name=${name%/}
        case "/$name/" in *"/../"*|*"/./"*|*"//"*)
            if ! { test "$kind" = volume && test "$entry" = ./; }; then
                echo "归档路径越界" >&2; return 1
            fi;;
        esac
        printf "%s\n" "$name" >> "$normalized"
    done < "$names"
    test -z "$(sort "$normalized" | uniq -d)" || { echo "归档含重复路径" >&2; return 1; }
    while IFS= read -r entry; do
        case "$entry" in -?*|d?*) ;; *) echo "归档含链接或特殊文件" >&2; return 1;; esac
    done < "$types"
    if test "$kind" = private; then
        for required in .env config.yml deploy/secrets/postgres_password deploy/secrets/postgres_admin_password deploy/secrets/redis_password deploy/runtime/Caddyfile deploy/runtime/installation.json; do
            grep -Fx "$required" "$names" >/dev/null || { echo "私有归档缺少必要配置" >&2; return 1; }
            record=$(tar -tvzf "$archive" "$required")
            case "$record" in -?*) ;; *) echo "必要配置不是普通文件" >&2; return 1;; esac
        done
    fi
    rm -f "$names" "$types" "$normalized"
)
'

finish() {
    result=$?
    trap - 0 HUP INT TERM
    set +e
    # Try every originally running service, even if restarting one fails.
    for service in redis app adminer caddy; do
        case " $STOPPED " in
            *" $service "*)
                if ! compose up -d --no-deps --no-build --wait --wait-timeout 120 "$service"; then
                    printf '原服务恢复失败，请手动检查: %s\n' "$service" >&2
                    result=1
                fi;;
        esac
    done
    if [ "$result" -eq 0 ] && [ -n "$SUCCESS" ]; then
        printf '%s\n' "$SUCCESS"
    fi
    exit "$result"
}

check_volume() {
    logical=$1
    physical=${PROJECT}_$logical
    labels=$(docker volume inspect --format '{{ index .Labels "com.docker.compose.project" }}:{{ index .Labels "com.docker.compose.volume" }}' "$physical")
    [ "$labels" = "$PROJECT:$logical" ] || die '数据卷与快速安装项目不匹配。'
}

backup() {
    for path in .env config.yml deploy/secrets deploy/runtime; do
        [ -e "$ROOT/$path" ] && [ ! -L "$ROOT/$path" ] || die '缺少快速安装私有配置，或配置路径为链接。'
    done
    parse_env < "$ROOT/.env"
    active=$(compose ps --status running --services)
    # Normalize the newline-separated service list before membership checks.
    active=$(printf '%s\n' "$active" | tr '\n' ' ')
    case " $active " in *' minio '*|*' minio-init '*) die 'MinIO 请使用原备份工具。';; esac
    case " $active " in *' postgres '*) ;; *) die 'PostgreSQL 尚未运行。';; esac
    pg_id=$(compose ps -q postgres)
    [ -n "$pg_id" ] || die '找不到 PostgreSQL 容器。'
    actual_project=$(docker inspect --format '{{ index .Config.Labels "com.docker.compose.project" }}' "$pg_id")
    [ "$actual_project" = "$PROJECT" ] || die '运行中的 Compose 项目与配置不一致。'
    for logical in postgres_data $VOLUMES; do check_volume "$logical"; done

    case "$OUTPUT" in /*) ;; *) OUTPUT="$ROOT/$OUTPUT";; esac
    output_name=$(basename -- "$OUTPUT")
    case "$output_name" in ''|.|..) die '备份目录名无效。';; esac
    mkdir -p -- "$(dirname -- "$OUTPUT")"
    output_parent=$(CDPATH='' cd -- "$(dirname -- "$OUTPUT")" && pwd -P)
    case "$output_parent/" in "$ROOT/deploy/secrets/"*|"$ROOT/deploy/runtime/"*) die '备份不能放入私有配置目录。';; esac
    OUTPUT=$output_parent/$output_name
    case "$OUTPUT" in *','*) die '备份路径不能含逗号。';; esac
    mkdir -m 700 -- "$OUTPUT" || die '备份目录已存在，拒绝覆盖。'
    host_uid=$(id -u) host_gid=$(id -g)
    printf 'format=faka-shell-v1\nproject=%s\nimage=%s\n' "$PROJECT" "$IMAGE" > "$OUTPUT/backup.meta"
    trap finish 0
    trap 'exit 129' HUP
    trap 'exit 130' INT
    trap 'exit 143' TERM
    for service in caddy app adminer redis; do
        case " $active " in *" $service "*)
            STOPPED="$service $STOPPED"
            compose stop "$service";;
        esac
    done
    compose exec -T postgres pg_dump -U postgres -d getgpt -Fc > "$OUTPUT/database.dump"
    utility --mount "type=bind,source=$ROOT,target=/source,readonly" \
        --mount "type=bind,source=$OUTPUT,target=/backup" alpine:3.22 sh -ec "$ARCHIVE_CHECK"'
        source=$1 output=$2
        for path in .env config.yml deploy/secrets deploy/runtime; do
            test -z "$(find "$source/$path" ! -type f ! -type d -print -quit)" || { echo "私有配置含链接或特殊文件" >&2; exit 1; }
        done
        tar -czf "$output/private.tar.gz" -C "$source" .env config.yml deploy/secrets deploy/runtime
        check_archive "$output/private.tar.gz" private
        chmod 600 "$output/private.tar.gz"
        chown "$3:$4" "$output/private.tar.gz"
    ' backup /source /backup "$host_uid" "$host_gid"
    for logical in $VOLUMES; do
        utility --mount "type=volume,source=${PROJECT}_$logical,target=/source,readonly" \
            --mount "type=bind,source=$OUTPUT,target=/backup" alpine:3.22 sh -ec "$ARCHIVE_CHECK"'
            source=$1 output=$2 name=$3
            test -z "$(find "$source" ! -type f ! -type d -print -quit)" || { echo "数据卷含链接或特殊文件" >&2; exit 1; }
            tar -czf "$output/$name.tar.gz" -C "$source" .
            check_archive "$output/$name.tar.gz" volume
            chmod 600 "$output/$name.tar.gz"
            chown "$4:$5" "$output/$name.tar.gz"
        ' backup /source /backup "$logical" "$host_uid" "$host_gid"
    done
    utility --mount "type=bind,source=$OUTPUT,target=/backup,readonly" alpine:3.22 sh -ec '
        cd "$1"; shift
        sha256sum "$@"
    ' backup /backup $ARTIFACTS > "$OUTPUT/SHA256SUMS"
    SUCCESS="备份完成: $OUTPUT。包含密钥，请加密保存到异机。"
}

restore() {
    [ "$CONFIRMED" = yes ] || die '恢复必须显式添加 --confirm-replace-database，仅用于空配置和空数据卷。'
    [ -d "$SOURCE" ] || die '备份目录不存在。'
    SOURCE=$(CDPATH='' cd -- "$SOURCE" && pwd -P)
    case "$SOURCE" in *','*) die '备份路径不能含逗号。';; esac
    for path in .env config.yml deploy/secrets deploy/runtime; do
        [ ! -e "$ROOT/$path" ] && [ ! -L "$ROOT/$path" ] || die '已有私有配置，拒绝覆盖；请在新目录恢复。'
    done
    [ -f "$SOURCE/SHA256SUMS" ] && [ ! -L "$SOURCE/SHA256SUMS" ] || die '不是 Shell v1 备份；旧格式请使用原备份工具。'
    utility --mount "type=bind,source=$SOURCE,target=/backup,readonly" alpine:3.22 sh -ec "$ARCHIVE_CHECK"'
        output=$1; shift
        for name in SHA256SUMS "$@"; do
            test -f "$output/$name" && test ! -L "$output/$name" || { echo "备份缺文件或含链接" >&2; exit 1; }
        done
        test "$(wc -c < "$output/SHA256SUMS")" -le 4096
        seen=""
        while IFS= read -r line || test -n "$line"; do
            checksum=${line%% *}; rest=${line#"$checksum"}
            test "${#checksum}" -eq 64
            case "$checksum" in *[!a-f0-9]*) exit 1;; esac
            case "$rest" in
                "  "*) name=${rest#"  "};;
                " *"*) name=${rest#" *"};;
                *) echo "校验清单格式无效" >&2; exit 1;;
            esac
            case "$name" in ""|*[!a-z_0-9.]*) echo "校验清单路径无效" >&2; exit 1;; esac
            case " $* " in *" $name "*) ;; *) echo "校验清单含非备份路径" >&2; exit 1;; esac
            case " $seen " in *" $name "*) exit 1;; esac
            seen="$seen $name"
        done < "$output/SHA256SUMS"
        for name in "$@"; do case " $seen " in *" $name "*) ;; *) exit 1;; esac; done
        cd "$output"
        sha256sum -c SHA256SUMS >/dev/null
        test "$(wc -c < backup.meta)" -le 2048
        check_archive private.tar.gz private
        for name in uploads app_state redis_data caddy_data caddy_config app_logs; do check_archive "$name.tar.gz" volume; done
        test "$(tar -xOzf private.tar.gz .env | wc -c)" -le 65536
    ' backup /backup $ARTIFACTS
    format='' meta_project='' meta_image='' meta_seen=''
    while IFS= read -r line || [ -n "$line" ]; do
        key=${line%%=*} value=${line#*=}
        case " $meta_seen " in *" $key "*) die '备份元数据重复。';; esac
        meta_seen="$meta_seen $key"
        case "$key" in format) format=$value;; project) meta_project=$value;; image) meta_image=$value;; *) die '备份元数据无效。';; esac
    done < "$SOURCE/backup.meta"
    [ "$format" = faka-shell-v1 ] || die '不支持该备份格式。'
    env_data=$(utility --mount "type=bind,source=$SOURCE,target=/backup,readonly" alpine:3.22 sh -ec 'tar -xOzf "$1/private.tar.gz" .env' backup /backup)
    parse_env <<EOF
$env_data
EOF
    [ "$PROJECT" = "$meta_project" ] && [ "$IMAGE" = "$meta_image" ] || die '备份元数据与配置不一致。'
    [ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ] || die '目标项目已有容器，拒绝恢复。'
    # Inspect EVERY destination before credentials or any data are restored.
    for logical in postgres_data $VOLUMES; do
        physical=${PROJECT}_$logical
        if docker volume inspect "$physical" >/dev/null 2>&1; then
            check_volume "$logical"
            utility --mount "type=volume,source=$physical,target=/target,readonly" alpine:3.22 sh -ec '
                test -z "$(find "$1" -mindepth 1 -maxdepth 1 -print -quit)" || { echo "目标数据卷非空，拒绝覆盖" >&2; exit 1; }
            ' backup /target
        fi
    done
    host_uid=$(id -u) host_gid=$(id -g)
    utility --mount "type=bind,source=$SOURCE,target=/backup,readonly" \
        --mount "type=bind,source=$ROOT,target=/target" alpine:3.22 sh -ec "$ARCHIVE_CHECK"'
        source=$1 target=$2
        check_archive "$source/private.tar.gz" private
        for path in .env config.yml deploy/secrets deploy/runtime; do test ! -e "$target/$path" && test ! -L "$target/$path"; done
        tar -xozf "$source/private.tar.gz" -C "$target"
        chown -R "$3:$4" "$target/.env" "$target/config.yml" "$target/deploy/secrets" "$target/deploy/runtime"
    ' backup /backup /target "$host_uid" "$host_gid"
    for logical in $VOLUMES; do
        physical=${PROJECT}_$logical
        if ! docker volume inspect "$physical" >/dev/null 2>&1; then
            docker volume create --label "com.docker.compose.project=$PROJECT" --label "com.docker.compose.volume=$logical" "$physical" >/dev/null
        fi
        utility --mount "type=bind,source=$SOURCE,target=/backup,readonly" \
            --mount "type=volume,source=$physical,target=/target" alpine:3.22 sh -ec "$ARCHIVE_CHECK"'
            source=$1 target=$2 name=$3
            check_archive "$source/$name.tar.gz" volume
            test -z "$(find "$target" -mindepth 1 -maxdepth 1 -print -quit)"
            tar -xzf "$source/$name.tar.gz" -C "$target"
        ' backup /backup /target "$logical"
    done
    compose up -d --no-build --wait --wait-timeout 300 postgres
    compose exec -T postgres pg_restore -U postgres -d getgpt --clean --if-exists --exit-on-error --single-transaction < "$SOURCE/database.dump"
    compose pull
    compose up -d --no-build --wait --wait-timeout 300
    printf '恢复完成。请检查商品、历史订单、图片与支付回调。\n'
}

OPERATION=${1:-help}
[ "$#" -eq 0 ] || shift
OUTPUT="backups/$(date -u +%Y%m%dT%H%M%SZ)" SOURCE='' CONFIRMED=no
while [ "$#" -gt 0 ]; do
    case "$1" in
        --output) [ "$#" -ge 2 ] || die '--output 缺少目录。'; OUTPUT=$2; shift 2;;
        --source) [ "$#" -ge 2 ] || die '--source 缺少目录。'; SOURCE=$2; shift 2;;
        --confirm-replace-database) CONFIRMED=yes; shift;;
        --help|-h) usage; exit 0;;
        *) die '未知参数，请运行 --help。';;
    esac
done
case "$OPERATION" in help|--help|-h) usage; exit 0;; backup|restore) ;; *) die '操作必须是 backup 或 restore。';; esac
[ "$OPERATION" != restore ] || [ -n "$SOURCE" ] || die 'restore 需要 --source。'
[ -f "$ROOT/compose.yml" ] && [ ! -L "$ROOT/compose.yml" ] || die '缺少受信任的 compose.yml。'
case "$ROOT" in *','*) die '部署路径不能含逗号。';; esac
command -v docker >/dev/null 2>&1 || die '需要 Docker Engine 和 Compose v2。'
docker compose version >/dev/null
docker info --format '{{.ServerVersion}}' >/dev/null
# Pull the small utility image before stopping the storefront.
docker image inspect alpine:3.22 >/dev/null 2>&1 || docker pull alpine:3.22 >/dev/null
"$OPERATION"
