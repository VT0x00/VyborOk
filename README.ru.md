<div align="center">

[English](README.md) | [Русский](README.ru.md)

</div>

# VyborOk — VO 👍

Платформа для проведения опросов и голосований.

> **Статус:** ранний MVP. Аутентификация готова, опросы/голосование/статистика — в работе.

## Стек

| Слой | Технологии |
|---|---|
| Бэкенд | Go 1.21+, [micro v3](https://github.com/unistack-org/micro) (`micro-server-http/v3`) |
| API | Protocol Buffers (proto3) с аннотациями `micro.api.http` |
| БД | PostgreSQL 16 |
| Кэш | Redis 7 *(план, фаза 5)* |
| Очередь | NATS 2.10 *(план, фаза 6)* |
| Фронтенд | Angular *(план, фаза 7)* |
| Локальная инфра | Docker Compose |

## Быстрый старт

```bash
# 1. Поднять инфраструктуру (postgres, redis, nats)
docker compose up -d

# 2. Создать .env из примера
cp .env.example .env

# 3. Применить миграции
make migrate-up

# 4. Запустить сервер
make run
# → micro http server started addr=:8080
```

Проверка:

```bash
curl -s http://localhost:8080/health
# {"status":"ok"}
```

## API

| Метод | Путь | Auth | Описание |
|---|---|---|---|
| `GET`  | `/health`          | —   | Health check |
| `POST` | `/auth/register`   | —   | Регистрация, возвращает токены и профиль |
| `POST` | `/auth/login`      | —   | Вход, возвращает токены и профиль |
| `POST` | `/auth/refresh`    | —   | Обмен refresh-токена на новую пару |
| `GET`  | `/auth/me`         | JWT | Профиль текущего пользователя |
| `PUT`  | `/user/profile`    | JWT | Обновление профиля текущего пользователя |
| `GET`  | `/user/{username}` | —   | Публичный профиль (учитывает `is_private`) |

**Auth** = `JWT` означает, что нужен заголовок `Authorization: Bearer <access_token>`.

### Примеры

**Регистрация**

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

**Вход**

```bash
curl -s -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email": "alice@example.com", "password": "secret12345"}' | jq
```

Сохрани `access_token` из ответа — он понадобится ниже.

**Мой профиль (защищённый)**

```bash
curl -s http://localhost:8080/auth/me \
  -H "Authorization: Bearer $ACCESS" | jq
```

**Обновление профиля (защищённый)**

```bash
curl -s -X PUT http://localhost:8080/user/profile \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -d '{
    "bio": "Привет, я Алиса",
    "is_private": true,
    "is_private_set": true,
    "public_fields": ["bio", "avatar_url"],
    "public_fields_set": true
  }' | jq
```

Обрати внимание на пары `*_set`: proto3 не отличает «поле не передано» от «поле передано пустым/false», поэтому булевы флаги идут в паре с `*_set`. Если `is_private_set` = false, поле `is_private` не меняется.

**Публичный профиль**

```bash
# Открытый профиль → всё, кроме email
curl -s http://localhost:8080/user/alice | jq

# Закрытый профиль, аноним → только id, username и поля из public_fields, profile_hidden=true
curl -s http://localhost:8080/user/bob | jq

# Закрытый профиль, как владелец → полный профиль, включая email
curl -s http://localhost:8080/user/bob \
  -H "Authorization: Bearer $ACCESS" | jq
```

**Обновление токенов**

```bash
curl -s -X POST http://localhost:8080/auth/refresh \
  -H 'Content-Type: application/json' \
  -d "{\"refresh_token\": \"$REFRESH\"}" | jq
```

## Переменные окружения

Скопируй `.env.example` в `.env` и поправь. Дефолты рассчитаны на `docker compose`.

| Переменная | Дефолт | Описание |
|---|---|---|
| `APP_ENV`  | `dev`  | `dev` → текстовые логи DEBUG, `prod` → JSON логи INFO |
| `APP_PORT` | `8080` | Порт HTTP-сервера |
| `POSTGRES_HOST` | `localhost` | Хост PostgreSQL |
| `POSTGRES_PORT` | `5432` | Порт PostgreSQL |
| `POSTGRES_USER` | `postgres` | Пользователь PostgreSQL |
| `POSTGRES_PASSWORD` | `postgres` | Пароль PostgreSQL |
| `POSTGRES_DB` | `vyborok` | Имя БД |
| `POSTGRES_SSLMODE` | `disable` | PostgreSQL `sslmode` |
| `POSTGRES_MAX_OPEN_CONNS` | `25` | Максимум открытых соединений в пуле |
| `POSTGRES_MAX_IDLE_CONNS` | `5` | Максимум idle-соединений в пуле |
| `POSTGRES_CONN_MAX_LIFETIME` | `5m` | Максимальное время жизни соединения в пуле |
| `REDIS_HOST` | `localhost` | Хост Redis *(план, фаза 5)* |
| `REDIS_PORT` | `6379` | Порт Redis *(план)* |
| `REDIS_PASSWORD` | *(пусто)* | Пароль Redis, пусто = без авторизации *(план)* |
| `REDIS_DB` | `0` | Номер логической БД Redis *(план)* |
| `NATS_HOST` | `localhost` | Хост NATS *(план, фаза 6)* |
| `NATS_PORT` | `4222` | Клиентский порт NATS *(план)* |
| `JWT_SECRET` | `dev-secret-change-me-in-prod` | HMAC-секрет. **Только dev-заглушка — в prod замени на длинную случайную строку.** |
| `JWT_ACCESS_TTL`  | `15m` | Время жизни access-токена |
| `JWT_REFRESH_TTL` | `720h` (30 дней) | Время жизни refresh-токена |
| `JWT_ISSUER` | `vyborok` | JWT `iss` |

## Тесты

| Команда | Что делает |
|---|---|
| `make test`        | `go vet` + unit-тесты (`-race`) |
| `make test-all`    | `go vet` + **все** тесты с `TEST_DATABASE_URL` (`-race -count=1`) |
| `make test-auth`   | Только пакет auth (интеграционные, нужна тестовая БД) |
| `make test-repo`   | Только репозиторий (интеграционные, нужна тестовая БД) |
| `make test-db-reset` | Пересоздать и накатить миграции на `vyborok_test` |

Интеграционные тесты (repo, auth service) требуют запущенного PostgreSQL и `TEST_DATABASE_URL`; без переменной они скипаются через `t.Skip`. Тестовая БД **отдельная** (`vyborok_test`), рабочие данные не задеваются.

Первый запуск:

```bash
docker compose up -d
make test-db-reset
make test-all
```

## Структура проекта

```
backend/
├── main.go                       # entrypoint: конфиг, БД, micro-сервер, graceful shutdown
├── http/
│   ├── handler/                  # тонкий слой RPC (без бизнес-логики и SQL)
│   └── proto/                    # единый источник API: main.proto + сгенерированный код
├── internal/
│   ├── auth/                     # JWT, middleware, register/login/refresh/profile
│   ├── config/                   # env → структура
│   ├── models/                   # модели БД
│   └── repository/               # только SQL, наружу — repository.ErrNotFound
├── pkg/
│   └── database/                 # пул соединений
└── db/migrations/                # пары up/down для golang-migrate
```

## Соглашения

- Ошибки оборачиваем через `fmt.Errorf("контекст: %w", err)` — не проглатываем.
- Репозиторий переводит `sql.ErrNoRows` → `repository.ErrNotFound`; наружу SQL-ошибки не уходят.
- Handler тонкий: принять запрос → вызвать сервис → собрать ответ. Бизнес-логика — в `internal/*`.
- SQL — **только** в репозиториях, всегда с плейсхолдерами `$1, $2`.
- `ctx` передаётся через все методы сервисов и репозиториев.

## Дорожная карта

См. [TODO.md](TODO.md). Сейчас — **Фаза 2: CRUD опросов**.
