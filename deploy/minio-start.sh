#!/bin/sh
set -eu
export MINIO_ROOT_USER="$(cat /run/secrets/minio_root_user)"
export MINIO_ROOT_PASSWORD="$(cat /run/secrets/minio_root_password)"
exec minio "$@"
