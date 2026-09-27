# VyborOk — Constitution

Non-negotiable principles. If a design decision violates one, stop and discuss.

## Architecture
1. **Monolith.** No microservices split before Phase 14 (CI/CD).
2. **API in `http/proto/main.proto`.** Codegen via `make gen`. Generated
   `*.pb.go` never edited by hand.
3. **Layered flow:** HTTP → middleware → handler → service → repository → DB.
   Each layer only calls the one below.
4. **SQL only in `internal/repository/`.** Services never see `*sql.DB` or `*sql.Tx`.
5. **Transactions** opened via `withTx(ctx, db, fn)` in `internal/repository/tx.go`.
   No ad-hoc `db.Begin()` scattered around.

## Domain
6. **Domain errors are sentinels** (`ErrNotFound`, `ErrInvalidInput`, `ErrForbidden`).
   Each domain owns its set (`internal/auth`, `internal/poll`, ...).
   Mapped to HTTP codes in the handler layer.
7. **Ownership checks live in SQL** — `WHERE id = $1 AND user_id = $2`.
   Missing row → `ErrNotFound`, never `ErrForbidden`. Don't leak existence
   of other users' rows.
8. **Immutable after creation:** questions and options. To change them,
   create a new poll. This protects future vote rows (Phase 3) that reference
   option IDs from cascade-deletion via UPDATE.
9. **`_set` companion fields** for optional booleans in proto3
   (`is_private_set`, `public_fields_set`, `anonymous_set`). False `_set`
   means "don't touch"; the value is ignored.
10. **Rune-counted lengths.** All user-facing string limits in runes
    (`utf8.RuneCountInString`), never bytes.

## Security
11. **JWT HS256.** Secret from env, no defaults in prod (enforce in Phase 13).
12. **bcrypt cost 10** for passwords. Password bytes limited to 72 (bcrypt truncation).
13. **Email is never exposed** in a public or hidden profile. Only the owner
    sees their email via `/auth/me` or owner-view of their own profile.

## Conventions
14. **Errors wrapped:** `fmt.Errorf("context: %w", err)`. Never ignored.
15. **`ctx` first argument** on every service and repository method.
16. **slog for logging.** Real error → `logger.Error`; client gets a neutral
    message via `map<Domain>Error`.

## Anti-patterns
- `fmt.Sprintf` into SQL strings.
- `_ = err` without a comment explaining why.
- Handler calling repository directly.
- Service touching `*sql.DB`.
- Editing files under `http/proto/*.pb.go`.
- Skipping `t.Skip` when `TEST_DATABASE_URL` is absent.

