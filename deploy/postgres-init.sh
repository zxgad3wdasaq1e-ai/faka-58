#!/bin/sh
(
set -eu
# The entrypoint has already created database getgpt as administrator postgres.
# Only the application role owns its database; it has no cluster administration rights.
psql --set ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    --set app_password="$(cat /run/secrets/postgres_password)" <<'SQL'
CREATE ROLE getgpt WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD :'app_password';
ALTER DATABASE getgpt OWNER TO getgpt;
REVOKE ALL ON DATABASE getgpt FROM PUBLIC;
GRANT CONNECT ON DATABASE getgpt TO getgpt;
SQL
)
