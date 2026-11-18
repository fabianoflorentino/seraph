#!/bin/sh
set -eu

out_dir="${SERAPH_OUT_DIR:-/usr/share/nginx/html/data}"
interval="${SERAPH_SYNC_INTERVAL:-300}"

mkdir -p "$out_dir"

/usr/local/bin/seraph validate --catalog /app/catalog/packs

(
  while true; do
    /usr/local/bin/seraph sync --catalog /app/catalog/packs || true
    sleep "$interval"
  done
) &

exec nginx -g "daemon off;"
