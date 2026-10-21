#!/bin/sh
set -e
umask 077

# State (admin account, cluster list) lives here and must be writable by uid 1000.
DATA_DIR="${KURABAN_DATA_DIR:-/app/data}"

mkdir -p "$DATA_DIR"

exec "$@"
