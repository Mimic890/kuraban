#!/bin/sh
# Starts the demo. Safe to run again: an existing .env is kept.
set -eu
cd "$(dirname "$0")"

secret() { od -An -N"$1" -tx1 /dev/urandom | tr -d ' \n'; }

if [ ! -f .env ]; then
  rpc_secret=$(secret 32)
  admin_token=$(secret 32)
  (
    umask 077
    cat > .env <<EOF
UID=$(id -u)
GID=$(id -g)
GARAGE_RPC_SECRET=$rpc_secret
GARAGE_ADMIN_TOKEN=$admin_token
EOF
  )
fi

# Created here so Docker does not create them as root.
mkdir -p data garage/meta garage/data
docker compose up -d --wait

echo "Kuraban is running at http://127.0.0.1:8080"
