# VyborOk — Agent Instructions

## Project
Go 1.21+ backend for a polling platform. **micro v3** (NOT v4/v5), PostgreSQL 16,
monolith. Angular frontend is planned (Phase 7+). Full spec: `docs/SPEC.md`.

## Commands
- `make gen` — regenerate proto (`protoc-gen-go`, `protoc-gen-go-micro` v3)
- `make run` — start the server
- `make test` — `go vet` + unit tests (`-race`)
- `make test-all` — vet + all tests with `TEST_DATABASE_URL` (`-race -count=1`)
- `make test-db-reset` — drop + recreate + migrate `vyborok_test`

## micro v3 specifics (never use v4/v5 APIs)
- Middleware: `server.Hooks(...)` with `server.HookHandler`, NOT `WrapHandler`
- `req.Header()` returns `metadata.Metadata`, not `map[string]string`
- `server.HandlerFunc` and `server.FuncHandler` are different types
- `srv.Start()` / `srv.Stop()`, NOT `Run()` / `Shutdown(ctx)`
- Codegen flag: `components=micro\|http` (escaped pipe, no quotes)

## Code rules
- Wrap errors: `fmt.Errorf("context: %w", err)`. Never swallow.
- SQL only in `internal/repository/`, always `$1, $2` placeholders.
- Handler is thin: parse → call service → map response. No SQL, no business logic.
- `ctx` through every service and repository method.
- Compile-time check at bottom of each handler file:
  `var _ pb.VyborokServer = (*VyborokHandler)(nil)`
- Repositories return `repository.ErrNotFound`, never `sql.ErrNoRows`.

## Never
- Edit `*.pb.go` by hand — regenerate with `make gen`.
- Use `fmt.Sprintf` into SQL.
- Return `sql.ErrNoRows` upward from repository.
- Put business logic in handler.
- Add `time.Sleep` to prod code (tests only).
- Use `len(s)` for user-facing string length — use `utf8.RuneCountInString`.

## Testing rules
- Integration tests require `TEST_DATABASE_URL`, else `t.Skip`.
- Test DB is `vyborok_test`, never `vyborok`.
- Repo tests: real DB. JWT / middleware: no DB. Service tests: real DB.
- Handler tests: not yet established (decide in Phase 2).

## Spec-Driven Workflow
Before implementing any feature:
1. Read `specs/<feature>/requirements.md` — what & why.
2. Read `specs/<feature>/design.md` — how.
3. Read `specs/<feature>/tasks.md` — checklist.
4. Read `specs/constitution.md` — non-negotiable principles.

If any file is missing, create it from `specs/templates/` before writing code.
Do NOT write code if `tasks.md` does not exist.

Working directory conventions:
- Active specs: `specs/NNNN-feature-name/`
- Completed specs: `specs/_archive/NNNN-feature-name/`
- Decisions (long-lived): `docs/adr/NNNN-title.md`
