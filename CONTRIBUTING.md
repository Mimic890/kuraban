# Contributing

Bug reports, feature requests and pull requests are welcome. Before opening an issue, check the [documentation](README.md#documentation) and existing issues.

## Reporting bugs

Include:

- Kuraban version (from the startup log or `GET /health`) and Garage version.
- How you deploy it: Docker or from source.
- Steps to reproduce, what you expected and what happened.
- Relevant backend logs. `KURABAN_LOGGING_LEVEL=debug` gives more detail. Remove tokens and keys before posting.

Report security issues privately through GitHub security advisories, not in public issues.

## Development setup

Requirements:

- Go 1.26
- Node.js 25 and npm
- [swag](https://github.com/swaggo/swag) for the API docs package: `go install github.com/swaggo/swag/cmd/swag@v1.16.6`
- Docker with Compose v2, for a local Garage and the smoke tests

A local Garage instance is described in [docs/garage-setup.md](docs/garage-setup.md). For development you can run only its `garage` service and expose ports `3900` and `3903` to the host.

### Backend

```bash
cd backend
swag init                     # generates backend/docs, required for the build
KURABAN_SERVER_ENVIRONMENT=development go run .
```

The API listens on http://localhost:8080. Settings are taken from environment variables and an optional `config.yaml`, see [docs/configuration.md](docs/configuration.md). State is written to `backend/data/`.

To fill a bucket with a large number of test objects, use `go run ./cmd/seed -count 10000`.

### Frontend

```bash
cd frontend
npm install
npm run dev
```

The Vite dev server runs on http://localhost:3000 and proxies `/api` and `/auth` to the backend on port 8080.

### Docker image

```bash
make build          # docker build -t ghcr.io/mimic890/kuraban:latest .
```

## Checks

Run these before opening a pull request. CI runs the backend checks and the smoke test on every pull request.

```bash
make test           # backend unit tests
make test-race      # backend tests with the race detector
make test-cover     # coverage with the minimum gate
make test-smoke     # end-to-end test against a real Garage in Docker

cd frontend
npm run lint
npm run test
npm run build
```

## Code style

- Go: `gofmt`, structured logging through `logpkg.FromCtx(ctx)`, handlers return `models.SuccessResponse` / `models.ErrorResponse`.
- TypeScript: ESLint config in the repo, the `@/` import alias, server state through TanStack Query.
- Add comments only where the reason for a decision isn't obvious from the code.
- New API routes need a permission declaration in `backend/internal/authz/vocabulary.go`. Startup fails without one.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the code layout and the non-obvious parts.

## Pull requests

- Keep a pull request to one change. Add or update tests for the behavior you change.
- Update the documentation if you change configuration, routes or user-visible behavior.
- By contributing you confirm that you wrote the code or have the right to submit it, and that it is licensed under the project license, [AGPL-3.0](LICENSE).

### Pull request titles

PR titles follow [Conventional Commits](https://www.conventionalcommits.org/). PRs are squash-merged, so only the title has to follow the format.

```
<type>(<optional-scope>): <subject>
```

| Type | Use for |
|---|---|
| `feat` | new user-facing functionality |
| `fix` | bug fix |
| `perf` | performance improvement |
| `docs` | documentation only |
| `refactor` | code change that is neither a feature nor a fix |
| `chore` | tooling, dependencies |
| `test` | tests |
| `ci` | CI workflows |
| `build` | build system |

Add `!` after the type for a breaking change (`feat!: ...`). Scopes are optional: `backend`, `frontend`, `ci`, `deps`.

```
feat(backend): add bucket quota enforcement
fix(frontend): correct theme toggle in Safari
chore(deps): bump axios from 1.15.0 to 1.15.2
docs: clarify OIDC setup
```
