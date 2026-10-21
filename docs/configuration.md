# Configuration

Kuraban is configured in three places:

1. **Environment variables** with the `KURABAN_` prefix. Most deployments need nothing else.
2. **An optional YAML file**, `config.yaml` in the working directory by default, or the path given with `-config`. Required only for `access_control`, which can't be expressed as environment variables.
3. **The Settings page** in the UI, for everything that can change at runtime. These values are stored in `kuraban.db`.

Environment variables override the YAML file. YAML keys mirror the variable names: `KURABAN_AUTH_OIDC_CLIENT_ID` is `auth.oidc.client_id`.

## Persistent data

| File | Contents |
|---|---|
| `$KURABAN_DATA_DIR/state.json` | Admin account and the list of clusters with their admin tokens. |
| `$KURABAN_DATA_DIR/kuraban.db` | SQLite: runtime settings and metrics history. |

`KURABAN_DATA_DIR` defaults to `data` relative to the working directory, which is `/app/data` in the Docker image. Back up this directory: `state.json` holds credentials.

## First-run provisioning

These variables are read only at startup and only when the matching state is still empty. Once an admin account or a cluster exists, they are ignored.

| Variable | Description |
|---|---|
| `KURABAN_AUTH_ADMIN_USERNAME`, `KURABAN_AUTH_ADMIN_PASSWORD` | Create the admin account without going through the setup page. |
| `KURABAN_GARAGE_ENDPOINT` | S3 endpoint of the first cluster, for example `http://garage:3900`. |
| `KURABAN_GARAGE_ADMIN_ENDPOINT` | Admin API endpoint, for example `http://garage:3903`. |
| `KURABAN_GARAGE_ADMIN_TOKEN` | `admin_token` from `garage.toml`. |
| `KURABAN_GARAGE_REGION` | S3 region. Default `garage`. |

Without these, the UI opens a setup page on first visit where you create the admin account (password of at least 12 characters), and clusters are added from the UI.

## Server

| Variable | Default | Description |
|---|---|---|
| `KURABAN_SERVER_HOST` | `::` | Listen address. `::` listens on IPv4 and IPv6. |
| `KURABAN_SERVER_PORT` | `8080` | Listen port. |
| `KURABAN_SERVER_ENVIRONMENT` | `production` | `production` or `development`. Production rejects insecure OIDC flags and cookies. |
| `KURABAN_SERVER_ROOT_URL` | | Public URL of the UI, for example `https://kuraban.example.com`. Required for OIDC. |
| `KURABAN_SERVER_DOMAIN` | | Public domain name. |
| `KURABAN_SERVER_PROTOCOL` | | `http` or `https`. |
| `KURABAN_SERVER_ALLOWED_IPS` | | Comma-separated IPs or CIDRs allowed to reach the UI. Empty allows everyone. Can be changed in Settings. |
| `KURABAN_SERVER_CLUSTER_ENDPOINT_ALLOWLIST` | | Private IPs or CIDRs that may be used as cluster endpoints when a cluster is added from the UI. See below. |
| `KURABAN_SERVER_MAX_BODY_SIZE` | 64 MB | Maximum request body in bytes. Limits the upload size. |
| `KURABAN_SERVER_MAX_HEADER_SIZE` | 32 KB | Maximum request header size in bytes. |
| `KURABAN_SERVER_READ_BUFFER_SIZE` | 4 KB | Read buffer in bytes. |
| `KURABAN_SERVER_WRITE_BUFFER_SIZE` | 4 KB | Write buffer in bytes. |

### Cluster endpoint allowlist

When an admin adds a cluster through the UI, private addresses (RFC 1918, unique-local IPv6) are rejected to prevent the server from being used to reach internal services. Loopback, link-local and cloud metadata addresses are always rejected.

If your Garage nodes live on a private network, list their addresses here, for example `10.0.0.0/8,192.168.1.10`. The UI also offers to add a blocked address to the list when you add a cluster.

Clusters provisioned from `KURABAN_GARAGE_*` variables are trusted and never checked.

## Logging

| Variable | Default | Description |
|---|---|---|
| `KURABAN_LOGGING_LEVEL` | `info` | `trace`, `debug`, `info`, `warn`, `error`. Can be changed in Settings. |
| `KURABAN_LOGGING_FORMAT` | `text` | `text` or `json`. |

## Authentication

| Variable | Default | Description |
|---|---|---|
| `KURABAN_AUTH_ADMIN_ENABLED` | `false` | Enable the admin login defined in config (username and password from the variables above). |
| `KURABAN_AUTH_TOKEN_ENABLED` | `false` | Allow logging in with the Garage admin token. |
| `KURABAN_AUTH_JWT_PRIVATE_KEY` | random | Ed25519 private key in PEM used to sign sessions. Without it a new key is generated on every start, so all users are logged out after a restart and replicas can't share sessions. |
| `KURABAN_AUTH_BOOTSTRAP_TOKEN` | | One-time bootstrap token. |

### OIDC

| Variable | Default | Description |
|---|---|---|
| `KURABAN_AUTH_OIDC_ENABLED` | `false` | Enable OIDC login. |
| `KURABAN_AUTH_OIDC_PROVIDER_NAME` | | Name shown on the login button. |
| `KURABAN_AUTH_OIDC_ISSUER_URL` | | Issuer URL. Must be HTTPS. |
| `KURABAN_AUTH_OIDC_CLIENT_ID` | | Client ID. |
| `KURABAN_AUTH_OIDC_CLIENT_SECRET` | | Client secret. |
| `KURABAN_AUTH_OIDC_SCOPES` | | Comma-separated scopes, for example `openid,profile,email`. Required. |
| `KURABAN_AUTH_OIDC_EMAIL_ATTRIBUTE` | | Claim with the email. |
| `KURABAN_AUTH_OIDC_USERNAME_ATTRIBUTE` | | Claim with the username. |
| `KURABAN_AUTH_OIDC_NAME_ATTRIBUTE` | | Claim with the display name. |
| `KURABAN_AUTH_OIDC_ROLE_ATTRIBUTE_PATH` | | [JMESPath](https://jmespath.org/) expression that returns the user's roles. |
| `KURABAN_AUTH_OIDC_ADMIN_ROLES` | | Comma-separated roles that grant admin access. `ADMIN_ROLE` accepts a single value. |
| `KURABAN_AUTH_OIDC_TEAM_ATTRIBUTE_PATH` | | JMESPath expression that returns the user's teams. Required with `access_control.teams`. |
| `KURABAN_AUTH_OIDC_SESSION_MAX_AGE` | `86400` | Session lifetime in seconds. |
| `KURABAN_AUTH_OIDC_COOKIE_NAME` | `kuraban_session` | Session cookie name. |
| `KURABAN_AUTH_OIDC_COOKIE_SECURE` | `true` | `Secure` cookie flag. |
| `KURABAN_AUTH_OIDC_COOKIE_HTTP_ONLY` | `true` | `HttpOnly` cookie flag. |
| `KURABAN_AUTH_OIDC_COOKIE_SAME_SITE` | `lax` | `lax`, `strict` or `none`. |
| `KURABAN_AUTH_OIDC_SKIP_ISSUER_CHECK`, `..._SKIP_EXPIRY_CHECK`, `..._TLS_SKIP_VERIFY` | `false` | Testing only. Rejected in production. |

Startup fails with a clear error if OIDC is enabled and the configuration is incomplete. In particular, without `access_control` you must set admin roles. Otherwise every user who can log in at the identity provider would become a cluster admin.

The redirect URI to register at the provider is `<ROOT_URL>/auth/oidc/callback`.

### Access control

Team-based permissions for OIDC users are configured in the YAML file only. See [access-control.md](access-control.md).

## Metrics endpoint

Kuraban can expose its own Prometheus metrics at `/metrics`.

| Variable | Default | Description |
|---|---|---|
| `KURABAN_AUTH_METRICS_PUBLIC` | `false` | Expose `/metrics` without login. |
| `KURABAN_AUTH_METRICS_SHARED_SECRET` | | Require this value in the `X-Metrics-Token` header or the `?token=` parameter. |
| `KURABAN_AUTH_METRICS_ALLOWED_IPS` | | Comma-separated IPs or CIDRs allowed to scrape. |

## CORS

Only needed when the frontend is served from a different origin than the API.

| Variable | Description |
|---|---|
| `KURABAN_CORS_ENABLED` | Enable CORS. |
| `KURABAN_CORS_ALLOWED_ORIGINS` | Comma-separated origins. |
| `KURABAN_CORS_ALLOWED_METHODS` | Comma-separated methods. |
| `KURABAN_CORS_ALLOWED_HEADERS` | Comma-separated headers. |
| `KURABAN_CORS_ALLOW_CREDENTIALS` | Allow credentials. |
| `KURABAN_CORS_MAX_AGE` | Preflight cache time in seconds. |

## Secrets from files

The following variables also accept a `_FILE` suffix pointing to a file, for use with Docker or Kubernetes secrets. The file content (without the trailing newline) takes precedence over the plain variable.

- `KURABAN_AUTH_BOOTSTRAP_TOKEN`
- `KURABAN_AUTH_ADMIN_USERNAME`, `KURABAN_AUTH_ADMIN_PASSWORD`
- `KURABAN_AUTH_JWT_PRIVATE_KEY`
- `KURABAN_AUTH_OIDC_CLIENT_ID`, `KURABAN_AUTH_OIDC_CLIENT_SECRET`
- `KURABAN_AUTH_METRICS_SHARED_SECRET`

Example: `KURABAN_AUTH_OIDC_CLIENT_SECRET_FILE=/run/secrets/oidc_client_secret`.

## Settings page

These values live in `kuraban.db` and are edited under **Settings**:

| Section | Settings |
|---|---|
| Monitoring | Collection on or off, polling interval (10 s to 5 min), retention (1 hour to 365 days), bucket statistics interval. Full resolution is kept up to 3 days, 5-minute averages up to 35 days, hourly averages for the whole retention. |
| Dashboard | Disk, error-rate and latency thresholds, shared by all users. |
| Security and server | Log level, allowed client IPs, cluster endpoint allowlist. These override the environment variables; "reset" returns to the environment value. An IP allowlist that would lock out your own address is rejected. |
| Appearance, language and region | Theme, fonts, text size, density, language, time zone. Stored per browser. |
