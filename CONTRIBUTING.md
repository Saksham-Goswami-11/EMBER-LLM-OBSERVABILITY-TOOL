# Contributing to Ember

Thanks for considering it. Ember is early — that means there's a lot of
open ground, and also that conventions are still settling. This doc is the
short version; ask in an issue if something's unclear.

## Before you start

For anything beyond a typo fix, open an issue first and say what you plan
to do. That avoids two people building the same thing, and lets us agree on
approach before you've written the code.

Issues labeled [`good first issue`](https://github.com/sakshamgoswami/ember/labels/good%20first%20issue)
are scoped to be doable without deep context on the rest of the codebase.

## Project layout

```
cmd/ember/          entrypoint: flags, startup banner, HTTP server lifecycle
internal/model/      shared domain types (Trace, Span, Session, Project)
internal/ingest/      OTLP/HTTP receiver — the only way data gets in
internal/storage/    SQLite schema + queries — the only package that imports database/sql
internal/pricing/    model → USD/1M-token rate table
internal/api/         read-only REST API the dashboard is built on
internal/server/    wires ingest + api + embedded dashboard into one handler
web/                  React + TypeScript dashboard (Vite)
examples/demo-agent/  a real OTel program that sends sample traces — also doubles as
                      instrumentation-example documentation
```

## Local setup

```bash
go build ./...            # backend compiles standalone
cd web && npm install && npm run build   # dashboard build
cd .. && make run         # full stack on :8080
```

For hot-reload frontend work, run `npm run dev` in `web/` (proxies `/api`
and `/v1` to `:8080`) alongside `go run ./cmd/ember` in the repo root.

## Before opening a PR

```bash
go vet ./...
go test ./...
cd web && npx tsc --noEmit
```

## Conventions

- Business logic that touches SQL stays inside `internal/storage` — other
  packages call its methods, never `database/sql` directly. Same idea as a
  persistence boundary in a larger codebase: it keeps the schema
  changeable from one place.
- New OTel `gen_ai.*` attributes get extracted in `internal/ingest/otlp.go`
  defensively — check both the current and prior attribute name where the
  spec has renamed something, since it's still pre-1.0.
- Frontend: no new runtime dependency without a reason — the "single
  binary, minimal footprint" positioning is a real constraint, not just
  marketing copy.
- Keep comments to the *why*, not the *what*. Code should read clearly
  enough that the *what* doesn't need restating.

## Reporting a security issue

Please don't open a public issue for a security vulnerability. Email the
maintainer listed in the repository's GitHub profile instead.
