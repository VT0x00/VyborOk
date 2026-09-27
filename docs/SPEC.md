# VyborOk — Technical Specification

## 1. Overview

VyborOk is a platform for creating and conducting surveys and polls. Users
register, create polls with multiple question types, share them, collect
responses, and view statistics.

**Status:** early MVP. Authentication is complete; the poll CRUD layer is
in active development. Voting, statistics, media, geo-restrictions, and the
frontend are planned.

## 2. Stack

| Layer        | Technology                                                      |
|--------------|-----------------------------------------------------------------|
| Backend      | Go 1.21+, micro v3 (`go.unistack.org/micro/v3`)                 |
| HTTP server  | `go.unistack.org/micro-server-http/v3`                          |
| API schema   | Protocol Buffers (proto3) with `micro.api.http` annotations     |
| DB           | PostgreSQL 16                                                   |
| Migrations   | golang-migrate                                                  |
| Cache        | Redis 7 *(planned, Phase 5)*                                    |
| Queue        | NATS 2.10 with JetStream *(planned, Phase 6)*                   |
| Frontend     | Angular *(planned, Phases 7–8)*                                 |
| Local infra  | Docker Compose                                                  |
| Logging      | `log/slog` (structured, JSON in prod, text in dev)              |

## 3. Architecture

Monolithic service with clear separation by business domain inside
`internal/`. No microservices split at the MVP stage; a single binary
handles all RPCs.

### Request flow

```
HTTP → micro server (with middleware hook) → handler → service → repository → DB
```

- **Handler** — thin. Parses request, calls service, maps domain errors to
  HTTP errors. No business logic, no SQL.
- **Service** — validation, orchestration, transactions across repositories.
- **Repository** — SQL only. Returns `repository.ErrNotFound` instead of
  `sql.ErrNoRows`. Never leaks SQL errors upward.
- **Middleware** — JWT authentication. Registered as `server.HookHandler`
  via `server.Hooks(...)`. If `Authorization: Bearer <access>` is present
  and valid, puts `userID` into `context.Context`. Missing header is not
  an error at this layer — public endpoints rely on this.

### API as single source of truth

All RPCs are defined in `http/proto/main.proto`. Code generation:

```
make gen   # runs protoc via http/proto/generate.go
```

Protoc plugins: `protoc-gen-go`, `protoc-gen-go-micro` (v3). Path to
micro-proto annotations is pinned in `generate.go`.

### Key micro v3 specifics

- `server.Hooks(...)` accepts `options.Hook` (`interface{}`); middleware is
  `server.HookHandler = func(server.FuncHandler) server.FuncHandler`.
- `server.HandlerFunc` and `server.FuncHandler` share a signature but are
  different types — middleware must return `server.FuncHandler`.
- Header access: `req.Header()` returns `metadata.Metadata`, not
  `map[string]string`.
- Server lifecycle: `srv.Start()` / `srv.Stop()`, not `Run()` / `Shutdown(ctx)`.
- Codegen flag: `components=micro\|http` (escaped pipe, no quotes).
- `go_micro_out` uses an underscore, not a hyphen.

## 4. API (implemented)

| Method | Path                | Auth | Description                              |
|--------|---------------------|------|------------------------------------------|
| GET    | `/health`           | —    | Health check                             |
| POST   | `/auth/register`    | —    | Register, returns tokens + profile       |
| POST   | `/auth/login`       | —    | Login, returns tokens + profile          |
| POST   | `/auth/refresh`     | —    | Exchange refresh token for a new pair    |
| GET    | `/auth/me`          | JWT  | Current user's full profile              |
| PUT    | `/user/profile`     | JWT  | Update current user's profile            |
| GET    | `/user/{username}`  | —    | Public profile (respects `is_private`)   |

### Auth error codes

| Domain error             | HTTP | Code                  |
|--------------------------|------|-----------------------|
| `ErrInvalidInput`        | 400  | `invalid_input`       |
| `ErrEmailTaken`          | 400  | `email_taken`         |
| `ErrUsernameTaken`       | 400  | `username_taken`      |
| `ErrInvalidCredentials`  | 401  | `invalid_credentials` |
| `ErrInvalidToken`        | 401  | `invalid_token`       |
| `ErrNotFound`            | 404  | `not_found`           |

### Profile visibility rules

- **Open profile**, any viewer → all fields except `email`.
- **Private profile**, owner (matching token) → full profile including email.
- **Private profile**, anonymous or other user → only `id`, `username`, and
  fields listed in `public_fields`; `profile_hidden = true`; email is never
  exposed.

## 5. Data model

### `users`

```
id              UUID PK
email           VARCHAR(255) UNIQUE NOT NULL
password_hash   VARCHAR(255) NOT NULL
username        VARCHAR(50)  UNIQUE NOT NULL
first_name      VARCHAR(100)
last_name       VARCHAR(100)
bio             TEXT
links           JSONB        NOT NULL DEFAULT '[]'
avatar_url      VARCHAR(255)
is_private      BOOLEAN      NOT NULL DEFAULT FALSE
public_fields   JSONB        NOT NULL DEFAULT '[]'
email_verified  BOOLEAN      NOT NULL DEFAULT FALSE
created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
```

### `polls`

```
id           UUID PK
user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
title        VARCHAR(255) NOT NULL
description  TEXT
status       VARCHAR(20)  NOT NULL DEFAULT 'active' CHECK IN ('active','closed')
anonymous    BOOLEAN      NOT NULL DEFAULT FALSE
created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
```

Indexes: `(user_id, created_at DESC)`, `(status, created_at DESC)`.

### `questions`

```
id              UUID PK
poll_id         UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE
type            VARCHAR(20) NOT NULL CHECK IN ('single','multiple','scale','text')
title           TEXT        NOT NULL
description     TEXT
position        INT         NOT NULL
scale_min       INT
scale_max       INT
text_max_length INT
created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
UNIQUE (poll_id, position)
```

### `options`

```
id          UUID PK
question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE
text        TEXT        NOT NULL
position    INT         NOT NULL
created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
UNIQUE (question_id, position)
```

## 6. Domain rules

### Polls

- A poll always belongs to a user (`user_id`).
- A poll is created with `status = 'active'`.
- Questions and options are **immutable** after creation. To change them,
  create a new poll. This protects future vote rows that reference option IDs
  from being cascade-deleted by an update.
- `title`, `description`, and `anonymous` can be updated on an active poll.
- `status` changes to `closed` via the dedicated `ClosePoll` RPC (Phase 2).
- Ownership checks live in the SQL `WHERE` clause (`WHERE id = $1 AND user_id = $2`),
  not in the service. Missing row → `ErrNotFound`, never `ErrForbidden`,
  to avoid leaking existence of other users' polls.

### Questions

- Types:
  - `single` — exactly one option from `options`.
  - `multiple` — one or more options from `options`.
  - `scale` — integer in `[scale_min, scale_max]`; `options` empty.
  - `text` — free text up to `text_max_length` characters.
- `single` and `multiple` require at least 2 options.
- `scale` requires `scale_min < scale_max`, range ≤ 100.
- `text` requires `0 < text_max_length ≤ 5000`.
- Positions are assigned by the service as `0, 1, 2, ...` in array order.

### `_set` companion fields

Because proto3 cannot distinguish "field not sent" from "field set to
false/empty", optional booleans use a `field` + `field_set` pair:

- `is_private` + `is_private_set` (UpdateProfile)
- `public_fields` + `public_fields_set` (UpdateProfile)
- `anonymous` + `anonymous_set` (UpdatePoll)

If `*_set` is false, the field is left unchanged in the DB, even if a value
was sent. This lets a client send a partial update without clobbering fields
it didn't intend to touch.

### Validation limits

| Field                | Limit        |
|----------------------|--------------|
| `title` (poll)       | 1–255 chars  |
| `description` (poll) | 0–5000 chars |
| Question title       | 1–500 chars  |
| Option text          | 1–500 chars  |
| Questions per poll   | 1–50         |
| Options per question | 2–50         |
| Scale range          | ≤ 100        |
| Text max length      | 1–5000       |
| Password             | 8–72 bytes   |
| Username             | 3–30 chars, `[a-zA-Z0-9_]` |
| Email                | RFC-ish regex |

All lengths count **runes**, not bytes.

## 7. Conventions

- Errors are wrapped with `fmt.Errorf("context: %w", err)`; never swallowed.
- Repositories translate `sql.ErrNoRows` → `repository.ErrNotFound`.
- Handlers log the real error (`slog.Error`) and return a neutral message
  via domain-specific mappers (`mapAuthError`, `mapPollError`).
- `ctx` is passed through every service and repository method.
- SQL placeholders are always `$1, $2, ...`; no `fmt.Sprintf` into queries.
- Transactions are opened via a small `withTx(ctx, db, fn)` helper in
  `internal/repository/tx.go`.
- Compile-time interface check at the bottom of each handler file:
  `var _ pb.VyborokServer = (*VyborokHandler)(nil)`.

## 8. Testing

| Scope       | How it runs                                              |
|-------------|----------------------------------------------------------|
| JWT         | Unit, no DB                                              |
| Middleware  | Unit, mock `server.Request`                              |
| Repository  | Integration, real DB, `t.Skip` if `TEST_DATABASE_URL` unset |
| Services    | Integration, real DB                                     |
| Handlers    | *(not yet covered — planned)*                            |

Test DB is `vyborok_test`, separate from `vyborok`.

```bash
make test           # go vet + unit tests (-race)
make test-all       # go vet + all tests with TEST_DATABASE_URL (-race -count=1)
make test-db-reset  # drop + recreate + migrate vyborok_test
```

## 9. Roadmap

### Phase 0 — Infrastructure ✅
Go module, `main.go`, config, Docker Compose, DB connection, migrations,
proto + codegen, micro HTTP server, first RPC, user model + repository,
register/login/JWT.

### Phase 1 — Auth: middleware and protected routes ✅
Bearer middleware, `Refresh`, `GetMe`, `UpdateProfile`, `GetProfile`,
integration tests.

### Phase 2 — Polls CRUD *(in progress)*
- [x] Day 21: migrations `polls`/`questions`/`options`, models.
- [x] Day 22: `PollRepository` — `Create`, `GetByID`, `ListByUser`, `Update`,
      `Delete` (with nested questions/options in one transaction).
- [x] Day 24: `CreatePoll` RPC — proto, service, handler, tests.
- [ ] Day 25: `GetPoll`.
- [ ] Day 26: `ListPolls` (with pagination).
- [ ] Day 27: `UpdatePoll`, `DeletePoll`, `ClosePoll`.
- [ ] Day 28: service tests, refactor, docs.

### Phase 3 — Voting
`votes` table, `VoteRepository`, `SubmitVote`, `HasVoted`, anonymous vs
authenticated, repeat-vote protection.

### Phase 4 — Basic statistics
`GetStats` (counts and percentages), aggregation for scale (mean, median,
histogram) and text (list), time-series, Redis caching, tests.

### Phase 5 — Redis: sessions, logout, rate limiting
Refresh tokens stored in Redis, `Logout`, optional access-token blacklist,
rate limiting on `/auth/*`, caching of public polls.

### Phase 6 — NATS and async
`vote.created` events, async stats recomputation, email queue, email
verification, forgot/reset password.

### Phases 7–8 — Angular frontend
Layout, routing, i18n (ru/en), auth module, JWT interceptor, guards, user
cabinet, poll creation wizard, poll view and voting, stats page with Chart.js,
public profile page.

### Phase 9 — Geo restrictions
GeoIP middleware, per-poll `geo_countries` filter on vote, UI country selector.

### Phase 10 — Media
Local file storage, `UploadMedia` (multipart), avatar resize, MIME and size
validation, media URLs in questions, serving via `/media/{path}`.

### Phase 11 — Embed
`/embed/{poll_id}` page, iframe snippet generator, CORS for embedding domains,
configurable width/height/title hiding.

### Phase 12 — Export
`ExportCSV` (raw data), `ExportPDF` (report with charts).

### Phase 13 — Monitoring and security
Prometheus metrics, Grafana dashboards, structured logs to Loki/EFK, health
checks for DB/Redis/NATS, security audit (CORS, rate limits, HTTPS, SQLi),
load testing with k6.

### Phase 14 — CI/CD
GitHub Actions build + test on push, Docker images to ghcr.io,
`docker-compose.prod.yml`, deploy via SSH, HTTPS via Let's Encrypt,
DB backups via `pg_dump` cron, prod smoke tests.

### Phase 15 — Documentation and polish
Swagger/OpenAPI auto-generated from proto, user docs, developer README,
ADR (why monolith, why micro, why JWT), UI onboarding tour, empty states,
final test run and release.

## 10. Post-MVP backlog

- CAPTCHA for anonymous voting.
- IP and fingerprint restrictions for anti-fraud.
- OAuth: Yandex ID, Google, GitHub.
- Dark theme.
- Kubernetes manifests.
- JavaScript widget instead of iframe.
- WebSocket for real-time statistics.
- Mobile application.
