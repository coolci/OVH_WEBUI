# Changelog

All notable changes to **OVH_WEBUI** are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and [Conventional Commits](https://www.conventionalcommits.org/).

## [0.5.0] - 2026-11-20

### Added
- **Repository Compliance & Governance**: Added AGPL-3.0 `LICENSE`, upstream attribution `NOTICE` (`gokele/ovh`), and `CONTRIBUTING.md`.
- **CI / Release Automation**: Added `.github/workflows/ci.yml` (`gofmt`, `go vet`, `golangci-lint`, `go test -race`, cross-platform build, ESLint, TypeScript, Vitest + regression suite, Vite build, route ledger & tooling proofs) and `.github/workflows/release.yml`.
- **Code Quality Tooling**: Added `server/.golangci.yml`, `web/.prettierrc.json`, `web/vitest.config.ts`, and AST layering gate (`TestHandlersDoNotAccessDBDirectly`).
- **Frontend Unit Tests (Vitest)**: Added co-located native TypeScript Vitest tests under `web/src/lib/*.test.ts` (`order-duration`, `server-sort`, `history-sort`, `drag-selection`, `variant-stock`, `order-inputs`, `stock-plans`).

### Changed
- **Repository Layout (Plan A — Upstream Aligned)**: Moved Go backend to `server/`, frontend to self-contained `web/`, deployment manifests to `deploy/`, and archived historical audit artifacts to `docs/archive/`.
- **TypeScript Strictness**: Enabled `noUncheckedIndexedAccess: true` in `web/tsconfig.app.json` and hardened 83 indexed lookups across 28 files.
- **Frontend Consolidation**: Unified all HTTP requests on `web/src/lib/http.ts`, removed dead `web/src/lib/api.ts`, `web/src/lib/api-client.ts`, orphaned `web/src/routes/settings-sections.tsx`, template `lovable-tagger`, and 21 duplicate `hooks/use-*.ts` re-export shims; optimized Vite `manualChunks` (`index` chunk reduced from 529 kB to 284 kB, zero chunk-size warnings).
- **Backend Architecture & Modularization**:
  - Decoupled HTTP handlers from direct `state.DB` access via `app.State` order-cancellation methods.
  - Eliminated all >1000-line production Go files by splitting `server_control_*`, `account*`, `telegram_*`, `purchase`, `monitor`, `catalog`, `price`, `app`, and `vps` into cohesive single-responsibility files.
- **Documentation**: Unified `README.md` and `README.en.md` across paths (`server/` + `web/` + `deploy/`), ports (`:19998` / `:8080`), SQLite path (`sniper.db`), and React Router v6 documentation.

### Security & Safety
- Preserved all 79 audit regression fixes (`FIXED_VERIFIED`), 272/272 OpenAPI & route ledger parity, OVH path traversal safety AST checks, Nginx `/assets/` & `/api/internal` hardening, and disabled Auto-Pay / `catalogwatch` auto-order safeguards.
