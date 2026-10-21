# Architecture

## Project Overview

Web UI for [Garage](https://garagehq.deuxfleurs.fr/), an S3-compatible distributed object storage system. Manages buckets, objects, access keys, and cluster state via Garage's Admin API (v1/v2) and S3 API. Intended for closed corporate VPN deployments.

- **Backend:** Go 1.26, Fiber v3 (HTTP), minio-go/v7 (S3 data plane), azuretls-client (Admin API), viper (config), zerolog (logging), golang-jwt + coreos/go-oidc (auth).
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS v4, TanStack React Query, Zustand, React Router v7, axios, uPlot, recharts, sonner, react-dropzone, highlight.js.
- **Data store:** Garage itself (S3) for objects. `data/state.json` for the admin account and clusters. `data/kuraban.db` (SQLite via pure-Go `modernc.org/sqlite`) for web-editable settings and the metrics history.
- **Deployment:** Multi-stage Dockerfile (Node build → Go build → Alpine runtime).

## Folder Structure

```
backend/
  main.go                      # Entrypoint: config load, service wiring, graceful shutdown
  internal/config/             # Viper config (YAML + env vars, KURABAN_ prefix)
  internal/handlers/           # Fiber HTTP handlers (buckets, objects, users, cluster, auth)
  internal/services/           # Garage admin client (v1 + v2) + S3 service (minio-go)
    admin_factory.go            # Probes /v2 at startup; returns v1 or v2 client + capabilities
    admin_v1.go / admin_v2.go # Two implementations of AdminService interface
    s3.go                      # All S3 operations via minio-go; per-bucket credential cache
    interfaces.go              # AdminService + S3Storage interfaces (mocks in mocks/)
  internal/routes/routes.go    # All route wiring; wildcard object routes with manual URL unescape
  internal/auth/               # JWT (Ed25519) + OIDC integration
  internal/authz/              # UI-layer permission policy (NOT a security boundary)
  internal/middleware/         # auth, CORS, request-id, structured logging
  internal/models/             # Request/response DTOs + Garage admin API JSON models
  internal/appsettings/        # Web-editable runtime settings (SQLite document, env overrides, subscribers)
  internal/telemetry/          # Metrics history: Prometheus parser, curated series, SQLite store
                               #   (raw / 5m / 1h tiers), range queries, background collector
  pkg/logger/                  # zerolog wrapper with per-request context + redaction
  pkg/utils/                   # In-memory TTL cache (GlobalCache), retry-with-backoff
frontend/
  src/main.tsx / App.tsx        # App entry + routing (BrowserRouter, ProtectedRoute)
  src/pages/                    # Route-level pages (Dashboard, Buckets, Cluster, AccessControl, Login)
  src/components/buckets/       # Object browser, table, preview, upload, dialogs
  src/components/layout/        # Shell, sidebar, header, bucket-detail-shell, theme
  src/components/ui/            # Radix-based primitives (dialog, table, button, etc.)
  src/components/auth/          # Login forms (Basic, Token, OIDC)
  src/hooks/                    # useApi (React Query hooks), useBucketObjects, usePermissions
  src/lib/api.ts                # axios instances + API client (objectsApi, bucketsApi, etc.)
  src/lib/query-client.ts       # QueryClient config + query key factory
  src/store/                    # Zustand stores (auth, cluster, settings, ui, dashboard layout)
  src/lib/locales/              # en.json, ru.json
  src/types/                    # TypeScript type definitions
```

## Key Files

| File | What it does |
|---|---|
| `backend/internal/routes/routes.go` | All API route definitions. Wildcard object routes (`/buckets/:bucket/objects/*`) use a custom `decodeObjectKey` that manually `url.QueryUnescape`es the path. SPA fallback serves `index.html` for non-API paths. |
| `backend/internal/services/s3.go` | S3 operations via minio-go. `getMinioClient` resolves per-bucket credentials from the admin API, caches them (`key:<bucket>:<op>`, 1h TTL). `ListObjects` fans out bounded `StatObject` goroutines for content-type. `InvalidateBucketCredsCache` / `ClearAllCredsCache` clear the cache after permission/key changes. |
| `backend/internal/services/admin_factory.go` | Probes `/v2/GetClusterHealth` at startup; 404 → v1 client. Returns `AdminServiceResult{Service, Capabilities, APIVersion}`. Capabilities gate which cluster UI features are available (statistics, nodeInfo, nodeStatistics). |
| `backend/internal/handlers/objects.go` | Object upload (multipart), download (stream + Range), delete, batch delete, metadata, presign. Content-Disposition + safe content-type rewriting for inline XSS prevention. |
| `backend/internal/handlers/buckets.go` | Bucket CRUD, permission grant/deny (split allow+deny), website config, quotas. `GrantBucketPermission` invalidates cred cache after changes. |
| `backend/internal/authz/vocabulary.go` | The permission registry. Every permission string (`bucket.list`, `object.write`, etc.) is defined here with its scope (global vs prefix) and mapped Garage endpoints. |
| `backend/internal/config/config.go` | Config struct tree. Env var override prefix: `KURABAN_` (e.g. `KURABAN_GARAGE_ENDPOINT`). |
| `frontend/src/lib/api.ts` | Two axios instances (`api` for `/api`, `authApiClient` for `/auth`). Response interceptor: 401 → redirect to `/login`, 501 → silent reject, all other errors → sonner toast. `encodeObjectKey` uses `encodeURIComponent` (slashes encoded; backend wildcard route decodes). |
| `frontend/src/hooks/useBucketObjects.ts` | Object listing state machine: debounced search, pagination tokens, upload tasks, delete. Has `fetchSeqRef` to discard stale fetch responses. |
| `frontend/src/hooks/useApi.ts` | React Query hooks (`useBuckets`, `useClusterHealth`, mutations with cache invalidation). Query keys defined in `query-client.ts`. |
| `frontend/src/store/auth-store.ts` | Zustand store (persisted). `initialize()` fetches `/auth/config` then `/auth/me`. Three auth modes: admin (basic), token, OIDC. |
| `backend/internal/telemetry/collector.go` | Scrapes `/metrics`, cluster health/status and (on its own interval) bucket stats for every cluster; the loop re-reads settings on change. |
| `backend/internal/telemetry/query.go` | Range evaluation: picks step/tier, spreads counter deltas across steps, forward-fills gauges for 5 minutes. |
| `frontend/src/components/charts/TimeSeriesChart.tsx` | uPlot wrapper (synced cursor, drag-zoom, tooltip, legend). Dashboard panels live in `components/dashboard/`. |
| `frontend/src/components/buckets/ObjectBrowserView.tsx` | Main object browser: drag-drop upload (two dropzone instances), breadcrumb nav, selection, bulk-delete confirm. |

## API Endpoints

All API routes under `/api/v1` require auth (JWT bearer token or OIDC cookie). `/auth/config` is public.

| Method | Path | Handler | Purpose |
|---|---|---|---|
| GET | `/health`, `/api/v1/health` | `health.Check` | Health + version (no auth) |
| GET | `/api/v1/capabilities` | `capabilities.GetCapabilities` | Feature flags for Garage v1 vs v2 |
| GET | `/api/v1/buckets/` | `buckets.ListBuckets` | List all buckets |
| POST | `/api/v1/buckets/` | `buckets.CreateBucket` | Create bucket (name from body) |
| GET | `/api/v1/buckets/:name` | `buckets.GetBucketInfo` | Bucket details |
| DELETE | `/api/v1/buckets/:name` | `buckets.DeleteBucket` | Delete bucket |
| POST | `/api/v1/buckets/:name/permissions` | `buckets.GrantBucketPermission` | Grant/deny key perms (split allow+deny) |
| PUT | `/api/v1/buckets/:name/website` | `buckets.UpdateBucketWebsite` | Website config |
| PUT | `/api/v1/buckets/:name/quotas` | `buckets.UpdateBucketQuotas` | Max size / max objects |
| GET | `/api/v1/buckets/:bucket/objects/` | `objects.ListObjects` | List objects (prefix, max_keys, continuation_token, search) |
| POST | `/api/v1/buckets/:bucket/objects/` | `objects.UploadObject` | Single upload (multipart/form-data) |
| POST | `/api/v1/buckets/:bucket/objects/upload-multiple` | `objects.UploadMultipleObjects` | Batch upload |
| POST | `/api/v1/buckets/:bucket/objects/delete-multiple` | `objects.DeleteMultipleObjects` | Batch delete (keys + recursive prefixes) |
| GET/HEAD/DELETE | `/api/v1/buckets/:bucket/objects/*` | wildcard handler | Object get/metadata/delete (key is `*` param) |
| GET | `.../objects/*/metadata` | `objects.GetObjectMetadata` | Object stat (HEAD) |
| GET | `.../objects/*/presign` | `objects.GetPresignedURL` | Presigned GET URL |
| GET | `.../objects/*/preview-url` | `objects.GetPreviewURL` | Short-lived media URL |
| POST | `/api/v1/buckets/:bucket/directories` | `objects.CreateDirectory` | Zero-byte dir marker |
| GET | `/api/v1/users/` | `users.ListUsers` | List access keys |
| POST | `/api/v1/users/` | `users.CreateUser` | Create key |
| GET/DELETE/PATCH | `/api/v1/users/:access_key` | `users.*` | Get / delete / update key |
| GET | `/api/v1/users/:access_key/secret` | `users.GetUserSecretKey` | Reveal secret key |
| GET | `/api/v1/cluster/health` | `cluster.GetHealth` | Cluster health |
| GET | `/api/v1/cluster/status` | `cluster.GetStatus` | Cluster status (nodes, layout) |
| GET | `/api/v1/cluster/statistics` | `cluster.GetStatistics` | Cluster stats (v2 only) |
| GET | `/api/v1/cluster/nodes/:node_id` | `cluster.GetNodeInfo` | Per-node info (v2 only) |
| GET | `/api/v1/cluster/nodes/:node_id/statistics` | `cluster.GetNodeStatistics` | Per-node stats (v2 only) |
| GET | `/api/v1/monitoring/dashboard` | `monitoring.GetDashboardMetrics` | Aggregated dashboard metrics |
| GET/PUT | `/api/v1/settings` | `settings.GetSettings` / `UpdateSettings` | Runtime settings (PUT: cluster admin) |
| GET | `/api/v1/telemetry/status` | `telemetry.Status` | Collector + database status for X-Cluster-Id |
| GET | `/api/v1/telemetry/series` | `telemetry.Series` | Recorded series catalogue |
| POST | `/api/v1/telemetry/query` | `telemetry.Query` | Batch range queries (`rate`, `increase`, `ratio`, `avg/max/min/last`, `group_by`, `top_k`) |
| POST | `/api/v1/telemetry/scrape` | `telemetry.Scrape` | Collect now (cluster admin) |
| DELETE | `/api/v1/telemetry/history` | `telemetry.Purge` | Delete history (`?scope=all` for every cluster) |
| GET | `/auth/config` | `auth.GetAuthConfig` | Auth mode flags (public) |
| POST | `/auth/login` | `auth.LoginAdmin` | Admin basic auth (if enabled) |
| POST | `/auth/login-token` | `auth.LoginToken` | Token auth (if enabled) |
| GET | `/auth/me` | `auth.GetMe` | Current user (if any auth enabled) |
| GET/POST | `/auth/oidc/*` | OIDC login/callback/logout | OIDC flow (if enabled) |

## Code Conventions

### Backend (Go)
- **Module path:** `Mimic890/kuraban` (not a standard domain path).
- **Interfaces:** `AdminService` and `S3Storage` in `services/interfaces.go`. Hand-rolled mocks in `services/mocks/` (not codegen).
- **JSON tags:** `snake_case` for API-facing structs (`json:"access_key_id"`); Garage admin models use `camelCase` matching Garage's JSON (`json:"accessKeyId"`).
- **Error responses:** All handlers return `models.ErrorResponse(code, message)` (sets `success: false`) or `models.SuccessResponse(data)`. Error codes are string constants (`models.ErrCodeBadRequest`, etc.).
- **Config:** Viper with YAML file + env override. Env prefix `KURABAN_`, nested via `_` (e.g. `KURABAN_AUTH_ADMIN_ENABLED`).
- **Logging:** `logpkg.FromCtx(ctx)` for per-request logger (carries request_id, user_id). Never `log.Print`.
- **Retry:** `utils.RetryWithBackoff` retries only on `ECONNREFUSED` (safe for idempotent and non-idempotent ops — request never reached server).
- **Tests:** `go test ./...`; smoke tests in `backend/tests/smoke/` with `//go:build smoke`.

### Frontend (TypeScript/React)
- **Path alias:** `@/` maps to `src/` (Vite + tsconfig).
- **Server state:** TanStack React Query. Query keys via `queryKeys` factory in `lib/query-client.ts`. Default `staleTime: 5min`, `refetchOnWindowFocus: false`.
- **Client state:** Zustand stores in `store/` (auth, active cluster, settings, UI, dashboard layout).
- **API client:** `lib/api.ts` — `objectsApi`, `bucketsApi`, `accessApi`, `garageApi`, `analyticsApi`, `authApi`, `healthApi`, `capabilitiesApi`. Two axios instances (`/api` and `/auth`).
- **Error surfacing:** Axios response interceptor auto-shows a sonner toast for all non-401/501 errors. 401 → `localStorage` clear + redirect to `/login`.
- **UI primitives:** Custom components in `components/ui/` (Radix-based: dialog, dropdown, tooltip, checkbox, etc.). Tailwind CSS v4 for styling. `cn()` from `lib/utils.ts` merges classes.
- **No comments in code** unless explaining a non-obvious decision.

## Build & Run

```bash
# Frontend dev (proxies /api and /auth to localhost:8080)
cd frontend && npm install && npm run dev        # Vite dev server on :3000

# Backend dev (run garage separately, or via docker-compose)
cd backend && swag init && CGO_ENABLED=0 go run .  # API on :8080

# Docker image
make build                                        # docker build -t ghcr.io/mimic890/kuraban:latest .

# Tests
make test                                         # Backend unit tests
make test-race                                    # With race detector
make test-cover                                   # Coverage gate (scripts/coverage-gate.sh)
make test-smoke                                   # Docker compose smoke tests
cd frontend && npm run test                       # Vitest frontend tests
cd frontend && npm run lint                       # ESLint
cd frontend && npm run build                      # tsc -b && vite build → dist/

# Swagger docs (generates backend/docs/ package, needed for Go build)
cd backend && swag init
```

## Known Gotchas

1. **Garage admin API v1 vs v2:** Factory probes `/v2/GetClusterHealth` at startup. A 404 means v1. v1 does not support `GetClusterStatistics`, `GetNodeInfo`, `GetNodeStatistics` — they return `ErrUnsupported`. The frontend gates these via `/api/v1/capabilities`.
2. **Object wildcard routing:** Object keys with slashes are `encodeURIComponent`'d on the frontend (producing `%2F`). The backend uses a `*` wildcard param + `url.QueryUnescape` to recover the full key. Do not change one side without the other.
3. **Credential cache:** `s3.go` caches per-bucket S3 credentials (`key:<bucket>:<op>`) for 1 hour. `InvalidateBucketCredsCache(bucket)` is called after `GrantBucketPermission`; `ClearAllCredsCache()` after `DeleteUser` / `UpdateUser`. If you add new permission-changing endpoints, add invalidation calls.
4. **Authz is UI-layer only:** `authz` package implements a permission policy but explicitly states it is NOT a security boundary — kuraban talks to Garage with a single admin token. Real isolation is at the network/Garage credential level.
5. **Directory markers:** `CreateDirectoryMarker` uses `PutObject` with `size=0` (not `-1`). minio-go with `size=-1` triggers multipart upload, which Garage rejects for empty bodies.
6. **`docs/` package not in git:** `swag init` generates `backend/docs/` from swagger comments. The Go build will fail without it (`import _ "Mimic890/kuraban/docs"`). Run `swag init` before `go build` if building outside Docker/CI.
7. **ListObjects StatObject fan-out:** `ListObjects` spawns up to 10 concurrent `StatObject` goroutines (semaphore-bounded) with 10s timeout each, to fetch `ContentType` which `ListObjectsV2` does not return.
8. **Upload re-entry guard:** `useBucketObjects.uploadFiles` has an `uploadingRef` guard — a second call while uploads are in progress is rejected with a toast.
9. **Selection clears on navigation:** `ObjectBrowserView` clears `selectedFileKeys`/`selectedFolderKeys` when `currentPath` changes, preventing bulk-delete of objects in a folder the user navigated away from.
10. **Stale fetch guard:** `useBucketObjects` uses a monotonic `fetchSeqRef` to discard responses from superseded fetches (e.g. rapid folder navigation).

## Data Model

Objects, buckets and keys live in Garage. Kuraban itself keeps only `data/state.json` (admin account, clusters) and `data/kuraban.db` (SQLite: runtime settings, metrics history). Key domain models (see `backend/internal/models/`):

| Model | Source | Key fields |
|---|---|---|
| `GarageBucketInfo` | Admin API GetBucketInfo | `ID`, `GlobalAliases`, `Keys[]BucketKeyInfo`, `Objects`, `Bytes`, `Quotas`, `WebsiteConfig` |
| `GarageKeyInfo` | Admin API GetKeyInfo | `AccessKeyID`, `SecretAccessKey` (only with `showSecret`), `Buckets[]KeyBucketInfo`, `Expiration`, `Expired` |
| `BucketKeyPermission` | Embedded in bucket/key | `Read`, `Write`, `Owner` (bools) |
| `ObjectInfo` | S3 ListObjectsV2 / StatObject | `Key`, `Size`, `LastModified`, `ETag`, `ContentType`, `StorageClass` |
| `ObjectListResponse` | S3 ListObjectsV2 | `Objects[]`, `Prefixes[]` (folders via delimiter), `IsTruncated`, `NextContinuationToken` |
| `ClusterHealth` | Admin API | `Status`, `NodesUp`, `NodesTotal`, `PartitionsAllOk`, `PartitionsQuorum` |
| `ClusterStatus` | Admin API | `Nodes[]NodeInfo`, `LayoutVersion` |
| `DashboardMetrics` | Aggregated backend | `TotalSize`, `ObjectCount`, `BucketCount`, `UsageByBucket[]` |

### Auth model
- **Admin auth:** username + password (HTTP basic → JWT).
- **Token auth:** Garage admin token → JWT.
- **OIDC:** redirect flow → cookie (Ed25519-signed JWT, `SessionMaxAge`).
- JWT stored as `auth-token` in `localStorage` (frontend) or cookie (OIDC).
- Auth config from `/auth/config`: `{ admin: {enabled}, oidc: {enabled, provider}, token: {enabled} }`.

### Access control (optional, `access_control` config block)
- Team-based: users belong to teams (via OIDC claims); teams get permissions scoped by `bucket_prefixes`.
- Permission vocabulary defined in `authz/vocabulary.go`. Admin-only perms (`key.create`, `key.delete`, etc.) not grantable to teams.
- When `access_control` is `nil`, the admin user has all permissions.
