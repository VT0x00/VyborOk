# Tasks: ListPolls

**Spec ID:** 0026
**Depends on:** `requirements.md` (approved), `design.md` (approved)

## Checklist

- [x] **T1 — Add `QuestionCount` field to `models.Poll`.**
  File: `internal/models/poll.go`.
  Add `QuestionCount int` with a doc comment: "Populated only by list
  methods. Use len(Questions) after GetByID."
  Expected: `go build ./...` green; existing tests unaffected (nothing
  reads the field yet).

- [x] **T2 — Extend `PollRepository.ListByUser` with question count.**
  File: `internal/repository/poll_repository.go`.
  Replace SQL with the correlated-subquery version from `design.md`.
  Add `&p.QuestionCount` to the existing `rows.Scan(...)`.
  Signature unchanged.
  Expected: `go build ./...` green.

- [x] **T3 — Repository test for `QuestionCount`.**
  File: `internal/repository/poll_repository_test.go`.
  Add `TestPollRepository_ListByUser_QuestionCount`:
    - create user; create 2 polls: one with 3 questions, one with 1 question.
    - call `ListByUser`.
    - assert the poll with 3 questions has `QuestionCount == 3`,
      the other has `QuestionCount == 1`.
    - assert `Questions` is empty (len == 0) on both.
  Expected: `make test-all` green.

- [x] **T4 — Add `ListByUser` to `internal/poll.Service`.**
  File: `internal/poll/service.go`.
  - Add consts `defaultListLimit = 20`, `maxListLimit = 100`.
  - Implement `ListByUser(ctx, userID, limit, offset)`:
    - `userID == uuid.Nil` → `ErrInvalidInput`.
    - normalize limit/offset per design table.
    - call `s.polls.ListByUser`, wrap error.
    - return the slice.
  Expected: `go build ./...` green.

- [x] **T5 — Service tests for `ListByUser`.**
  File: `internal/poll/service_test.go`.
  Add a helper `createN(t, svc, userID, n)` that creates n polls with
  distinct titles and sleeps 2 ms between each (for deterministic
  `created_at DESC` ordering).
  Test cases:
    - R2: user with 0 polls → empty slice, not nil.
    - R3: user with 5 polls, limit=20, offset=0 → 5 items, newest first.
    - R4: limit=2, offset=0 → 2 items, newest two.
    - R5: limit=2, offset=2 → 2 items, oldest two.
    - R6: limit=500 → clamped, still returns all (up to 100).
    - R7: limit=0 → uses default 20.
    - R7: limit=-5 → uses default 20.
    - R8: offset=-1 → treated as 0.
    - R9: user A has 3 polls, user B has 2 → A sees only 3, B sees only 2.
    - R10: poll with 3 questions → `QuestionCount == 3`.
    - R11: poll with 0 questions — skip (cannot be created via service);
          covered by repository test if needed.
  Expected: `make test-all` green.

- [x] **T6 — Implement handler `ListPolls`.**
  File: `http/handler/poll.go`.
  - Replace the `not_implemented` stub.
  - Require `auth.UserIDFromContext` (R1).
  - Call `h.poll.ListByUser`.
  - Build `rsp.Polls` as a non-nil slice (for R2's `[]`).
  - Add `toPollCard` helper in the same file.
  Expected: `go build ./...` green.

- [x] **T7 — Manual verification via curl.**
  Setup: register a user, create 5 polls with distinct titles, wait a
  second between creations (or use `psql` to set `created_at`). Then:
    - `GET /polls` without auth → 401 (R1).
    - `GET /polls` with auth → 5 items, newest first (R3).
    - `GET /polls?limit=2&offset=0` → 2 items (R4).
    - `GET /polls?limit=2&offset=2` → 2 items (R5).
    - `GET /polls?limit=2&offset=4` → 1 item.
    - `GET /polls?limit=500` → 5 items (clamped, but only 5 exist).
    - `GET /polls?limit=-5` → 5 items.
    - `GET /polls?offset=-1` → 5 items.
    - Second user with 0 polls → `{ "polls": [] }` (R2).
    - Check `question_count` on a poll created with 3 questions → 3 (R10).
  Expected: every line matches `requirements.md`.

- [x] **T8 — Full test run + archive.**
  - `go vet ./...`
  - `make test-all` — green with `-race -count=1`.
  - Update `TODO.md` / `TODO.ru.md`: mark Day 26 done.
  - Move spec to `specs/_archive/0026-list-polls/`.

## Traceability

- **R1** → T6, T7
- **R2** → T5, T6, T7
- **R3** → T2 (query), T4 (service), T5, T6, T7
- **R4** → T4 (normalize + offset), T5, T7
- **R5** → T4, T5, T7
- **R6** → T4 (clamp), T5, T7
- **R7** → T4 (default), T5, T7
- **R8** → T4 (offset normalization), T5, T7
- **R9** → T2 (`WHERE user_id = $1`), T5, T7
- **R10** → T2 (subquery), T3, T5, T7
- **R11** → T2 (LEFT-less subquery returns 0), T3

## Verification

- **Automated:** `make test-all` green.
- **Manual:** curl matrix in T7.
- **DB:** `EXPLAIN ANALYZE` on the list query with 20+ polls — confirm the
  index `idx_polls_user_id_created` is used (expect `Index Scan`), and
  the correlated subquery runs against `questions_poll_id_position_key`.

## Rollback

- Revert commits T1–T6. No migrations.
- `ListPolls` reverts to `not_implemented`. No other RPC depends on it.
- `QuestionCount` removal: nothing reads it after revert.
