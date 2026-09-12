# Technical Specification for "VyborOk" Polling Platform

## 1. Project Goals and Purpose

Develop a high‑performance, scalable, and user‑friendly web platform for creating and conducting polls and surveys. The platform shall provide both core functionality (simple poll creation) and advanced features (user authentication, statistics with charts, flexible access settings, multimedia, embedding on external websites, anti‑fraud protection). The primary focus of the first version (MVP) is a rapid launch with key features, but with an architecture that allows easy extension in the future.

## 2. Functional Requirements

### 2.1. Landing Page (`/`)
- A landing page describing the platform’s features:
  - Create polls with various question types (single choice, multiple choice, scale, text answer).
  - Access control: public or link‑only.
  - Embedding on external websites via iframe.
  - Real‑time statistics with charts.
  - Security and protection (future).
- Prominent “Sign In” and “Sign Up” buttons.
- Responsive design for mobile and desktop devices.

### 2.2. Authentication and Profile Management
- **Registration**: by email and password with email verification (link sent via email). Password reset via email.
- **Login**: by email and password, returns JWT access and refresh tokens.
- **User profile** contains:
  - Avatar (uploaded image).
  - Unique username (changeable).
  - First name, last name, bio (description), links to social networks.
- **Privacy settings**:
  - **Closed profile** – completely inaccessible to unauthenticated users; authenticated users see only fields explicitly marked as public by the owner (by default nothing is visible). The `/username` page for such profiles shows a placeholder “Profile is hidden”.
  - **Open profile** – everyone can see the header with avatar, name, and the list of public polls created by the user.

### 2.3. Authenticated User Dashboard
- Header: avatar, username, “Settings” button to edit profile.
- Feed of polls created by the user (pagination). Each poll is displayed as a card containing:
  - Title, creation date, status (active / closed / draft).
  - Number of participants and percentage of votes.
  - Buttons: “Edit” (before first vote), “Close”, “Delete”, “View Statistics”, “Get Embed Code”.
- Floating “+” button to create a new poll.

### 2.4. Poll Creation and Management
- **Creation form** (step‑by‑step wizard):
  1. Title (required) and description (optional).
  2. Add questions:
     - Types: single choice, multiple choice, scale (1–5 or 1–10), text answer.
     - For choice questions – answer options (add/remove).
     - Ability to attach multimedia (image, video, audio) to each question.
       - Formats: JPEG, PNG, MP4, WebM, MP3, OGG.
       - Size limits: image – up to 10 MB, video – up to 100 MB, audio – up to 50 MB.
  3. Access settings:
     - **Public** – visible to everyone.
     - **Link‑only** – accessible only via direct link (not indexed by search engines).
     - Allow voting without authentication (enabled by default for public polls).
  4. **Geo‑restriction** – select countries (by voter IP address), e.g., only Russia.
  5. **Results visibility** – immediately after voting or after poll closure.
  6. **Automatic closing date/time** (optional).
- **Editing** allowed only until the first vote is received. After votes appear, editing is prohibited (only close or delete allowed).
- **Deletion** – poll is completely removed with all data.

### 2.5. Poll Page (Taking the Poll)
- Unique URL: `/poll/{id}`.
- Displays title, description, and all questions.
- **Authenticated users** – vote is linked to the account.
- **Unauthenticated users** – if allowed, they can vote without logging in; the vote is stored without any identifier (only the fact of the vote). A warning about possible manipulation is shown.
- After submission, a check is performed (for authenticated – whether the user has already voted). For anonymous – no check (in MVP).
- After voting (or after poll closure) the user is redirected to the results page.

### 2.6. Statistics and Charts
- The page `/poll/{id}/stats` contains:
  - Total number of participants.
  - For choice‑based questions – a pie chart (toggleable to a bar chart) with percentages and absolute numbers.
  - For scale questions – average, median, histogram distribution.
  - For text answers – a list of all answers with search capability.
  - A line chart showing the dynamics of votes over time (by days/hours).
- All charts are rendered on the client using Chart.js based on JSON data from the backend.
- **Export**:
  - CSV – raw data for each question.
  - PDF – a report with charts (generated on the client or server – initially we do client‑side with html2pdf, but the API provides the data).
- Statistics update in real time (cache in Redis is invalidated on each vote).

### 2.7. Embedding on External Websites
- In the dashboard, for each poll, an HTML code (iframe) is generated for embedding on third‑party resources.
- Example code:
  ```html
  <iframe src="https://vyborok.ru/embed/{poll_id}" width="100%" height="600" frameborder="0"></iframe>
  ```
- Settings (width, height, hide header) are passed via URL parameters.
- The page `/embed/{poll_id}` – a separate HTML page without header/footer, only the poll and voting button.

### 2.8. Anti‑Fraud Protection (postponed to phase 2)
- MVP does not include protection. Instead, a warning about risks is shown during poll creation.
- Future plans: CAPTCHA for unauthenticated voters, IP‑based limits, fingerprinting.

## 3. Non‑Functional Requirements

### 3.1. Performance and Scalability
- Initial target: **100 concurrent users** (voters) without degradation.
- Architecture must allow horizontal scaling (increasing replicas) without code changes.
- Caching (Redis) for all GET requests, especially polls and statistics.
- Use of message queues (NATS) for asynchronous tasks (email sending, statistics recalculation).

### 3.2. Fault Tolerance
- Each microservice (future) or component must have health checks.
- Use of Docker and orchestration (Kubernetes – future, initially Docker Compose on a VPS).
- PostgreSQL replication (master‑slave) and Redis (Sentinel) for high availability.

### 3.3. Security
- Password storage with bcrypt.
- JWT with short lifetime (access + refresh tokens).
- All endpoints except public ones (landing, poll page, public profiles, embed) require authentication.
- CORS configured for allowed domains (for widgets).
- Input validation on all data (via protobuf tags).
- HTTPS mandatory.

### 3.4. Responsiveness and Design
- Three breakpoints: desktop, tablet, mobile phone.
- Colour scheme: light (white/light beige) with an accent colour (warm orange or dark blue). Branding – use of the “ВО👍” icon.
- Support for two languages: Russian and English (via Angular i18n).

## 4. Architecture and Technology Stack

### 4.1. Architectural Choice
The project is built as a **monolithic service** in Go using the `go.unistack.org/micro/v3` framework (as in the reference project [bookserver-micro](https://github.com/VT0x00/bookserver-micro)). Inside the monolith, business domains (authentication, polls, voting, statistics, media, embedding) are clearly separated at the package and handler level. This simplifies development and deployment at the initial stage, but allows individual modules to be extracted into microservices later if needed.

### 4.2. Technology Stack

| Component | Technology | Note |
|-----------|------------|------|
| **Backend** | Go 1.21+ | |
| **Framework** | `go.unistack.org/micro/v3` | HTTP server, JSON codec, routing |
| **API definitions** | Protocol Buffers (proto3) | Single file `http/proto/main.proto` |
| **Code generation** | protoc‑gen‑go, protoc‑gen‑micro, protoc‑gen‑micro‑http, protoc‑gen‑openapiv3 | Automatic generation of clients, server stubs, OpenAPI documentation |
| **Database** | PostgreSQL 16 | Primary storage |
| **Cache** | Redis 7 | Sessions, poll caching, statistics |
| **Message queue** | NATS 2.10 | Asynchronous tasks (email, stats recalculation) |
| **Frontend** | Angular 16+ | TypeScript, RxJS, Angular Material or Tailwind |
| **Containerisation** | Docker + Docker Compose | For development and production (Kubernetes later) |
| **Monitoring** | Prometheus + Grafana | (planned, not in MVP) |
| **Logging** | structured (slog) | Built into micro |

### 4.3. Project Structure (following bookserver-micro)

```
VyborOk/
├── .env                     # environment variables
├── .gitignore
├── docker-compose.yml       # for development (all services)
├── Makefile                 # unified commands: make gen, make build, make run, make test
├── README.md
│
├── main.go                  # entry point, starts the server
├── main_test.go             # integration tests
├── tools.go                 # tools for go generate
├── go.mod, go.sum
│
├── internal/                # internal packages
│   ├── config.go            # load configuration from .env and env vars
│   ├── init_server.go       # initialise the micro HTTP server
│   ├── auth/                # authentication business logic
│   ├── poll/                # poll management business logic
│   ├── vote/                # voting business logic
│   ├── stats/               # statistics business logic
│   ├── media/               # file handling
│   ├── embed/               # embedding
│   └── models/              # data structures (ORM models)
│
├── http/
│   ├── handler/             # RPC method implementations (handlers)
│   │   ├── handler.go       # VyborokHandler structure, registration of all methods
│   │   ├── auth.go          # Register, Login, Logout, Refresh, VerifyEmail...
│   │   ├── poll.go          # CreatePoll, GetPoll, ListPolls, UpdatePoll, DeletePoll, ClosePoll...
│   │   ├── vote.go          # SubmitVote, HasVoted
│   │   ├── stats.go         # GetStats, ExportCSV, ExportPDF
│   │   ├── media.go         # UploadMedia
│   │   └── embed.go         # GetEmbed
│   ├── proto/               # Protobuf definitions
│   │   ├── main.proto       # main file with all API
│   │   ├── main.pb.go       # generated protobuf code
│   │   ├── main_micro.pb.go # generated micro interfaces
│   │   ├── main_micro_http.pb.go # HTTP bindings
│   │   └── apidocs.swagger.yml   # auto‑generated OpenAPI documentation
│   ├── gen.sh               # generation script (or make gen)
│   └── generate.go          # //go:generate directive for protoc
│
├── db/
│   └── migrations/          # PostgreSQL migrations (golang‑migrate)
│       ├── 000001_init.up.sql
│       └── 000001_init.down.sql
│
├── frontend/                # Angular application (separate project)
│   ├── src/...
│   ├── angular.json
│   ├── package.json
│   └── Dockerfile.dev       # development image
│
└── docs/                    # documentation
    ├── technical_specification.md  # this TZ
    └── user_manual.md
```

### 4.4. Key Architectural Decisions

- **Single proto file** – all RPC methods are described in `main.proto` with `micro.api.http` annotations for HTTP routing.
- **Automatic generation** – upon changing the proto, simply run `make gen` to update all interfaces and documentation.
- **Business logic layer** – extracted into `internal/` packages, each corresponding to its domain. Handlers (`http/handler`) call these services.
- **Repositories** – database access is performed via separate repositories (e.g., `internal/repository`), injected into services.
- **Caching** – services use Redis for caching query results.
- **Asynchronicity** – events (e.g., `vote.created`) are sent via NATS for statistics recalculation and email sending.

## 5. Data Model

### 5.1. PostgreSQL Tables

```sql
-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    username VARCHAR(50) UNIQUE NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    bio TEXT,
    links JSONB,            -- array of strings
    avatar_url VARCHAR(255),
    is_private BOOLEAN DEFAULT FALSE,
    public_fields JSONB,    -- array of fields visible when profile is private
    email_verified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Polls
CREATE TABLE polls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'active', -- active, closed, draft
    access_type VARCHAR(20) DEFAULT 'public', -- public, private
    allow_unauth BOOLEAN DEFAULT TRUE,
    geo_countries TEXT[],   -- array of ISO country codes
    show_results VARCHAR(20) DEFAULT 'immediate', -- immediate, after_close
    close_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Questions
CREATE TABLE questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    question_text TEXT NOT NULL,
    question_type VARCHAR(20) NOT NULL, -- single, multiple, scale, text
    order_index INT NOT NULL,
    scale_min INT DEFAULT 1,
    scale_max INT DEFAULT 5,
    media_url VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Options (for single/multiple)
CREATE TABLE options (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    option_text VARCHAR(255) NOT NULL,
    order_index INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Votes
CREATE TABLE votes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    option_id UUID NULL REFERENCES options(id) ON DELETE SET NULL,
    text_value TEXT,               -- for text answers
    scale_value INT NULL,          -- for scale
    ip_address INET NULL,
    session_id UUID NULL,          -- for anonymous votes (generated on client)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Performance indexes
CREATE INDEX idx_polls_user_id ON polls(user_id);
CREATE INDEX idx_questions_poll_id ON questions(poll_id);
CREATE INDEX idx_options_question_id ON options(question_id);
CREATE INDEX idx_votes_poll_id ON votes(poll_id);
CREATE INDEX idx_votes_user_id ON votes(user_id);
CREATE INDEX idx_votes_created_at ON votes(created_at);
```

### 5.2. Redis Cache
- `poll:{id}:data` – full poll object with questions and options (TTL 5 minutes).
- `poll:{id}:stats` – aggregated statistics (TTL 1 minute, invalidated on vote).
- `user:{username}:profile` – cached public profile (TTL 10 minutes).
- User sessions (refresh tokens) are stored in Redis with TTL corresponding to their lifetime.

## 6. API (defined via Protobuf)

All endpoints are listed in the proto file (see appendix). Main groups:

| Group | Methods | HTTP Method | Path |
|-------|---------|-------------|------|
| **Authentication** | Register | POST | /auth/register |
| | Login | POST | /auth/login |
| | Logout | POST | /auth/logout |
| | Refresh | POST | /auth/refresh |
| | VerifyEmail | GET | /auth/verify |
| | ForgotPassword | POST | /auth/forgot |
| | ResetPassword | POST | /auth/reset |
| **Profile** | GetProfile | GET | /user/{username} |
| | UpdateProfile | PUT | /user/profile |
| **Polls** | CreatePoll | POST | /polls |
| | GetPoll | GET | /polls/{id} |
| | ListPolls | GET | /polls |
| | UpdatePoll | PUT | /polls/{id} |
| | DeletePoll | DELETE | /polls/{id} |
| | ClosePoll | POST | /polls/{id}/close |
| | GetUserPolls | GET | /user/{username}/polls |
| **Voting** | SubmitVote | POST | /polls/{id}/vote |
| | HasVoted | GET | /polls/{id}/has-voted |
| **Statistics** | GetStats | GET | /polls/{id}/stats |
| | ExportCSV | GET | /polls/{id}/stats/export/csv |
| | ExportPDF | GET | /polls/{id}/stats/export/pdf |
| **Media** | UploadMedia | POST | /media/upload |
| **Embedding** | GetEmbed | GET | /embed/{poll_id} |

All methods use JSON in request/response bodies (except `UploadMedia`, which uses `multipart/form-data`).

Detailed message structures are provided in the proto file (see appendix).

## 7. Development Plan (for a single developer)

Development is split into iterations, each ending with a working functional block.

| Iteration | Name | Contents | Estimate (weeks) |
|-----------|------|----------|------------------|
| 0 | Infrastructure preparation | Docker Compose setup, project structure, Makefile, basic dependencies | 1 |
| 1 | Basic authentication and simple polls | Registration, login, create poll (single choice), public/link access, voting (authenticated and anonymous), primitive statistics (numbers), dashboard | 5 |
| 2 | Extended question types and management | Multiple choice, scale, text answer, poll edit/close/delete, Redis caching | 4 |
| 3 | Profiles and privacy | Avatar, profile, privacy settings, public user page | 3 |
| 4 | Statistics and charts | Charts (pie, bar, line), CSV export (PDF stub), real‑time updates | 3 |
| 5 | Geo‑restrictions and access improvements | Country selection, IP check during voting | 2 |
| 6 | Embedding | Embed page, iframe code generation, CORS | 2 |
| 7 | Media files | Upload images/video/audio for questions, avatar resizing | 3 |
| 8 | Final touches and infrastructure | Monitoring (Prometheus+Grafana), logging, optimisation, documentation | 3 |
| **Total** | | | **26 weeks** (~6.5 months) |

Parallel with development, tests are written (unit tests for business logic, integration tests for API) and documentation is maintained.

## 8. Infrastructure and Deployment

### 8.1. Development Environment
- **Docker Compose** is used to spin up all dependencies (PostgreSQL, Redis, NATS). Backend and frontend are run locally (or also in containers) with code mounted for hot‑reload.
- Environment variables are stored in `.env` (not committed).
- For backend, `air` is used for automatic reload on code changes.

### 8.2. Production Environment
- Deployment on a VPS using Docker Compose (or Kubernetes in the future).
- Images are built and pushed to a registry (GitHub Container Registry or Docker Hub).
- Nginx is used as a web server in front of the frontend (serves static files and proxies API).

### 8.3. Monitoring and Logging (planned, not in MVP)
- Prometheus for metrics, Grafana for dashboards.
- Centralised log collection (ELK or Loki) – deferred.

## 9. Documentation and Testing Requirements

- **API documentation** is automatically generated from the proto file in OpenAPI (Swagger) format.
- **User documentation** – instructions for using the service (creating polls, embedding) in Markdown.
- **Unit tests** cover business logic (packages `internal/...`) – target ≥70% coverage.
- **Integration tests** test API endpoints using a test database (SQLite or a separate container).
- **Load testing** (k6) – simulation of 100 concurrent voters, checking response times.

## 10. Conclusion

This Technical Specification fully describes the functional and non‑functional requirements for the “VyborOk” platform, as well as defining the architecture and development plan. The project is intended for a single developer, uses a modern stack (Go+micro+Protobuf on the backend and Angular on the frontend). The architecture is monolithic but with clear separation by business domains, allowing future extraction of microservices if needed. The development process is iterative, gradually building functionality. Implementation will take about 6.5 months.

---

**Appendix:** The full proto file with all message definitions and the service is provided as a separate document (included in the project).
