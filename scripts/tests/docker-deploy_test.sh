#!/bin/sh
# Docker/curl are mocked. Deliberately failing python commands stay on PATH.
set -eu

repository=$(CDPATH='' cd "$(dirname "$0")/../.." && pwd -P)
bootstrap=$repository/deploy/docker-deploy.sh
shell=$(command -v sh)
temp_base=$(CDPATH='' cd "${TMPDIR:-/tmp}" && pwd -P)
test_root=$(mktemp -d "$temp_base/faka-58-shell-test.XXXXXXXX")
cleanup() { case "$test_root" in "$temp_base"/faka-58-shell-test.*) rm -rf -- "$test_root" ;; esac; }
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir "$test_root/bin"
export STORE_TEST_REPOSITORY=$repository
export STORE_TEST_LOG=$test_root/commands.log
export STORE_TEST_PYTHON_LOG=$test_root/python.log
export STORE_TEST_CLOCK=$test_root/clock
export TMPDIR=$test_root

cat >"$test_root/bin/docker" <<'MOCK'
#!/bin/sh
set -eu
{
    printf 'docker'
    for argument do printf '\t%s' "$argument"; done
    printf '\n'
} >>"$STORE_TEST_LOG"
case "$1" in
    info) [ "${MOCK_DAEMON_FAILURE:-false}" = false ]; exit ;;
    pull) [ "${MOCK_PULL_FAILURE:-false}" = false ]; exit ;;
    image)
        printf '%s\n' "${MOCK_SOURCE_LABEL:-\"https://github.com/zxgad3wdasaq1e-ai/faka-58\"}" \
            "${MOCK_REVISION_LABEL:-\"abcd1234abcd1234abcd1234abcd1234abcd1234\"}" \
            "${MOCK_DIGESTS:-ghcr.io/zxgad3wdasaq1e-ai/faka-58@sha256:1111111111111111111111111111111111111111111111111111111111111111}"
        exit ;;
    compose)
        case "$*" in
            *'up -d --no-build --wait --wait-timeout 300 postgres') [ "${MOCK_DATABASE_FAILURE:-false}" = false ] || exit 1 ;;
            *' app db-check '*) [ "${MOCK_DATABASE_CHECK_FAILURE:-false}" = false ] || exit 3 ;;
        esac
        if [ "$2" = exec ] && [ "${MOCK_SEED_FAILURE:-false}" = true ]; then exit 1; fi
        exit 0 ;;
    run)
        action=
        image=
        local_mode=false
        while [ "$#" -gt 0 ]; do
            case "$1" in
                init|info) action=$1; shift ;;
                --image) image=$2; shift 2 ;;
                --local) local_mode=true; shift ;;
                *) shift ;;
            esac
        done
        case "$action" in
            init)
                mkdir -p deploy/runtime deploy/secrets
                printf 'APP_IMAGE=%s\n' "$image" >.env
                printf '%s\n' 'private-config-MUST-NOT-PRINT' >config.yml
                printf '%s\n' '{}' >deploy/runtime/installation.json
                if [ "$local_mode" = true ]; then
                    printf '%s\n' 'http://localhost:1888' >deploy/runtime/test-origin
                else
                    printf '%s\n' 'https://xopenai.vip' >deploy/runtime/test-origin
                fi
                printf '%s\n' 'initial-password-MUST-NOT-PRINT' >deploy/secrets/initial-admin.txt
                [ "${MOCK_INIT_FAILURE:-false}" = false ] ;;
            info) cat deploy/runtime/test-origin ;;
            *) echo 'unexpected mocked docker run' >&2; exit 91 ;;
        esac ;;
    *) echo 'unexpected mocked Docker command' >&2; exit 92 ;;
esac
MOCK

cat >"$test_root/bin/curl" <<'MOCK'
#!/bin/sh
set -eu
output=
url=
write_out=
while [ "$#" -gt 0 ]; do
    case "$1" in
        --output|-o) output=$2; shift 2 ;;
        --write-out) write_out=$2; shift 2 ;;
        --proto|--proto-redir|--max-time) shift 2 ;;
        http://*|https://*) url=$1; shift ;;
        *) shift ;;
    esac
done
printf 'curl\t%s\n' "$url" >>"$STORE_TEST_LOG"
case "$url" in
    https://raw.githubusercontent.com/zxgad3wdasaq1e-ai/faka-58/abcd1234abcd1234abcd1234abcd1234abcd1234/*)
        relative=${url#https://raw.githubusercontent.com/zxgad3wdasaq1e-ai/faka-58/abcd1234abcd1234abcd1234abcd1234abcd1234/}
        [ "$relative" != "${MOCK_DOWNLOAD_FAILURE:-never}" ] || exit 22
        case "$relative" in
            deploy/start.sh) cp "$STORE_TEST_REPOSITORY/deploy/start.sh" "$output" ;;
            compose.yml|deploy/Caddyfile|deploy/postgres-init.sh|deploy/backup.sh|LICENSE|README.md) printf 'runtime fixture\n' >"$output" ;;
            *) echo 'unknown runtime path' >&2; exit 93 ;;
        esac
        [ "$write_out" = '%{url_effective}' ] || exit 94
        if [ "${MOCK_REDIRECT:-false}" = true ]; then printf 'https://example.test/other'; else printf '%s' "$url"; fi ;;
    */health)
        [ "${MOCK_HEALTH_FAILURE:-false}" = false ] || exit 7
        [ "$write_out" = '%{http_code}' ] || exit 95
        printf '200' ;;
    *) echo 'unexpected mocked curl URL' >&2; exit 96 ;;
esac
MOCK

cat >"$test_root/bin/python" <<'MOCK'
#!/bin/sh
printf 'unexpected Python invocation\n' >>"$STORE_TEST_PYTHON_LOG"
exit 97
MOCK
cp "$test_root/bin/python" "$test_root/bin/python3"
cat >"$test_root/bin/date" <<'MOCK'
#!/bin/sh
set -eu
clock=1000
if [ -f "$STORE_TEST_CLOCK" ]; then read -r clock <"$STORE_TEST_CLOCK"; fi
printf '%s\n' "$((clock + 60))" >"$STORE_TEST_CLOCK"
printf '%s\n' "$clock"
MOCK
cat >"$test_root/bin/sleep" <<'MOCK'
#!/bin/sh
exit 0
MOCK
chmod +x "$test_root/bin/"*
PATH=$test_root/bin:$PATH
export PATH

count=0
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
new_case() {
    count=$((count + 1))
    case_dir=$test_root/case-$count
    mkdir "$case_dir"
    : >"$STORE_TEST_LOG"
    rm -f "$STORE_TEST_CLOCK"
    unset MOCK_SOURCE_LABEL MOCK_REVISION_LABEL MOCK_DIGESTS MOCK_PULL_FAILURE MOCK_DAEMON_FAILURE
    unset MOCK_DOWNLOAD_FAILURE MOCK_REDIRECT MOCK_INIT_FAILURE MOCK_SEED_FAILURE MOCK_HEALTH_FAILURE
    unset MOCK_DATABASE_FAILURE MOCK_DATABASE_CHECK_FAILURE
}
run_install() { (cd "$case_dir" && "$shell" "$bootstrap" "$@") >"$test_root/output" 2>&1; }
expect_failure() { if run_install "$@"; then fail 'expected installation failure'; fi; }
assert_no_public_files() { [ ! -e "$case_dir/compose.yml" ] && [ ! -e "$case_dir/deploy" ] || fail 'runtime files published on rejected install'; }
assert_no_secret_output() { if grep -q 'MUST-NOT-PRINT' "$test_root/output"; then fail 'credentials appeared in stdout/stderr'; fi; }

new_case
mkdir "$test_root/no-tools"
PATH=$test_root/no-tools "$shell" "$bootstrap" --help >"$test_root/output"
PATH=$test_root/no-tools "$shell" "$repository/deploy/start.sh" --help >>"$test_root/output"
printf 'ok - help requires no Docker, curl or language runtime\n'

new_case
run_install
[ -f "$case_dir/config.yml" ] || fail 'init did not run'
grep -q '^APP_IMAGE=ghcr.io/zxgad3wdasaq1e-ai/faka-58@sha256:1111' "$case_dir/.env" || fail 'image was not pinned'
[ "$(grep -c '^curl.*raw.githubusercontent.com' "$STORE_TEST_LOG")" -eq 7 ] || fail 'runtime allowlist download count differs'
if grep '^curl.*raw.githubusercontent.com' "$STORE_TEST_LOG" | grep -v '/abcd1234abcd1234abcd1234abcd1234abcd1234/'; then fail 'runtime downloaded from mutable branch'; fi
grep -q 'compose.*up.*--no-build.*--wait' "$STORE_TEST_LOG" || fail 'start attempted a source build'
grep -q 'compose.*exec.*storectl.*seed.*http://127.0.0.1:8080' "$STORE_TEST_LOG" || fail 'seed did not use in-container helper'
grep -q 'run.*--network.none.*--user.*--volume.*--entrypoint./app/storectl.*init' "$STORE_TEST_LOG" || fail 'init isolation flags differ'
grep -q 'init.*--local' "$STORE_TEST_LOG" || fail 'default installation did not select HTTP mode'
grep -q '店面: http://localhost:1888' "$test_root/output" || fail 'default HTTP origin differs'
grep -q '远程访问: http://服务器IP:1888' "$test_root/output" || fail 'remote IP access hint missing'
assert_no_secret_output
printf 'ok - default HTTP :1888 installation with matching digest and revision\n'

new_case
run_install --local --admin-user manager --subnet 172.28.240.0/24
grep -q '店面: http://localhost:1888' "$test_root/output" || fail 'local origin differs'
grep -q 'init.*--admin-user.manager.*--subnet.172.28.240.0/24.*--local' "$STORE_TEST_LOG" || fail 'init flags not forwarded'
printf 'ok - local mode and initialization flags\n'

new_case
run_install --domain xopenai.vip
grep -q '店面: https://xopenai.vip' "$test_root/output" || fail 'explicit domain did not select HTTPS'
grep -q 'init.*--domain.xopenai.vip' "$STORE_TEST_LOG" || fail 'domain was not forwarded'
if grep -q 'init.*--local' "$STORE_TEST_LOG"; then fail 'explicit HTTPS domain selected HTTP mode'; fi
printf 'ok - explicit domain preserves automatic HTTPS deployment\n'

new_case
cp "$bootstrap" "$case_dir/docker-deploy.sh"
run_install --local
cmp "$bootstrap" "$case_dir/docker-deploy.sh" || fail 'existing normal bootstrap was overwritten'
printf 'ok - existing bootstrap file is preserved\n'

new_case
printf 'private-config-MUST-NOT-PRINT\n' >"$case_dir/config.yml"
printf 'private-env-MUST-NOT-PRINT\n' >"$case_dir/.env"
expect_failure
grep -q '^private-config-MUST-NOT-PRINT$' "$case_dir/config.yml" || fail 'existing config changed'
grep -q '^private-env-MUST-NOT-PRINT$' "$case_dir/.env" || fail 'existing env changed'
if grep -q 'docker.*pull\|^curl' "$STORE_TEST_LOG"; then fail 'nonempty target used network'; fi
assert_no_secret_output
printf 'ok - existing private files untouched\n'

new_case
if ln -s "$test_root/missing" "$case_dir/docker-deploy.sh" 2>/dev/null && [ -L "$case_dir/docker-deploy.sh" ]; then
    expect_failure
    [ -L "$case_dir/docker-deploy.sh" ] || fail 'symbolic link was changed'
    printf 'ok - symbolic links are refused\n'
else
    printf 'skip - this host cannot create symbolic links (Linux CI exercises refusal)\n'
fi

new_case
mkdir "$case_dir/unknown"
expect_failure
[ -d "$case_dir/unknown" ] || fail 'unknown directory was removed'
printf 'ok - unknown directories are refused\n'

for invalid in '--help-image' 'https://ghcr.io/a/b:tag' 'ghcr.io/a/b' 'ghcr.io/a/b:$TAG' 'ghcr.io/a/b:tag;touch-bad' 'registry:0/app:tag' 'registry:99999/app:tag'; do
    new_case
    expect_failure --image "$invalid"
    assert_no_public_files
done
printf 'ok - invalid OCI references cannot reach Docker pull\n'

new_case
MOCK_SOURCE_LABEL=$(printf '"%s"' 'https://github.com/other/repository'); export MOCK_SOURCE_LABEL
expect_failure
assert_no_public_files
new_case
MOCK_SOURCE_LABEL=$(printf '"%s"' 'https://github.com/zxgad3wdasaq1e-ai/faka-58\nspoof'); export MOCK_SOURCE_LABEL
expect_failure
assert_no_public_files
new_case
MOCK_REVISION_LABEL=$(printf '"%s"' 'main'); export MOCK_REVISION_LABEL
expect_failure
assert_no_public_files
printf 'ok - untrusted or newline-spoofed OCI labels refused\n'

new_case
MOCK_DIGESTS=ghcr.io/other/repository@sha256:1111111111111111111111111111111111111111111111111111111111111111; export MOCK_DIGESTS
expect_failure
assert_no_public_files
new_case
expect_failure --image ghcr.io/zxgad3wdasaq1e-ai/faka-58@sha256:2222222222222222222222222222222222222222222222222222222222222222
assert_no_public_files
printf 'ok - foreign repository or mismatched explicit digest refused\n'

new_case
MOCK_DOWNLOAD_FAILURE=README.md; export MOCK_DOWNLOAD_FAILURE
expect_failure
assert_no_public_files
new_case
MOCK_REDIRECT=true; export MOCK_REDIRECT
expect_failure
assert_no_public_files
printf 'ok - interrupted or redirected staged downloads never publish\n'

new_case
MOCK_PULL_FAILURE=true; export MOCK_PULL_FAILURE
expect_failure
grep -q 'public.*docker login ghcr.io' "$test_root/output" || fail 'missing GHCR recovery hint'
assert_no_public_files
printf 'ok - GHCR visibility/login failures are explained\n'

new_case
run_install --local
credentials_before=$(cksum "$case_dir/deploy/secrets/initial-admin.txt")
printf '%s\n' 'SHOULD_NOT_RUN=$(touch should-not-exist)' >>"$case_dir/.env"
(cd "$case_dir" && "$shell" deploy/start.sh --seed) >"$test_root/output" 2>&1
[ ! -e "$case_dir/should-not-exist" ] || fail '.env was executed as shell code'
[ "$credentials_before" = "$(cksum "$case_dir/deploy/secrets/initial-admin.txt")" ] || fail 'repeat start changed credentials'
assert_no_secret_output
printf 'ok - repeat startup preserves credentials and does not source env\n'

printf '%s\n' 'http://localhost:8080' >"$case_dir/deploy/runtime/test-origin"
(cd "$case_dir" && "$shell" deploy/start.sh --seed) >"$test_root/output" 2>&1
grep -q '店面: http://localhost:8080' "$test_root/output" || fail 'legacy HTTP origin cannot retry'
printf 'ok - legacy HTTP :8080 configuration can still retry\n'

printf '%s\n' 'APP_IMAGE=$(touch should-not-exist)' >"$case_dir/.env"
if (cd "$case_dir" && "$shell" deploy/start.sh) >"$test_root/output" 2>&1; then fail 'malicious APP_IMAGE accepted'; fi
[ ! -e "$case_dir/should-not-exist" ] || fail 'APP_IMAGE executed code'
printf 'ok - APP_IMAGE is validated as data\n'

new_case
MOCK_INIT_FAILURE=true; export MOCK_INIT_FAILURE
expect_failure --local
[ -f "$case_dir/config.yml" ] || fail 'failed init removed user configuration'
unset MOCK_INIT_FAILURE
(cd "$case_dir" && "$shell" deploy/start.sh --seed) >"$test_root/output" 2>&1
assert_no_secret_output
printf 'ok - initialization failure retains configuration for retry\n'

new_case
MOCK_DATABASE_FAILURE=true; export MOCK_DATABASE_FAILURE
expect_failure --local
if grep -q 'compose.*up.*300$' "$STORE_TEST_LOG"; then fail 'unhealthy database did not block app startup'; fi
assert_no_secret_output
printf 'ok - database authentication health failure blocks application startup\n'

new_case
MOCK_DATABASE_CHECK_FAILURE=true; export MOCK_DATABASE_CHECK_FAILURE
expect_failure --local
if grep -q 'compose.*up.*300$' "$STORE_TEST_LOG"; then fail 'invalid application credentials did not block startup'; fi
assert_no_secret_output
printf 'ok - application credentials are verified before application startup\n'

new_case
MOCK_HEALTH_FAILURE=true; export MOCK_HEALTH_FAILURE
expect_failure --local
grep -q 'HTTP 站点未就绪.*1888.*防火墙' "$test_root/output" || fail 'HTTP health timeout lacks port recovery hint'
if grep -q 'DNS.*80/443' "$test_root/output"; then fail 'HTTP timeout displayed HTTPS recovery hint'; fi
printf 'ok - HTTP health wait has a bounded deadline and relevant port hint\n'

new_case
MOCK_HEALTH_FAILURE=true; export MOCK_HEALTH_FAILURE
expect_failure --domain xopenai.vip
grep -q 'HTTPS 站点未就绪.*DNS.*80/443' "$test_root/output" || fail 'HTTPS health timeout lacks certificate recovery hint'
printf 'ok - HTTPS health timeout retains DNS and certificate recovery hint\n'

[ ! -s "$STORE_TEST_PYTHON_LOG" ] || fail 'installation invoked Python'
printf 'PASS: %s Docker-only shell scenarios; python/python3 were present but forbidden.\n' "$count"
