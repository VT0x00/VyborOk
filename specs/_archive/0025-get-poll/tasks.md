# Tasks: GetPoll

**Spec ID:** 0025
**Depends on:** `requirements.md` (approved), `design.md` (approved)

## Checklist

Each task = one small edit, one commit, one checkbox. Order matters:
later tasks assume earlier ones are done.

- [x] **T1 — Refactor `poll.NewService` constructor.**
  Add `users *repository.UserRepository` as the second parameter.
  File: `internal/poll/service.go` (struct + constructor only).
  Update callers in the same commit:
    - `main.go` — pass `userRepo` to `poll.NewService`.
    - `internal/poll/service_test.go` — `newTestService` must construct
      both `PollRepository` and `UserRepository`.
  Expected: `go build ./...` green, `make test-all` green (nothing broken).

- [x] **T2 — Implement `Service.GetByID`.**
  File: `internal/poll/service.go`.
  Signature: `GetByID(ctx, id, viewerID uuid.UUID) (*models.Poll, error)`.
  Logic exactly as in `design.md`:
    - `id == uuid.Nil` → `ErrInvalidInput`.
    - `polls.GetByID` → propagate `ErrNotFound`.
    - owner check → return full poll.
    - load owner via `users.GetByID`.
    - `active` → return full poll.
    - `closed` + owner private → `ErrNotFound`.
    - `closed` + owner public → `stripToStructure(poll)`.
  Add helper `stripToStructure(p *models.Poll) *models.Poll` in the same
  file. For MVP it returns a shallow copy (questions/options preserved),
  but is the single place Phase 4 will redact stats from.
  Expected: `go build ./...` green.

- [x] **T3 — Service tests for `GetByID`.**
  File: `internal/poll/service_test.go`.
  Add helper `newTestService` already returns the service; add
  `setPrivate(t, db, userID)` helper that flips `users.is_private`.
  Test cases:
    - R1/R2: owner fetches active poll → full data, 4 questions, options present.
    - R2: owner fetches closed poll → full data (call `Close` once it exists
      — for now, flip status via direct `UPDATE` in the test).
    - R3: anonymous fetches active poll → full data.
    - R3: other authenticated user fetches active poll → full data.
    - R4: anonymous fetches closed poll with public owner → full structure
      (questions and options present), no error.
    - R6: `GetByID` with random UUID → `ErrNotFound`.
    - R7: anonymous fetches closed poll with private owner → `ErrNotFound`.
    - R7: other authenticated user fetches same → `ErrNotFound`.
    - R1: `GetByID(ctx, uuid.Nil, ...)` → `ErrInvalidInput`.
  Expected: `make test-all` green.

- [x] **T4 — Implement handler `GetPoll`.**
  File: `http/handler/poll.go`.
  Replace the `not_implemented` stub:
    - Parse `req.Id` with `uuid.Parse`. Parse error → `400 invalid_input`.
    - `viewerID` from `auth.UserIDFromContext(ctx)` if present, else `uuid.Nil`.
    - Call `h.poll.GetByID(ctx, id, viewerID)`.
    - On error → `mapPollError(err)`.
    - On success → `rsp.Poll = toPoll(poll)`.
  Expected: `go build ./...` green.

- [x] **T5 — Manual verification via curl.**
  Register two users (owner, viewer). Owner creates a poll (Day 24 flow).
  Commands:
    - `GET /polls/{id}` without auth → 200 full poll (R3).
    - `GET /polls/{id}` with viewer JWT → 200 full poll (R3).
    - `GET /polls/{id}` with owner JWT → 200 full poll (R2).
    - Flip poll status to `closed` via `psql`.
    - `GET /polls/{id}` without auth → 200 stripped poll (R4).
    - Flip owner `is_private = true` via `psql`.
    - `GET /polls/{id}` without auth → 404 not_found (R7).
    - `GET /polls/{id}` with owner JWT → 200 full poll (R2, R7 exception).
    - `GET /polls/not-a-uuid` → 400 invalid_input (R5).
    - `GET /polls/{random-uuid}` → 404 not_found (R6).
  Expected: every line matches `requirements.md`.

- [x] **T6 — Full test run + cleanup.**
  - `go vet ./...`
  - `make test-all` — all green with `-race -count=1`.
  - Remove any debug logs added during T5.
  - Move spec to `specs/_archive/0025-get-poll/` after merge.
  Expected: green CI-ready state.

## Traceability

- **R1** → T2, T4
- **R2** → T2 (owner branch), T3 (owner test), T5 (owner curl)
- **R3** → T2 (active branch), T3, T5
- **R4** → T2 (`stripToStructure`), T3, T5
- **R5** → T4 (uuid.Parse), T5
- **R6** → T2 (`ErrNotFound` propagation), T3, T5
- **R7** → T2 (private + closed), T3, T5
- **R8** → T2 (public + closed), T3, T5

## Verification

- **Automated:** `make test-all` green.
- **Manual:** curl matrix in T5.
- **DB:** `docker exec -it vyborok-postgres psql -U postgres -d vyborok -c '\d polls'`
  — no schema change expected (T1–T5 touch no migrations).

## Rollback

- Revert the T1–T5 commits (squash-merge reverts cleanly).
- No migrations to roll back.
- `Service.GetByID` is new API — nothing else depends on it yet.
- `NewService` signature change is reverted together with `main.go`
  and `service_test.go` in T1's commit.
