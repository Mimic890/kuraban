#!/bin/sh
# Starts the demo. Safe to run again: an existing .env is kept.
set -eu
cd "$(dirname "$0")"

secret() { od -An -N"$1" -tx1 /dev/urandom | tr -d ' \n'; }

if [ ! -f .env ]; then
  rpc_secret=$(secret 32)
  admin_token=$(secret 32)
  admin_password=$(secret 12)
  (
    umask 077
    cat > .env <<EOF
UID=$(id -u)
GID=$(id -g)
GARAGE_RPC_SECRET=$rpc_secret
GARAGE_ADMIN_TOKEN=$admin_token
KURABAN_AUTH_ADMIN_USERNAME=admin
KURABAN_AUTH_ADMIN_PASSWORD=$admin_password
EOF
  )
fi

# Created here so Docker does not create them as root.
mkdir -p data garage/meta garage/data
docker compose up -d --wait

garage() { docker compose exec -T -e RUST_LOG=warn garage /garage "$@"; }

# A new node stores nothing until it has a layout. An existing layout is left alone.
layout=$(garage layout show)
case "$layout" in
  *"Current cluster layout version: 0"*)
    node=$(garage node id -q | cut -d@ -f1)
    garage layout assign -z dc1 -c 1G "$node" > /dev/null
    garage layout apply --version 1 > /dev/null
    ;;
esac

# Not sourced: .env sets UID, which is read-only where sh is bash.
env_value() { sed -n "s/^$1=//p" .env; }
cat <<EOF
Kuraban is running at http://127.0.0.1:8080
Username: $(env_value KURABAN_AUTH_ADMIN_USERNAME)
Password: $(env_value KURABAN_AUTH_ADMIN_PASSWORD)
EOF
