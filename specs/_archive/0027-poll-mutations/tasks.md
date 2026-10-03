# Tasks: Poll Mutations

**Spec ID:** 0027
**Depends on:** `requirements.md` (approved), `design.md` (approved)

## Checklist

- [x] **T1 — Decide HTTP code for `ErrPollClosed`.**
  File: `http/handler/poll.go` (function `mapPollError`).
  Try `microerr.Conflict("poll_closed", ...)`. If the helper does not exist
  in `go.unistack.org/micro/v3/errors`, keep `microerr.BadRequest(...)`.
  Do NOT spend time on workarounds. Record the outcome in this file:
  `## Notes / T1: <BadRequest|Conflict>`.
  Expected: `go build ./...` green.

- [x] **T2 — Add `mustUserID` helper to `http/handler/handler.go`.**
  Refactor `GetMe`, `UpdateProfile`, `CreatePoll` to use it (three call
  sites). Behavior unchanged. Update `GetProfile` to keep using the existing
  `if id, ok := ...` pattern (that method deliberately allows anonymous).
  Expected: `make test-all` green (existing tests still pass).

- [x] **T3 — Add `PollRepository.Close`.**
  File: `internal/repository/poll_repository.go`.
  `Close(ctx, id, userID uuid.UUID) error`. SQL per design. 0 rows → `ErrNotFound`.
  Expected: `go build ./...` green.

- [x] **T4 — Repository tests for `Close`.**
  File: `internal/repository/poll_repository_test.go`.
  - `TestPollRepository_Close`: create poll; close it; `GetByID` → `status: "closed"`.
  - `TestPollRepository_Close_Idempotent`: close twice; second call returns nil.
  - `TestPollRepository_Close_WrongOwner`: close with other user's id → `ErrNotFound`; poll still active.
  - `TestPollRepository_Close_NotFound`: random UUID → `ErrNotFound`.
  Expected: `make test-all` green.

- [x] **T5 — Service methods `Update`, `Delete`, `Close`.**
  File: `internal/poll/service.go`.
  - `UpdateInput` struct (Title, Description, Anonymous, AnonymousSet).
  - `Update(ctx, id, userID uuid.UUID, in UpdateInput) (*models.Poll, error)`.
  - `Delete(ctx, id, userID uuid.UUID) error`.
  - `Close(ctx, id, userID uuid.UUID) (*models.Poll, error)`.
  Reuse `validateTextLen`, `maxTitleLen`, `maxDescriptionLen`.
  Ownership check via `poll.UserID != userID → ErrNotFound`.
  Status check in `Update`: `poll.Status == closed → ErrPollClosed`.
  Expected: `go build ./...` green.

- [x] **T6 — Service tests for Update.**
  File: `internal/poll/service_test.go`.
  - `TestService_Update_TitleOnly`: R2 — title changed, description/anonymous unchanged, questions preserved.
  - `TestService_Update_NothingChanged`: R3 — empty input → no-op, 200.
  - `TestService_Update_AnonymousSetTrue` / `_SetFalse`: R4.
  - `TestService_Update_TitleTooLong`: R5 — `ErrInvalidInput`.
  - `TestService_Update_WrongOwner` / `_NotFound`: R6.
  - `TestService_Update_PollClosed`: R7 — close poll first via `psql` or `svc.Close`; update → `ErrPollClosed`.
  Expected: `make test-all` green.

- [x] **T7 — Service tests for Delete.**
  File: `internal/poll/service_test.go`.
  - `TestService_Delete_Active`: R9 — poll and questions gone.
  - `TestService_Delete_Closed`: R10 — allowed.
  - `TestService_Delete_WrongOwner` / `_NotFound`: R11.
  - `TestService_Delete_Twice`: R12 — second call → `ErrNotFound`.
  Expected: `make test-all` green.

- [x] **T8 — Service tests for Close.**
  File: `internal/poll/service_test.go`.
  - `TestService_Close_Active`: R14 — status changes, questions loaded.
  - `TestService_Close_Idempotent`: R15 — second call → still closed, no error.
  - `TestService_Close_WrongOwner` / `_NotFound`: R16.
  - `TestService_Close_ThenGetPoll_NonOwner`: R17 — cross-check with `GetByID`.
  Expected: `make test-all` green.

- [x] **T9 — Implement handlers `UpdatePoll`, `DeletePoll`, `ClosePoll`.**
  File: `http/handler/poll.go`.
  Replace three stubs. Use `mustUserID` from T2. Parse `req.Id` with
  `uuid.Parse` → `400 invalid_input` on failure.
  Expected: `go build ./...` green.

- [x] **T10 — Manual verification via curl.**
  Setup: register owner + viewer; owner creates an active poll.
  Commands (matrix):
    - `PUT /polls/{id}` without auth → 401 (R1).
    - `PUT /polls/{id}` owner, `{ "title": "new" }` → 200, title updated (R2).
    - `PUT /polls/{id}` owner, `{}` → 200, unchanged (R3).
    - `PUT /polls/{id}` owner, `{ "anonymous": true, "anonymous_set": true }` → 200 (R4).
    - `PUT /polls/{id}` owner, title of 256 chars → 400 (R5).
    - `PUT /polls/{id}` viewer JWT → 404 (R6).
    - `POST /polls/{id}/close` owner → 200, `status: "closed"` (R14).
    - `POST /polls/{id}/close` owner again → 200, still closed (R15).
    - `PUT /polls/{id}` owner after close → 409 `poll_closed` (R7, per T1: Conflict exists in micro v3).
    - `POST /polls/{id}/close` viewer JWT → 404 (R16).
    - `DELETE /polls/{id}` viewer JWT → 404 (R11).
    - `DELETE /polls/{id}` owner → 200, empty body (R9).
    - `DELETE /polls/{id}` owner again → 404 (R12).
    - `GET /polls/{id}` after delete → 404.
  Expected: every line matches `requirements.md`.

- [x] **T11 — Full test run + archive.**
  - `go vet ./...`
  - `make test-all` — green with `-race -count=1`.
  - Update `TODO.md` / `TODO.ru.md`: mark Day 27 done.
  - Move spec to `specs/_archive/0027-poll-mutations/`.

## Traceability

- **R1** → T9, T10
- **R2** → T5, T6, T9, T10
- **R3** → T5 (empty-string check), T6, T10
- **R4** → T5 (AnonymousSet), T6, T10
- **R5** → T5 (validateTextLen), T6, T10
- **R6** → T5 (ownership), T6, T9, T10
- **R7** → T5 (status check), T6, T9, T10
- **R8** → T9, T10
- **R9** → T3, T5, T7, T9, T10
- **R10** → T5 (no status check in Delete), T7, T10
- **R11** → T5 (ownership in repo), T7, T10
- **R12** → T3 (0 rows → ErrNotFound), T7, T10
- **R13** → T9, T10
- **R14** → T3, T5, T8, T10
- **R15** → T3 (plain UPDATE), T4, T8, T10
- **R16** → T3, T5, T8, T10
- **R17** → T5 (`Close` returns full poll), T8, T10

## Verification

- **Automated:** `make test-all` green.
- **Manual:** curl matrix in T10.
- **DB:** after T10, `SELECT COUNT(*) FROM polls` for the test owner → 0 rows
  (all test polls deleted). After close-only tests, `status = 'closed'`.

## Rollback

- Revert commits T1–T9. No migrations.
- `UpdatePoll`, `DeletePoll`, `ClosePoll` revert to `not_implemented`.
- `PollRepository.Close` removal: nothing else uses it.
- `mustUserID` refactor (T2) is standalone — reverting it alone is safe,
  though other code that depends on it (handlers from T9) must be reverted first.

## Notes / T1: Conflict (409) — microerr.Conflict exists in micro v3
