# Local Garage cluster for testing

This sets up a single-node Garage and Kuraban with Docker Compose. It is meant for trying the UI or for development, not for production. If you already have a Garage cluster, go to the [quick start](../README.md#quick-start).

## 1. Create the files

Create an empty directory with two files.

`garage.toml`:

```toml
metadata_dir = "/var/lib/garage/meta"
data_dir = "/var/lib/garage/data"
db_engine = "lmdb"
replication_factor = 1

rpc_bind_addr = "[::]:3901"
rpc_public_addr = "127.0.0.1:3901"
rpc_secret = "REPLACE_WITH_RPC_SECRET"

[s3_api]
s3_region = "garage"
api_bind_addr = "[::]:3900"
root_domain = ".s3.garage"

[admin]
api_bind_addr = "[::]:3903"
admin_token = "REPLACE_WITH_ADMIN_TOKEN"
```

Generate the two secrets and paste them in:

```bash
openssl rand -hex 32   # rpc_secret
openssl rand -hex 32   # admin_token
```

`docker-compose.yml`:

```yaml
services:
  garage:
    image: dxflrs/garage:v2.1.0
    restart: unless-stopped
    volumes:
      - ./garage.toml:/etc/garage.toml:ro
      - garage_meta:/var/lib/garage/meta
      - garage_data:/var/lib/garage/data

  kuraban:
    image: ghcr.io/mimic890/kuraban:latest
    restart: unless-stopped
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - kuraban_data:/app/data
    environment:
      - KURABAN_SERVER_ENVIRONMENT=development
      - KURABAN_SERVER_ROOT_URL=http://localhost:8080
      - KURABAN_GARAGE_ENDPOINT=http://garage:3900
      - KURABAN_GARAGE_ADMIN_ENDPOINT=http://garage:3903
      - KURABAN_GARAGE_ADMIN_TOKEN=REPLACE_WITH_ADMIN_TOKEN
    depends_on:
      - garage

volumes:
  garage_meta:
  garage_data:
  kuraban_data:
```

Use the same `admin_token` in both files.

## 2. Start Garage and assign the layout

A fresh Garage node has no storage role until a layout is applied:

```bash
docker compose up -d garage
NODE_ID=$(docker compose exec -T garage /garage node id -q | cut -d@ -f1)
docker compose exec -T garage /garage layout assign -z dc1 -c 10G "$NODE_ID"
docker compose exec -T garage /garage layout apply --version 1
```

## 3. Start the UI

```bash
docker compose up -d kuraban
```

Open http://localhost:8080 and create the admin account. The cluster is already connected through the `KURABAN_GARAGE_*` variables.

## Further reading

- [Configuration reference](configuration.md)
- [Garage documentation](https://garagehq.deuxfleurs.fr/documentation/)
