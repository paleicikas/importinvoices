#!/bin/sh
# Entrypoint for the importinvoices Docker image.
#
# On first start (when config.json does not yet exist in the data directory)
# it writes a default config that binds the HTTP server to 0.0.0.0:8080 so the
# port is reachable from outside the container. It never overwrites an existing
# config.json, so user edits made via the Settings page or by hand are preserved
# across container restarts.
set -e

DATA_DIR="${DATA_DIR:-/data}"
CONFIG="$DATA_DIR/config.json"

if [ ! -f "$CONFIG" ]; then
    mkdir -p "$DATA_DIR"
    cat > "$CONFIG" <<EOF
{
  "data_dir": "$DATA_DIR",
  "db_path": "$DATA_DIR/data.db",
  "http_addr": "0.0.0.0:8080",
  "storage_path": "$DATA_DIR/files",
  "max_upload_bytes": 10485760
}
EOF
fi

exec importinvoices --data-dir "$DATA_DIR" "$@"
