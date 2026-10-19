#!/bin/sh
set -eu
umask 077
bucket="${MINIO_BUCKET:-storefront-media}"
root_user="$(cat /run/secrets/minio_root_user)"
root_password="$(cat /run/secrets/minio_root_password)"
app_user="$(cat /run/secrets/minio_app_user)"
app_password="$(cat /run/secrets/minio_app_password)"
mc alias set store http://minio:9000 "$root_user" "$root_password" >/dev/null
mc mb --ignore-existing "store/$bucket" >/dev/null
# 应用凭据只允许在图床 bucket 写入/清理对象，不授予服务器管理权限。
cat >/tmp/upload-policy.json <<EOF
{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:PutObject","s3:DeleteObject"],"Resource":["arn:aws:s3:::$bucket/*"]}]}
EOF
mc admin user add store "$app_user" "$app_password" >/dev/null
mc admin policy create store storefront-upload /tmp/upload-policy.json >/dev/null
mc admin policy attach store storefront-upload --user "$app_user" >/dev/null
# 图床只公开对象读取，不允许匿名列举 bucket、上传或删除。
cat >/tmp/public-policy.json <<EOF
{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::$bucket/*"]}]}
EOF
mc anonymous set-json /tmp/public-policy.json "store/$bucket" >/dev/null
echo 'MinIO 图床 bucket 和应用账号已就绪。'
