<div align="center">

[English](README.md) | [Русский](README.ru.md)

</div>

# VyborOk — VO 👍

A platform for conducting surveys and polls.

> **Status:** early MVP. Auth is done, polls/votes/stats are in progress.

## Stack

| Layer | Tech |
|---|---|
| Backend | Go 1.21+, [micro v3](https://github.com/unistack-org/micro) (`micro-server-http/v3`) |
| API | Protocol Buffers (proto3) with `micro.api.http` annotations |
| DB | PostgreSQL 16 |
| Cache | Redis 7 *(planned, phase 5)* |
| Queue | NATS 2.10 *(planned, phase 6)* |
| Frontend | Angular *(planned, phase 7)* |
| Local infra | Docker Compose |

## Quick start

```bash
# 1. Start infra (postgres, redis, nats)
docker compose up -d

# 2. Create your .env from the example
cp .env.example .env

# 3. Apply migrations
make migrate-up

# 4. Run the server
make run
# → micro http server started addr=:8080
```

Check that it's alive:

```bash
curl -s http://localhost:8080/health
# {"status":"ok"}
```

## API

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET`  | `/health`          | —   | Health check |
| `POST` | `/auth/register`   | —   | Register a new user, returns tokens + profile |
| `POST` | `/auth/login`      | —   | Login, returns tokens + profile |
| `POST` | `/auth/refresh`    | —   | Exchange a refresh token for a new pair |
| `GET`  | `/auth/me`         | JWT | Get the current user's profile |
| `PUT`  | `/user/profile`    | JWT | Update the current user's profile |
| `GET`  | `/user/{username}` | —   | Public profile (respects `is_private`) |

**Auth** column: `JWT` means the request must include an `Authorization: Bearer <access_token>` header.

### Examples

**Register**

```bash
curl -s -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "alice@example.com",
    "password": "secret12345",
    "username": "alice",
    "first_name": "Alice",
    "last_name": "Doe"
  }' | jq
```

**Login**

```bash
curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "alice@example.com", "password": "secret12345"}' | jq
```

Save the `access_token` from the response and use it below.

**Get my profile (protected)**

```bash
curl -s http://localhost:8080/auth/me \
  -H "Authorization: Bearer $ACCESS" | jq
```

**Update my profile (protected)**

```bash
curl -s -X PUT http://localhost:8080/user/profile \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -d '{
    "bio": "Hello, I am Alice",
    "is_private": true,
    "is_private_set": true,
    "public_fields": ["bio", "avatar_url"],
    "public_fields_set": true
  }' | jq
```

Note the `*_set` companions: because proto3 cannot distinguish "not sent" from "set to false/empty", a boolean like `is_private` is accompanied by `is_private_set`. If `is_private_set` is false, the field is left unchanged.

**Public profile**

```bash
# Open profile → all fields except email
curl -s http://localhost:8080/user/alice | jq

# Private profile, anonymous → only id, username, and public_fields, profile_hidden=true
curl -s http://localhost:8080/user/bob | jq

# Private profile, as the owner → full profile including email
curl -s http://localhost:8080/user/bob \
  -H "Authorization: Bearer $ACCESS" | jq
```

**Refresh**

```bash
curl -s -X POST http://localhost:8080/auth/refresh \
  -H 'Content-Type: application/json' \
  -d "{\"refresh_token\": \"$REFRESH\"}" | jq
```

## Environment variables

Copy `.env.example` to `.env` and adjust. Defaults are tuned for `docker compose`.

| Variable | Default | Description |
|---|---|---|
| `APP_ENV`  | `dev`  | `dev` → text logs at DEBUG, `prod` → JSON logs at INFO |
| `APP_PORT` | `8080` | HTTP listen port |
| `POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `POSTGRES_PORT` | `5432` | PostgreSQL port |
| `POSTGRES_USER` | `postgres` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL password |
| `POSTGRES_DB` | `vyborok` | Database name |
| `POSTGRES_SSLMODE` | `disable` | PostgreSQL `sslmode` |
| `POSTGRES_MAX_OPEN_CONNS` | `25` | Max open connections in the pool |
| `POSTGRES_MAX_IDLE_CONNS` | `5` | Max idle connections in the pool |
| `POSTGRES_CONN_MAX_LIFETIME` | `5m` | Max lifetime of a pooled connection |
| `REDIS_HOST` | `localhost` | Redis host *(planned, phase 5)* |
| `REDIS_PORT` | `6379` | Redis port *(planned)* |
| `REDIS_PASSWORD` | *(empty)* | Redis password, empty = no auth *(planned)* |
| `REDIS_DB` | `0` | Redis logical DB index *(planned)* |
| `NATS_HOST` | `localhost` | NATS host *(planned, phase 6)* |
| `NATS_PORT` | `4222` | NATS client port *(planned)* |
| `JWT_SECRET` | `dev-secret-change-me-in-prod` | HMAC secret. **Dev placeholder only — replace with a long random string in prod.** |
| `JWT_ACCESS_TTL`  | `15m` | Access-token lifetime |
| `JWT_REFRESH_TTL` | `720h` (30d) | Refresh-token lifetime |
| `JWT_ISSUER` | `vyborok` | JWT `iss` claim |

## Tests

| Command | What it does |
|---|---|
| `make test`        | `go vet` + unit tests (`-race`) |
| `make test-all`    | `go vet` + **all** tests with `TEST_DATABASE_URL` set (`-race -count=1`) |
| `make test-auth`   | Auth package only (integration, needs test DB) |
| `make test-repo`   | Repository package only (integration, needs test DB) |
| `make test-db-reset` | Drop + recreate + migrate the `vyborok_test` database |

Integration tests (repo, auth service) require a running PostgreSQL and `TEST_DATABASE_URL`; without it they `t.Skip`. The test DB is a **separate** database (`vyborok_test`) so tests never touch your dev data.

First-time setup:

```bash
docker compose up -d
make test-db-reset
make test-all
```

## Project layout

```
backend/
├── main.go                       # entrypoint: config, DB, micro server, graceful shutdown
├── http/
│   ├── handler/                  # thin HTTP/RPC layer (no business logic, no SQL)
│   └── proto/                    # single source of truth: main.proto + generated code
├── internal/
│   ├── auth/                     # JWT, middleware, register/login/refresh/profile
│   ├── config/                   # env → struct
│   ├── models/                   # DB models
│   └── repository/               # SQL only, returns repository.ErrNotFound
├── pkg/
│   └── database/                 # pgx/pg connection pool
└── db/migrations/                # golang-migrate up/down pairs
```

## Conventions

- Errors are wrapped with `fmt.Errorf("context: %w", err)` — never swallowed.
- Repositories translate `sql.ErrNoRows` → `repository.ErrNotFound`; callers never see SQL errors.
- Handlers are thin: parse → call service → map to response. Business logic lives in `internal/*`.
- SQL lives **only** in repositories, always with `$1, $2` placeholders.
- `ctx` is passed through every service and repo method.

## Roadmap

See [TODO.md](TODO.md). Currently on **Phase 2 — Polls: CRUD**.
