### 1. Общие цели и видение (без изменений)

Создать высокопроизводительную, масштабируемую и удобную платформу для создания и проведения опросов. Первоочередная цель — запуск MVP с базовым функционалом, но с архитектурой, позволяющей легко расширять возможности в будущем.

---

### 2. Технологический стек (обновлён)

| Компонент | Технология | Обоснование |
|-----------|------------|-------------|
| **Фреймворк** | `go.unistack.org/micro/v3` | Используется в bookserver-micro, обеспечивает гибкую микросервисную архитектуру с поддержкой HTTP и gRPC |
| **HTTP-сервер** | `go.unistack.org/micro-server-http/v3` | Готовый HTTP-сервер поверх micro |
| **Кодек** | `go.unistack.org/micro-codec-json/v3` | Для работы с JSON в HTTP-запросах |
| **Логирование** | `go.unistack.org/micro/v3/logger/slog` | Структурированное логирование через slog |
| **API-определения** | **Protocol Buffers (proto3)** | Используется в bookserver-micro для описания всех API-эндпоинтов |
| **HTTP-аннотации** | `micro.api.http` | Позволяет декларативно описывать HTTP-маршруты в proto-файлах |
| **OpenAPI-документация** | `micro.openapiv3` | Автоматическая генерация Swagger-документации из proto |
| **База данных** | **PostgreSQL** (основная) + **Redis** (кэш) | Соответствует исходным требованиям (сочетание) |
| **Очереди** | **NATS** | Лёгкий и надёжный брокер сообщений |
| **Фронтенд** | **Angular** | Без изменений |
| **Контейнеризация** | **Docker** | Без изменений |

---

### 3. Структура проекта (обновлена под bookserver-micro)

```
VyborOk/
├── .env
├── .gitignore
├── docker-compose.yml
├── Makefile                    # единая точка входа для всех команд
├── README.md
│
├── main.go                     # точка входа (минималистичный, как в bookserver-micro)
├── main_test.go                # интеграционные тесты
├── tools.go                    # инструменты для генерации
├── go.mod                      # модуль: github.com/VT0x00/vyborok
├── go.sum
│
├── internal/                   # внутренние пакеты (не экспортируются)
│   ├── init_server.go          # инициализация HTTP-сервера (аналог bookserver-micro)
│   ├── config.go               # загрузка конфигурации
│   └── ...                     # другие внутренние пакеты
│
├── http/                       # HTTP-слой (аналогично bookserver-micro)
│   ├── handler/                # реализация обработчиков
│   │   ├── handler.go          # структура и методы ServerHandler
│   │   └── ...                 # вспомогательные файлы
│   ├── proto/                  # Protobuf-определения API
│   │   ├── main.proto          # все RPC-методы и сообщения
│   │   ├── main.pb.go          # сгенерированный Protobuf-код
│   │   ├── main_micro.pb.go    # сгенерированный micro-код
│   │   ├── main_micro_http.pb.go # HTTP-привязки для micro
│   │   ├── apidocs.swagger.yml # автоматически сгенерированная Swagger-документация
│   │   └── micro_errors.pb.go  # стандартные ошибки micro
│   ├── gen.sh                  # скрипт генерации кода из proto
│   └── generate.go             # директива go:generate
│
├── db/                         # базы данных и миграции
│   ├── migrations/             # SQL-миграции (PostgreSQL)
│   │   └── 000001_init.up.sql
│   └── sqlite3/                # (опционально) для тестов или локальной разработки
│
├── pkg/                        # публичные пакеты (можно использовать в других проектах)
│   ├── database/               # подключение к БД
│   ├── redis/                  # работа с кэшем
│   └── nats/                   # работа с очередью
│
├── frontend/                   # Angular-приложение
│   ├── src/...
│   ├── angular.json
│   └── package.json
│
└── docs/                       # документация
    ├── technical_specification.md
    └── ...
```

**Ключевые особенности структуры (по аналогии с bookserver-micro):**

1. **Корневой `main.go`** — минималистичный, только запускает сервер.
2. **`internal/init_server.go`** — содержит всю логику инициализации сервера: создание `http.Server`, регистрация обработчиков, настройка кодеков и логгера.
3. **`http/proto/main.proto`** — единый источник истины для API. Все эндпоинты описываются в proto-файле с использованием аннотаций `micro.api.http` для маршрутизации.
4. **`http/handler/`** — реализация всех RPC-методов, описанных в proto.
5. **`Makefile`** — стандартизирует команды: `make gen` (генерация кода из proto), `make build` (сборка), `make run` (запуск), `make test` (тесты).
6. **`tools.go`** — фиксирует версии инструментов генерации (protoc, плагины).

---

### 4. Архитектура микросервисов (обновлена)

Вместо отдельных микросервисов (Auth, Poll, Vote, Stats, Media, Embed) мы используем **один монолитный сервис** с чётким разделением по бизнес-областям внутри `http/handler/`. Это упрощает разработку и соответствует подходу bookserver-micro.

**Внутренняя структура обработчиков (`http/handler/`):**

```
http/handler/
├── handler.go          # основная структура ServerHandler, регистрация всех RPC
├── auth.go             # методы: Register, Login, Logout, Refresh, VerifyEmail...
├── poll.go             # методы: CreatePoll, GetPoll, ListPolls, UpdatePoll, DeletePoll...
├── vote.go             # методы: SubmitVote, HasVoted...
├── stats.go            # методы: GetStats, ExportCSV, ExportPDF...
├── media.go            # методы: UploadMedia, GetMedia...
└── embed.go            # методы: GetEmbed...
```

Каждый метод соответствует RPC, описанному в `http/proto/main.proto`.

**Регистрация обработчиков (аналог bookserver-micro):**

```go
// internal/init_server.go
func InitServer(ctx context.Context, reg register.Register) *httpsrv.Server {
    // ... настройка логгера, кодеков ...
    srv := httpsrv.NewServer(
        server.Address(":"+port),
        server.Name("vyborok"),
        server.Register(reg),
        server.Codec("application/json", jsoncodec.NewCodec()),
    )
    // Регистрация всех RPC-методов
    pb.RegisterVyborokServer(srv, handler.NewServerHandler())
    // ...
    return srv
}
```

---

### 5. API-определения (Protobuf)

Все API-эндпоинты описываются в едином proto-файле `http/proto/main.proto` с использованием аннотаций:

```protobuf
syntax = "proto3";
package vyborok;
option go_package = "github.com/VT0x00/vyborok/http/proto;pb";

import "tag/tag.proto";
import "api/annotations.proto";
import "openapiv3/annotations.proto";

service Vyborok {
    // === Аутентификация ===
    rpc Register(RegisterReq) returns (RegisterRsp) {
        option (micro.api.http) = { post: "/auth/register"; body: "*"; };
    }
    rpc Login(LoginReq) returns (LoginRsp) {
        option (micro.api.http) = { post: "/auth/login"; body: "*"; };
    }
    rpc Logout(EmptyReq) returns (EmptyRsp) {
        option (micro.api.http) = { post: "/auth/logout"; };
    }
    rpc Refresh(RefreshReq) returns (LoginRsp) {
        option (micro.api.http) = { post: "/auth/refresh"; body: "*"; };
    }
    rpc VerifyEmail(VerifyEmailReq) returns (EmptyRsp) {
        option (micro.api.http) = { get: "/auth/verify"; };
    }
    rpc ForgotPassword(ForgotPasswordReq) returns (EmptyRsp) {
        option (micro.api.http) = { post: "/auth/forgot"; body: "*"; };
    }
    rpc ResetPassword(ResetPasswordReq) returns (EmptyRsp) {
        option (micro.api.http) = { post: "/auth/reset"; body: "*"; };
    }

    // === Управление опросами ===
    rpc CreatePoll(CreatePollReq) returns (PollRsp) {
        option (micro.api.http) = { post: "/polls"; body: "*"; };
    }
    rpc GetPoll(GetPollReq) returns (PollRsp) {
        option (micro.api.http) = { get: "/polls/{id}"; };
    }
    rpc ListPolls(ListPollsReq) returns (ListPollsRsp) {
        option (micro.api.http) = { get: "/polls"; };
    }
    rpc UpdatePoll(UpdatePollReq) returns (PollRsp) {
        option (micro.api.http) = { put: "/polls/{id}"; body: "*"; };
    }
    rpc DeletePoll(DeletePollReq) returns (EmptyRsp) {
        option (micro.api.http) = { delete: "/polls/{id}"; };
    }
    rpc ClosePoll(ClosePollReq) returns (PollRsp) {
        option (micro.api.http) = { post: "/polls/{id}/close"; };
    }
    rpc GetUserPolls(GetUserPollsReq) returns (ListPollsRsp) {
        option (micro.api.http) = { get: "/user/{username}/polls"; };
    }

    // === Голосование ===
    rpc SubmitVote(SubmitVoteReq) returns (SubmitVoteRsp) {
        option (micro.api.http) = { post: "/polls/{id}/vote"; body: "*"; };
    }
    rpc HasVoted(HasVotedReq) returns (HasVotedRsp) {
        option (micro.api.http) = { get: "/polls/{id}/has-voted"; };
    }

    // === Статистика ===
    rpc GetStats(GetStatsReq) returns (StatsRsp) {
        option (micro.api.http) = { get: "/polls/{id}/stats"; };
    }
    rpc ExportCSV(ExportCSVReq) returns (ExportRsp) {
        option (micro.api.http) = { get: "/polls/{id}/stats/export/csv"; };
    }
    rpc ExportPDF(ExportPDFReq) returns (ExportRsp) {
        option (micro.api.http) = { get: "/polls/{id}/stats/export/pdf"; };
    }

    // === Медиа ===
    rpc UploadMedia(UploadMediaReq) returns (UploadMediaRsp) {
        option (micro.api.http) = { post: "/media/upload"; body: "*"; };
    }

    // === Встраивание ===
    rpc GetEmbed(GetEmbedReq) returns (EmbedRsp) {
        option (micro.api.http) = { get: "/embed/{poll_id}"; };
    }

    // === Профиль пользователя ===
    rpc GetProfile(GetProfileReq) returns (ProfileRsp) {
        option (micro.api.http) = { get: "/user/{username}"; };
    }
    rpc UpdateProfile(UpdateProfileReq) returns (ProfileRsp) {
        option (micro.api.http) = { put: "/user/profile"; body: "*"; };
    }
}
```

**Преимущества такого подхода:**
- Единый источник истины для API.
- Автоматическая генерация клиентского и серверного кода.
- Автоматическая генерация OpenAPI/Swagger-документации.
- Чёткое разделение между определением API и реализацией.

---

### 6. Процесс разработки (обновлён)

| Шаг | Описание | Команда |
|-----|----------|---------|
| 1 | Определить API в `http/proto/main.proto` | Редактирование proto-файла |
| 2 | Сгенерировать код из proto | `make gen` |
| 3 | Реализовать обработчики в `http/handler/` | Написание Go-кода |
| 4 | Написать тесты | `make test` |
| 5 | Собрать и запустить | `make run` |

**Makefile (адаптирован из bookserver-micro):**

```makefile
.PHONY: all clean init gen build run test test-coverage

HOST=127.0.0.1
PORT=8080
NAME=vyborok
GO_CMD=go
GO_TEST=$(GO_CMD) test
GO_BUILD=$(GO_CMD) build
BIN_DIR=./bin/

all: clean init gen build run

clean:
	rm -f $(BIN_DIR)* coverage.out

init:
	mkdir -p $(BIN_DIR)

gen:
	cd ./http/ && go generate

build: init gen
	$(GO_BUILD) -o $(BIN_DIR)$(NAME)

run: build
	$(BIN_DIR)$(NAME) $(HOST) $(PORT)

test:
	$(GO_TEST) -v -race ./...

test-coverage:
	$(GO_TEST) -coverprofile=coverage.out ./...
	$(GO_CMD) tool cover -html=coverage.out
```

---

### 7. Зависимости (go.mod)

```go
module github.com/VT0x00/vyborok

go 1.21

require (
    github.com/google/uuid v1.6.0
    github.com/jmoiron/sqlx v1.4.0
    go.unistack.org/micro-client-http/v3 v3.9.19
    go.unistack.org/micro-codec-json/v3 v3.10.3
    go.unistack.org/micro-proto/v3 v3.4.6
    go.unistack.org/micro-server-http/v3 v3.11.39
    go.unistack.org/micro/v3 v3.11.51
    google.golang.org/protobuf v1.36.11
)
```

---

### 8. Что остаётся без изменений

- Все функциональные требования (типы вопросов, управление опросами, мультимедиа, доступ, встраивание, статистика).
- Требования к БД (PostgreSQL + Redis).
- Требования к фронтенду (Angular, адаптивность, дизайн).
- Требования к безопасности и производительности.
- План работ и итерации.

---

### 9. Ключевые отличия от предыдущей версии ТЗ

| Аспект | Было | Стало |
|--------|------|-------|
| **Архитектура** | Отдельные микросервисы | Монолитный сервис с чётким разделением по бизнес-областям |
| **API-определение** | Swagger/OpenAPI вручную | Protobuf + автоматическая генерация |
| **HTTP-сервер** | Стандартный `net/http` | `micro-server-http` |
| **Генерация кода** | Нет | `make gen` из proto |
| **Структура проекта** | Произвольная | По аналогии с bookserver-micro |
| **Документация API** | Отдельный файл | Автоматически из proto (apidocs.swagger.yml) |

