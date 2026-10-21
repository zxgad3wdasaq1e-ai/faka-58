#!/bin/sh
(
set -eu
# The entrypoint has already created POSTGRES_DB as administrator POSTGRES_USER.
# Only the application role owns its database; it has no cluster administration rights.
psql --set ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    --set app_user="$POSTGRES_APP_USER" --set app_database="$POSTGRES_DB" \
    --set app_password="$(cat /run/secrets/postgres_password)" <<'SQL'
CREATE ROLE :"app_user" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD :'app_password';
ALTER DATABASE :"app_database" OWNER TO :"app_user";
REVOKE ALL ON DATABASE :"app_database" FROM PUBLIC;
GRANT CONNECT ON DATABASE :"app_database" TO :"app_user";
SQL
)
