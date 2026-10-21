# Kuraban frontend

React single-page application for Kuraban. In production it is built into `dist/` and served by the Go backend. See the [root README](../README.md) for deployment.

## Stack

React 19, TypeScript, Vite, Tailwind CSS v4, TanStack Query, Zustand, React Router v7, Radix UI primitives, uPlot and recharts, Vitest.

## Development

Requires Node.js 25 and a backend running on port 8080 ([development setup](../CONTRIBUTING.md#development-setup)).

```bash
npm install
npm run dev
```

The dev server runs on http://localhost:3000 and proxies `/api` and `/auth` to `http://localhost:8080`. No `.env` file is needed.

| Command | Description |
|---|---|
| `npm run dev` | Dev server with hot reload |
| `npm run build` | Type check and production build into `dist/` |
| `npm run preview` | Serve the production build locally |
| `npm run lint` | ESLint |
| `npm run test` | Unit tests (Vitest) |
| `npm run test:watch` | Tests in watch mode |
| `npm run coverage` | Tests with coverage report |

## Layout

```
src/
  pages/              route-level pages
  components/
    ui/               Radix-based primitives
    layout/           app shell, sidebar, header, theming
    buckets/          object browser, upload, preview, dialogs
    dashboard/        dashboard panels
    charts/           time-series chart wrapper
    auth/             login forms and route guard
  hooks/              React Query hooks and page state
  lib/
    api.ts            API client (axios)
    query-client.ts   QueryClient and query keys
  store/              Zustand stores
  types/              shared types
```

Imports use the `@/` alias for `src/`. Code conventions and known pitfalls are described in [ARCHITECTURE.md](../ARCHITECTURE.md).

## Translations

The interface is available in English and Russian. Strings live in `src/lib/locales/en.json` and `ru.json`. When you add user-visible text, add the key to both files.
