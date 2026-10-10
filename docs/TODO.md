## Phase 0. Infrastructure and skeleton (completed)

- [x] **Day 1.** Go, `go mod init`, `main.go` with `/health`, first commit.
- [x] **Day 2.** `config.go` with environment variable loading.
- [x] **Day 3.** `docker-compose.yml` (PostgreSQL, Redis, NATS).
- [x] **Day 4.** `pkg/database/postgres.go` + DB connection.
- [x] **Day 5.** Migrations (`golang-migrate`), `users` table.
- [x] **Day 6–7.** proto + code generation (`protoc`, `protoc-gen-go-micro`), `make gen`.
- [x] **Day 8–9.** micro HTTP server, first RPC `Health`.
- [x] **Day 10–11.** `User` model, repository, integration tests.
- [x] **Day 12–13.** `Register`, `Login`, bcrypt, JWT, handler, error handling.
- [x] **Auth tests** — JWT + service.

## Phase 1. Authentication: middleware and protected routes (week 1)

- [x] **Day 14.** Middleware: parses `Authorization: Bearer`, puts `userID` into context. `UserIDFromContext`.
- [x] **Day 15.** RPC `Refresh` — exchange refresh token for a new token pair.
- [x] **Day 16.** RPC `GetMe` — protected, returns the current user's profile.
- [x] **Day 17.** RPC `UpdateProfile` — protected, updates username/bio/avatar/is_private.
- [x] **Day 18.** RPC `GetProfile` — public, returns profile by username (respecting `is_private`).
- [x] **Day 19.** Tests for middleware, Refresh, GetMe, UpdateProfile.
- [x] **Day 20.** Debugging, refactoring, endpoint documentation.

## Phase 2. Polls: CRUD (week 2)

- [x] **Day 21.** Migration: `polls`, `questions`, `options`. Models.
- [x] **Day 22.** `PollRepository`: `Create`, `GetByID`, `ListByUser`, `Update`, `Delete`.
- [x] **Day 23.** Repository for questions and options (nested).
- [x] **Day 24.** RPC `CreatePoll` (POST /polls) — create with questions and options.
- [x] **Day 25.** RPC `GetPoll` (GET /polls/{id}).
- [x] **Day 26.** RPC `ListPolls` (GET /polls) — with pagination.
- [x] **Day 27.** RPC `UpdatePoll`, `DeletePoll`, `ClosePoll`.
- [x] **Day 28.** Repository and poll service tests.

## Phase 3. Voting (week 3)

- [ ] **Day 29.** Migration: `votes`. Model. Indexes on `poll_id`, `user_id`.
- [ ] **Day 30.** `VoteRepository`: `Create`, `HasVoted`, `ListByPoll`.
- [ ] **Day 31.** RPC `SubmitVote` — accepts an array of answers, validates, saves.
- [ ] **Day 32.** RPC `HasVoted` — check whether the current user has already voted.
- [ ] **Day 33.** Verification: anonymous voting, authenticated voting, repeat voting.
- [ ] **Day 34.** VoteService tests.
- [ ] **Day 35.** Refactoring + documentation.

## Phase 4. Basic statistics (week 4)

- [ ] **Day 36.** RPC `GetStats` — aggregation per question (vote counts, percentages).
- [ ] **Day 37.** Aggregation for scales (mean, median, histogram).
- [ ] **Day 38.** Aggregation for text responses (list).
- [ ] **Day 39.** Time dynamics (by day/hour).
- [ ] **Day 40.** Redis caching with invalidation on vote.
- [ ] **Day 41.** StatsService tests.
- [ ] **Day 42.** Refactoring, documentation.

## Phase 5. Redis: sessions, logout, rate limiting (week 5)

- [ ] **Day 43.** Redis integration in `pkg/redis`, wired into main.
- [ ] **Day 44.** Store refresh tokens in Redis (for logout).
- [ ] **Day 45.** RPC `Logout` — removes refresh from Redis.
- [ ] **Day 46.** Access-token blacklist (optional).
- [ ] **Day 47.** Rate limiting by IP on `/auth/*` (brute-force protection).
- [ ] **Day 48.** Cache public polls in Redis.
- [ ] **Day 49.** Redis layer tests.
- [ ] **Day 50.** Refactoring, documentation.

## Phase 6. NATS and async (week 6)

- [ ] **Day 51.** NATS integration in `pkg/nats`, wired into main.
- [ ] **Day 52.** Publish `vote.created` events on vote.
- [ ] **Day 53.** Async stats recomputation from the event.
- [ ] **Day 54.** Email queue: registration confirmation.
- [ ] **Day 55.** Email delivery implementation (SMTP or SendGrid).
- [ ] **Day 56.** RPC `VerifyEmail`, RPC `ForgotPassword`, RPC `ResetPassword`.
- [ ] **Day 57.** Queue and email scenario tests.

## Phase 7. Frontend: Angular (weeks 7–10)

- [ ] **Week 7.** Angular init, layout, routing, i18n (ru/en).
- [ ] **Week 8.** Auth module: login/register forms, JWT interceptor, guard.
- [ ] **Week 9.** User cabinet: header, poll feed, "+" button.
- [ ] **Week 10.** Poll creation: step-by-step wizard, all question types.

## Phase 8. Frontend: polls, voting, statistics (weeks 11–13)

- [ ] **Week 11.** Poll page, voting, results display.
- [ ] **Week 12.** Statistics page: Chart.js, pie/bar/line charts.
- [ ] **Week 13.** Public profile, `/username` page, privacy settings.

## Phase 9. Geo restrictions (week 14)

- [ ] **Day 60.** GeoIP database (MaxMind GeoLite2), middleware to detect country.
- [ ] **Day 61.** Check `geo_countries` on `SubmitVote`.
- [ ] **Day 62.** Geo tests.
- [ ] **Day 63.** Country selector UI on poll creation.

## Phase 10. Media (week 15)

- [ ] **Day 64.** `pkg/storage` — local file storage.
- [ ] **Day 65.** RPC `UploadMedia` (multipart/form-data).
- [ ] **Day 66.** Avatar resize, MIME and size validation.
- [ ] **Day 67.** Integration with polls (`media_url` in questions).
- [ ] **Day 68.** Tests.
- [ ] **Day 69.** File upload UI.
- [ ] **Day 70.** Serve files via `/media/{path}`.

## Phase 11. Embed (week 16)

- [ ] **Day 71.** `/embed/{poll_id}` page without header/footer.
- [ ] **Day 72.** iframe snippet generation in the user cabinet.
- [ ] **Day 73.** CORS for embedding domains.
- [ ] **Day 74.** Parameters (width, height, title hiding).
- [ ] **Day 75.** Tests.

## Phase 12. CSV/PDF export (week 17)

- [ ] **Day 76.** RPC `ExportCSV` — raw data.
- [ ] **Day 77.** RPC `ExportPDF` — report with charts.
- [ ] **Day 78.** Export buttons on the frontend.
- [ ] **Day 79.** Tests.

## Phase 13. Infrastructure, monitoring, security (week 18)

- [ ] **Day 80.** Prometheus metrics (`/metrics`) for every layer.
- [ ] **Day 81.** Grafana dashboard: RPS, latency, errors.
- [ ] **Day 82.** Structured logs → Loki (or EFK).
- [ ] **Day 83.** Health checks for DB/Redis/NATS.
- [ ] **Day 84.** Security audit: CORS, rate limit, HTTPS, SQL injection.
- [ ] **Day 85.** Load testing (k6) — 100 concurrent users.
- [ ] **Day 86.** Optimize bottlenecks.

## Phase 14. CI/CD and deploy (week 19)

- [ ] **Day 87.** GitHub Actions: build and test on push to main.
- [ ] **Day 88.** Build Docker images and push to ghcr.io.
- [ ] **Day 89.** `docker-compose.prod.yml` on the VPS.
- [ ] **Day 90.** Deploy via SSH from GitHub Actions.
- [ ] **Day 91.** HTTPS via Let's Encrypt (certbot / traefik).
- [ ] **Day 92.** DB backups (`pg_dump` on cron).
- [ ] **Day 93.** Prod smoke tests.

## Phase 15. Documentation and polish (week 20)

- [ ] **Day 94.** Swagger/OpenAPI — auto-generated from proto.
- [ ] **Day 95.** User documentation on the landing page.
- [ ] **Day 96.** Developer README: how to run the project.
- [ ] **Day 97.** ADR: why monolith, micro, JWT, bcrypt.
- [ ] **Day 98.** UI onboarding tour (first login).
- [ ] **Day 99.** Empty states, error handling on the frontend.
- [ ] **Day 100.** Final test run and release.

## Post-MVP (backlog)

- [ ] CAPTCHA for anonymous voting.
- [ ] IP restrictions and fingerprinting for anti-fraud.
- [ ] OAuth: Yandex ID, Google, GitHub.
- [ ] Dark theme.
- [ ] Kubernetes manifests.
- [ ] JavaScript widget instead of iframe.
- [ ] WebSocket for real-time statistics.
- [ ] Mobile application -- (?)

