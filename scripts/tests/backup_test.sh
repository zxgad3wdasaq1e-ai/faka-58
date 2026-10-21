#!/usr/bin/env bash
# Exercise real tar/checksum/shell programs with a Docker daemon stub.
set -Eeuo pipefail
REPOSITORY=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
TEST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/faka-backup-test.XXXXXXXX")
trap 'rm -rf -- "$TEST_TMP"' EXIT
mkdir -p "$TEST_TMP/bin" "$TEST_TMP/volumes" "$TEST_TMP/helper-tmp"
export MOCK_LOG="$TEST_TMP/docker.log" MOCK_VOLUMES="$TEST_TMP/volumes" MOCK_HELPER_TMP="$TEST_TMP/helper-tmp"
export MOCK_ACTIVE='postgres app redis caddy' MOCK_PROJECT=faka-58
export PATH="$TEST_TMP/bin:$PATH"

cat > "$TEST_TMP/bin/docker" <<'DOCKER'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MOCK_LOG"
case "$1" in
    info|pull) exit 0;;
    image) exit 0;;
    inspect) printf '%s\n' "$MOCK_PROJECT"; exit 0;;
    ps) printf '%s' "${MOCK_EXISTING_CONTAINERS:-}"; exit 0;;
    volume)
        operation=$2 name=${!#}
        case "$operation" in
            inspect)
                [[ -d "$MOCK_VOLUMES/$name" ]] || exit 1
                if [[ "$*" == *--format* ]]; then printf '%s:%s\n' "$MOCK_PROJECT" "${name#"${MOCK_PROJECT}_"}"; fi;;
            create) mkdir -p "$MOCK_VOLUMES/$name"; printf '%s\n' "$name";;
            *) exit 88;;
        esac
        exit 0;;
    compose)
        shift
        if [[ "${MOCK_ASSERT_PROXY_MODE_UNSET:-no}" == yes && "${1:-}" == --project-name && "${CADDY_PROXY_MODE+x}" == x ]]; then
            printf 'CADDY_PROXY_MODE leaked into Compose\n' >&2
            exit 89
        fi
        while [[ $# -gt 0 ]]; do
            case "$1" in --project-name|--env-file|-f) shift 2;; *) break;; esac
        done
        case "$1" in
            version|pull) exit 0;;
            ps)
                if [[ "$*" == *'-q postgres'* ]]; then printf 'pg-test-id\n'; else printf '%s\n' $MOCK_ACTIVE; fi
                exit 0;;
            stop)
                [[ "${MOCK_FAIL_STOP:-}" != "$2" ]]; exit;;
            up)
                if [[ -n "${MOCK_FAIL_UP:-}" && "$*" == *" ${MOCK_FAIL_UP}" ]]; then exit 17; fi
                exit 0;;
            exec)
                [[ "$4 $5" == 'sh -ec' ]] || exit 88
                POSTGRES_DB=${MOCK_POSTGRES_DB:-faka-58} POSTGRES_USER=${MOCK_POSTGRES_USER:-postgres} sh -ec "$6"
                exit 0;;
            *) exit 88;;
        esac;;
    run)
        shift
        declare -A mounts=()
        while [[ $# -gt 0 ]]; do
            case "$1" in
                --rm|--read-only) shift;;
                --network|--cap-drop|--cap-add|--security-opt|--tmpfs) shift 2;;
                --mount)
                    source='' target='' type=''
                    IFS=, read -ra fields <<< "$2"
                    for field in "${fields[@]}"; do
                        case "$field" in source=*) source=${field#source=};; target=*) target=${field#target=};; type=*) type=${field#type=};; esac
                    done
                    if [[ "$type" == volume ]]; then source="$MOCK_VOLUMES/$source"; fi
                    mounts["$target"]=$source
                    shift 2;;
                alpine:3.22) shift; break;;
                *) exit 88;;
            esac
        done
        [[ "$1 $2" == 'sh -ec' ]] || exit 88
        program=$3 dummy=$4; shift 4
        arguments=()
        for argument in "$@"; do
            if [[ -n "${mounts[$argument]+yes}" ]]; then argument=${mounts[$argument]}; fi
            arguments+=("$argument")
        done
        flags=-ec
        [[ "${MOCK_TRACE:-no}" != yes ]] || flags=-exc
        TMPDIR="$MOCK_HELPER_TMP" sh "$flags" "$program" "$dummy" "${arguments[@]}"
        exit;;
    *) exit 88;;
esac
DOCKER
chmod +x "$TEST_TMP/bin/docker"
cat > "$TEST_TMP/bin/pg_dump" <<'PGDUMP'
#!/usr/bin/env bash
set -euo pipefail
printf 'pg_dump %s\n' "$*" >> "$MOCK_LOG"
[[ "${MOCK_FAIL_DUMP:-no}" != yes ]] || exit 13
printf 'fake-postgres-dump\n'
PGDUMP
cat > "$TEST_TMP/bin/pg_restore" <<'PGRESTORE'
#!/usr/bin/env bash
set -euo pipefail
printf 'pg_restore %s\n' "$*" >> "$MOCK_LOG"
cat > "$MOCK_VOLUMES/restored.dump"
PGRESTORE
chmod +x "$TEST_TMP/bin/pg_dump" "$TEST_TMP/bin/pg_restore"

pass_count=0
pass() { printf 'ok - %s\n' "$1"; pass_count=$((pass_count + 1)); }
fail() { printf 'not ok - %s\n' "$1" >&2; exit 1; }
success() {
    local name=$1; shift
    if "$@" > "$TEST_TMP/output" 2>&1; then pass "$name"; else cat "$TEST_TMP/output" >&2; fail "$name"; fi
}
failure() {
    local name=$1; shift
    if "$@" > "$TEST_TMP/output" 2>&1; then fail "$name"; else pass "$name"; fi
}
contains() { grep -F -- "$2" "$1" > /dev/null || fail "$3"; }
not_contains() { if grep -F -- "$2" "$1" > /dev/null; then fail "$3"; fi; }

make_public() {
    local root=$1
    mkdir -p "$root/deploy"
    cp "$REPOSITORY/deploy/backup.sh" "$root/deploy/backup.sh"
    printf 'services: {}\n' > "$root/compose.yml"
}
make_installation() {
    local root=$1
    make_public "$root"
    mkdir -p "$root/deploy/secrets" "$root/deploy/runtime"
    cat > "$root/.env" <<EOF
COMPOSE_PROJECT_NAME=faka-58
POSTGRES_DB=faka-58
POSTGRES_APP_USER=faka-58
SITE_ADDRESS=http://localhost
HTTP_PORT=8080
HTTPS_PORT=8443
APP_SUBNET=172.29.240.0/24
CADDY_IP=172.29.240.10
CADDY_PROXY_MODE=direct
APP_IMAGE=ghcr.io/example/store@sha256:$(printf 'a%.0s' {1..64})
APP_VERSION=v1.0.0-test
COMPOSE_FILE=compose.yml
EOF
    printf '{}\n' > "$root/config.yml"
    printf '{}\n' > "$root/deploy/runtime/installation.json"
    printf 'localhost { reverse_proxy app:8080 }\n' > "$root/deploy/runtime/Caddyfile"
    for secret in postgres_password postgres_admin_password redis_password; do printf 'fixture-only-password\n' > "$root/deploy/secrets/$secret"; done
    for volume in postgres_data uploads app_state redis_data caddy_data caddy_config app_logs; do
        mkdir -p "$MOCK_VOLUMES/faka-58_$volume"
        printf 'fixture-%s\n' "$volume" > "$MOCK_VOLUMES/faka-58_$volume/data"
    done
}
rehash() {
    local output=$1
    (cd "$output" && sha256sum backup.meta database.dump private.tar.gz uploads.tar.gz app_state.tar.gz redis_data.tar.gz caddy_data.tar.gz caddy_config.tar.gz app_logs.tar.gz > SHA256SUMS)
}

success 'help requires neither configuration nor Docker calls' sh "$REPOSITORY/deploy/backup.sh" --help
[[ ! -e "$MOCK_LOG" ]] || fail 'help invoked Docker'

source_root="$TEST_TMP/source"
make_installation "$source_root"
backup="$TEST_TMP/saved"
success 'backup accepts direct mode without inherited proxy overrides' env CADDY_PROXY_MODE=cloudflare MOCK_ASSERT_PROXY_MODE_UNSET=yes sh "$source_root/deploy/backup.sh" backup --output "$backup"
[[ -f "$backup/SHA256SUMS" ]] || fail 'missing SHA256SUMS'
(cd "$backup" && sha256sum -c SHA256SUMS > /dev/null)
contains "$MOCK_LOG" 'up -d --no-deps --no-build --wait --wait-timeout 120 app' 'app was not resumed'
contains "$MOCK_LOG" 'up -d --no-deps --no-build --wait --wait-timeout 120 caddy' 'caddy was not resumed'
not_contains "$MOCK_LOG" 'up -d --no-deps --no-build --wait --wait-timeout 120 adminer' 'inactive adminer was started'
contains "$MOCK_LOG" 'pg_dump -U postgres -d faka-58 -Fc' 'backup ignored the container database'
success 'backup uses the existing database and administrator identity' env MOCK_POSTGRES_DB=legacy-db MOCK_POSTGRES_USER=legacy-admin sh "$source_root/deploy/backup.sh" backup --output "$TEST_TMP/legacy-database"
contains "$MOCK_LOG" 'pg_dump -U legacy-admin -d legacy-db -Fc' 'backup replaced the existing database identity'
failure 'existing backup directories are never overwritten' sh "$source_root/deploy/backup.sh" backup --output "$backup"

cp "$source_root/.env" "$TEST_TMP/original.env"
sed 's/^CADDY_PROXY_MODE=direct$/CADDY_PROXY_MODE=cloudflare/' "$TEST_TMP/original.env" > "$source_root/.env"
success 'backup accepts cloudflare mode without inherited proxy overrides' env CADDY_PROXY_MODE=direct MOCK_ASSERT_PROXY_MODE_UNSET=yes sh "$source_root/deploy/backup.sh" backup --output "$TEST_TMP/cloudflare"
tar -xOzf "$TEST_TMP/cloudflare/private.tar.gz" .env > "$TEST_TMP/archived.env"
contains "$TEST_TMP/archived.env" 'CADDY_PROXY_MODE=cloudflare' 'backup lost the configured proxy mode'
for mode in '' invalid; do
    sed "s/^CADDY_PROXY_MODE=.*/CADDY_PROXY_MODE=$mode/" "$TEST_TMP/original.env" > "$source_root/.env"
    : > "$MOCK_LOG"
    failure "unsupported proxy mode '$mode' is refused before stopping services" sh "$source_root/deploy/backup.sh" backup --output "$TEST_TMP/invalid-mode"
    contains "$TEST_TMP/output" 'CADDY_PROXY_MODE' 'invalid proxy mode failed for an unrelated reason'
    not_contains "$MOCK_LOG" 'compose --project-name' 'invalid proxy mode reached Compose operations'
done
cp "$TEST_TMP/original.env" "$source_root/.env"

export MOCK_ACTIVE='postgres app'
: > "$MOCK_LOG"
export MOCK_FAIL_DUMP=yes
failure 'database dump failure is propagated' sh "$source_root/deploy/backup.sh" backup --output "$TEST_TMP/failed-dump"
unset MOCK_FAIL_DUMP
contains "$MOCK_LOG" 'up -d --no-deps --no-build --wait --wait-timeout 120 app' 'app was not resumed after failure'
not_contains "$MOCK_LOG" 'up -d --no-deps --no-build --wait --wait-timeout 120 redis' 'inactive redis was started after failure'
not_contains "$MOCK_LOG" 'up -d --no-deps --no-build --wait --wait-timeout 120 caddy' 'inactive caddy was started after failure'
pass 'failure trap resumes only originally active services'

export MOCK_ACTIVE='postgres app redis caddy' MOCK_FAIL_UP=redis
: > "$MOCK_LOG"
failure 'resume failure prevents a successful backup status' sh "$source_root/deploy/backup.sh" backup --output "$TEST_TMP/failed-resume"
unset MOCK_FAIL_UP
contains "$MOCK_LOG" 'up -d --no-deps --no-build --wait --wait-timeout 120 caddy' 'resume failure prevented remaining service recovery'
pass 'resume failure still attempts every originally active service'

printf 'COMPOSE_PROFILES=storage\n' >> "$source_root/.env"
failure 'MinIO/custom profiles are explicitly refused' sh "$source_root/deploy/backup.sh" backup --output "$TEST_TMP/minio"
cp "$TEST_TMP/original.env" "$source_root/.env"
printf 'DANGEROUS=$(touch %s)\n' "$TEST_TMP/executed" >> "$source_root/.env"
failure 'environment files are parsed as data and never sourced' sh "$source_root/deploy/backup.sh" backup --output "$TEST_TMP/injection"
[[ ! -e "$TEST_TMP/executed" ]] || fail '.env was executed'
cp "$TEST_TMP/original.env" "$source_root/.env"

restore_root="$TEST_TMP/restore"
make_public "$restore_root"
failure 'restore requires explicit confirmation' sh "$restore_root/deploy/backup.sh" restore --source "$backup"
failure 'nonempty destination volumes are refused before writing configuration' sh "$restore_root/deploy/backup.sh" restore --source "$backup" --confirm-replace-database
[[ ! -e "$restore_root/.env" ]] || fail 'configuration written before volume preflight'
printf 'existing-config\n' > "$restore_root/config.yml"
failure 'existing private configuration is never overwritten' sh "$restore_root/deploy/backup.sh" restore --source "$backup" --confirm-replace-database
[[ $(cat "$restore_root/config.yml") == existing-config ]] || fail 'existing configuration changed'
rm "$restore_root/config.yml"

bad="$TEST_TMP/tampered"
cp -R "$backup" "$bad"
printf 'changed\n' >> "$bad/database.dump"
failure 'checksum mismatch is refused before writing configuration' sh "$restore_root/deploy/backup.sh" restore --source "$bad" --confirm-replace-database
[[ ! -e "$restore_root/.env" ]] || fail 'configuration written for corrupted archive'
printf '%064d  ../outside\n' 0 >> "$bad/SHA256SUMS"
failure 'checksum manifest cannot reference outside files' sh "$restore_root/deploy/backup.sh" restore --source "$bad" --confirm-replace-database

bad="$TEST_TMP/linked"
cp -R "$backup" "$bad"
mkdir "$TEST_TMP/link-fixture"
if ln -s ../outside "$TEST_TMP/link-fixture/link" 2>/dev/null; then
    tar -czf "$bad/uploads.tar.gz" -C "$TEST_TMP/link-fixture" .
    rehash "$bad"
    failure 'symlink archives are refused even with valid checksums' sh "$restore_root/deploy/backup.sh" restore --source "$bad" --confirm-replace-database
else
    printf 'ok - symlink archive rejection # SKIP symlinks unavailable\n'
fi

mkdir "$TEST_TMP/archive-fixture"
printf 'archive fixture\n' > "$TEST_TMP/archive-fixture/data"
for attack in traversal absolute duplicate; do
    bad="$TEST_TMP/archive-$attack"
    cp -R "$backup" "$bad"
    case "$attack" in
        traversal) tar -czf "$bad/uploads.tar.gz" --transform='s|^|../|' -C "$TEST_TMP/archive-fixture" data;;
        absolute) tar -czf "$bad/uploads.tar.gz" --transform='s|^|/|' -C "$TEST_TMP/archive-fixture" data;;
        duplicate) tar -czf "$bad/uploads.tar.gz" -C "$TEST_TMP/archive-fixture" data data;;
    esac
    rehash "$bad"
    failure "$attack tar paths are refused even with valid checksums" sh "$restore_root/deploy/backup.sh" restore --source "$bad" --confirm-replace-database
    [[ ! -e "$restore_root/.env" ]] || fail 'configuration written before unsafe tar rejection'
done

if ln "$TEST_TMP/archive-fixture/data" "$TEST_TMP/archive-fixture/hardlink" 2>/dev/null; then
    bad="$TEST_TMP/hardlinked"
    cp -R "$backup" "$bad"
    tar -czf "$bad/uploads.tar.gz" -C "$TEST_TMP/archive-fixture" data hardlink
    rehash "$bad"
    failure 'hardlink archives are refused even with valid checksums' sh "$restore_root/deploy/backup.sh" restore --source "$bad" --confirm-replace-database
else
    printf 'ok - hardlink archive rejection # SKIP hardlinks unavailable\n'
fi

# Remove only fixture data under this test-owned temporary volume directory.
for volume in "$MOCK_VOLUMES"/faka-58_*; do rm -rf -- "$volume"; done
export MOCK_EXISTING_CONTAINERS=existing-container
failure 'an existing Compose project is refused even with empty volumes' sh "$restore_root/deploy/backup.sh" restore --source "$backup" --confirm-replace-database
unset MOCK_EXISTING_CONTAINERS
: > "$MOCK_LOG"
success 'fresh restore imports PostgreSQL transactionally using its existing database identity' env CADDY_PROXY_MODE=cloudflare MOCK_ASSERT_PROXY_MODE_UNSET=yes MOCK_POSTGRES_DB=legacy-db MOCK_POSTGRES_USER=legacy-admin sh "$restore_root/deploy/backup.sh" restore --source "$backup" --confirm-replace-database
cmp "$source_root/.env" "$restore_root/.env"
cmp "$source_root/config.yml" "$restore_root/config.yml"
contains "$MOCK_LOG" '--exit-on-error --single-transaction' 'restore is not transactional'
contains "$MOCK_LOG" 'pg_restore -U legacy-admin -d legacy-db --clean --if-exists --exit-on-error --single-transaction' 'restore replaced the configured database identity'
contains "$MOCK_LOG" 'up -d --no-build --wait --wait-timeout 300' 'restore did not use prebuilt images'
[[ $(cat "$MOCK_VOLUMES/faka-58_uploads/data") == fixture-uploads ]] || fail 'uploads were not restored'
[[ $(cat "$MOCK_VOLUMES/restored.dump") == fake-postgres-dump ]] || fail 'database dump was not restored'
not_contains "$MOCK_LOG" 'volume rm' 'restore removed a volume'
printf 'Passed %s backup tests.\n' "$pass_count"
