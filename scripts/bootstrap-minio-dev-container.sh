#!/bin/sh
set -eu

alias_name=bootstrap
bucket=timelapse-dev
policy=timelapse-dev-uploader

mc alias set "$alias_name" http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"
mc mb --ignore-existing "$alias_name/$bucket"
mc anonymous set none "$alias_name/$bucket"
mc admin user add "$alias_name" "$MINIO_UPLOADER_ACCESS_KEY" "$MINIO_UPLOADER_SECRET_KEY"
mc admin policy create "$alias_name" "$policy" /config/dev-uploader-policy.json
mc admin policy attach "$alias_name" "$policy" --user "$MINIO_UPLOADER_ACCESS_KEY"

mc stat "$alias_name/$bucket"
