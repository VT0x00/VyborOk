# VyborOk — Обзор архитектуры бэкенда

Краткое описание структуры. Полная спецификация: `docs/SPEC.md`. Правила для кода:
`AGENTS.md`. Непреложные принципы: `specs/constitution.md`.

## Основные библиотеки

| Назначение | Библиотека |
|---|---|
| **Веб-сервер (HTTP)** | `go.unistack.org/micro/v3` — фреймворк **micro v3** (не v4/v5); HTTP-транспорт `go.unistack.org/micro-server-http/v3`, JSON-кодек `go.unistack.org/micro-codec-json/v3` |
| **БД** | PostgreSQL 16; драйвер `github.com/lib/pq` поверх стандартного `database/sql` (пул в `pkg/database`) |
| API-контракт | proto3 (`http/proto/main.proto`, аннотации `micro.api.http`); кодоген `protoc-gen-go` + `protoc-gen-go-micro` v3 |
| Миграции БД | golang-migrate (`backend/db/migrations/`) |
| JWT | `github.com/golang-jwt/jwt/v5` (HS256, access + refresh) |
| Хэш паролей | `golang.org/x/crypto/bcrypt` (cost 10) |
| UUID | `github.com/google/uuid` |
| Конфигурация | env-переменные + `github.com/joho/godotenv` (`internal/config`) |
| Логирование | стандартная `log/slog` (JSON в prod, text в dev) |

## Структура каталогов

```
backend/
├── main.go                  # composition root: DI, запуск сервера, graceful shutdown
├── db/migrations/           # SQL-миграции (golang-migrate, up/down)
├── http/
│   ├── handler/             # HTTP-слой (тонкие хендлеры)
│   │   ├── handler.go       #   VyborokHandler: auth-эндпоинты + маппинг ошибок
│   │   └── poll.go          #   polls-эндпоинты (Create — реализован, остальное — stubs)
│   └── proto/               # main.proto + сгенерированные *.pb.go (не править руками!)
├── internal/
│   ├── auth/                # домен «auth»: Service, JWTManager, middleware, ошибки
│   ├── poll/                # домен «polls»: Service (валидация, правила), ошибки
│   ├── repository/          # единственный слой с SQL: UserRepository, PollRepository, withTx
│   ├── models/              # доменные модели: User, Poll, Question, Option
│   └── config/              # Load() — конфигурация из env
└── pkg/database/            # Connect() — пул *sql.DB (lib/pq), лимиты соединений
```

## Поток запроса

```
HTTP → micro HTTP-сервер (JWT hook) → handler → service → repository → PostgreSQL
```

1. **Middleware** — `JWTManager.Middleware()` зарегистрирован через `server.Hooks(...)`,
   тип `server.HookHandler`. Если заголовок `Authorization: Bearer <token>` есть и
   валиден — кладёт `userID` в `context` (`auth.UserIDFromContext`). Отсутствие
   заголовка не ошибка: публичные эндпоинты опираются на это.
2. **Handler** (`http/handler`) — тонкий слой: парсит pb-запрос → вызывает сервис →
   мапит доменные ошибки в `microerr.*` (400/401/403/404/500) и заполняет pb-ответ.
   Без SQL и бизнес-логики. В конце файла — compile-time check
   `var _ pb.VyborokServer = (*VyborokHandler)(nil)`.
3. **Service** (`internal/auth`, `internal/poll`) — валидация (rune-длины, лимиты,
   cross-field правила) и бизнес-логика. Возвращает доменные sentinel-ошибки
   (`ErrNotFound`, `ErrInvalidInput`, `ErrForbidden`, ...), каждая доменная
   подсистема владеет своим набором ошибок.
4. **Repository** (`internal/repository`) — весь SQL здесь: плейсхолдеры `$1, $2`,
   `fmt.Sprintf` в SQL запрещён. Возвращает `repository.ErrNotFound`, никогда
   `sql.ErrNoRows`. Транзакции — только через `withTx(ctx, db, fn)` (`tx.go`).
5. **DB** (`pkg/database`) — пул `*sql.DB` с лимитами (MaxOpenConns/MaxIdleConns/
   ConnMaxLifetime) из конфигурации.

Правило конститции: каждый слой вызывает только слой ниже; сервисы никогда не
видят `*sql.DB`/`*sql.Tx`; handler не ходит в repository напрямую.

## API (единый источник правды — `http/proto/main.proto`)

| Метод | Путь | Статус |
|---|---|---|
| GET | `/health` | реализован |
| POST | `/auth/register` | реализован |
| POST | `/auth/login` | реализован |
| POST | `/auth/refresh` | реализован |
| GET | `/auth/me` | реализован |
| PUT | `/user/profile` | реализован |
| GET | `/user/{username}` | реализован |
| POST | `/polls` (CreatePoll) | реализован |
| GET/PUT/DELETE/POST | `/polls/{id}*` (Get/List/Update/Delete/Close) | stubs `not_implemented` |

Кодогенерация: `make gen` (протопуть micro-proto закреплён в
`http/proto/generate.go`). Файлы `*.pb.go` не редактируются руками.

## Специфика micro v3 (не v4/v5)

- Middleware: `server.Hooks(...)` + `server.HookHandler`, **не** `WrapHandler`.
- `server.HandlerFunc` и `server.FuncHandler` — разные типы; middleware
  возвращает `server.FuncHandler`.
- `req.Header()` возвращает `metadata.Metadata`, не `map[string]string`.
- Жизненный цикл: `srv.Start()` / `srv.Stop()`, не `Run()` / `Shutdown(ctx)`.
- Флаг кодогена: `components=micro\|http` (экранированная пайп, без кавычек).

## Spec-driven workflow

Перед фичей читаются: `specs/<feature>/requirements.md` (что/почему, критерии
R1..Rn) → `design.md` (как: data model, API, service/repo слои, варианты) →
`tasks.md` (чек-лист T1..Tn + traceability R→T). Код без `tasks.md` не пишется.
Активные спеки — `specs/NNNN-feature-name/`, завершённые — `specs/_archive/`.

## Инфраструктура и окружение

- `docker-compose.yml`: PostgreSQL 16, Redis 7 (запланирован на Phase 5),
  NATS 2.10 JetStream (запланирован на Phase 6).
- Конфигурация — только env (`APP_PORT`, `POSTGRES_*`, `JWT_*`, ...),
  fallback-значения для dev в `internal/config`.
- Тесты: `make test` (vet + unit, `-race`), `make test-all` (с `TEST_DATABASE_URL`,
  тест-БД `vyborok_test`), `make test-db-reset`.
