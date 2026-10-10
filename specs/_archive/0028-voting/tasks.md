# Tasks: Voting

**Spec ID:** 0028
**Depends on:** `requirements.md` (approved), `design.md` (approved)

## Checklist

- [ ] **T1 — Migration `000003_votes`.**
  Files: `backend/db/migrations/000003_votes.up.sql`,
  `backend/db/migrations/000003_votes.down.sql`.
  `votes` table per design (JSONB `answers`, `UNIQUE (poll_id, user_id)`,
  indexes on `poll_id` and `user_id`). Down drops the table.
  Expected: `make test-db-reset` applies cleanly.

- [ ] **T2 — Model `models.Vote` + `models.Answer`.**
  File: `backend/internal/models/vote.go`.
  `Vote{ID, PollID, UserID *uuid.UUID, Answers []Answer, CreatedAt}` and
  `Answer{QuestionID, OptionIDs []uuid.UUID, Value *int, Text string}`.
  Expected: `go build ./...` green.

- [ ] **T3 — `repository.ErrAlreadyVoted` + `VoteRepository`.**
  File: `backend/internal/repository/vote_repository.go` (sentinel next to
  `ErrNotFound` in `user_repository.go`).
  `Create`, `HasVoted`, `ListByPoll` per design. JSON marshal/unmarshal of
  `answers`. Unique violation (lib/pq code 23505) → `ErrAlreadyVoted`.
  Expected: `go build ./...` green.

- [ ] **T4 — Repository tests.**
  File: `backend/internal/repository/vote_repository_test.go`.
  - `TestVoteRepository_CreateAndList`: create a vote, `ListByPoll` returns it
    with answers round-tripped.
  - `TestVoteRepository_Create_Anonymous`: `UserID = nil` stored as NULL.
  - `TestVoteRepository_Create_Duplicate`: same (poll, user) twice →
    `ErrAlreadyVoted`.
  - `TestVoteRepository_HasVoted`: false before, true after.
  - `TestVoteRepository_ListByPoll_Ordering`: oldest first.
  Expected: `make test-all` green.

- [ ] **T5 — Vote domain errors.**
  File: `backend/internal/vote/errors.go`.
  `ErrInvalidInput`, `ErrNotFound`, `ErrAuthRequired`, `ErrPollClosed`,
  `ErrAlreadyVoted`.
  Expected: `go build ./...` green.

- [ ] **T6 — `vote.Service` (`Submit`, `HasVoted`).**
  File: `backend/internal/vote/service.go`.
  Per design: load poll, validate all answers, repeat check, insert.
  Expected: `go build ./...` green.

- [ ] **T7 — Service tests (validation matrix).**
  File: `backend/internal/vote/service_test.go`.
  Cover R5–R23 and R26–R29:
  - `TestService_Submit_SingleSuccess` (R5), `_AnonymousNoJWT` (R6),
    `_AnonymousWithJWT` (R7).
  - `TestService_Submit_Repeat` (R8), `_AnonymousRepeat_NoJWT` (R9a),
    `_AnonymousRepeat_SameJWT` (R9b).
  - `TestService_Submit_Single_ZeroOptions` (R10), `_Single_TwoOptions` (R11),
    `_OptionNotInQuestion` (R12), `_Multiple_ZeroOptions` (R13),
    `_Multiple_DuplicateOption` (R14).
  - `TestService_Submit_Scale_OutOfRange` (R15), `_Scale_InRange` (R16).
  - `TestService_Submit_Text_TooLong` (R17), `_Text_Empty` (R18),
    `_Text_OK` (R19).
  - `TestService_Submit_MissingQuestion` (R20), `_DuplicateQuestion` (R21),
    `_UnknownQuestion` (R22), `_WrongPayload` (R23).
  - `TestService_Submit_ClosedPoll` (R3), `_NonAnonymousNoJWT` (R4).
  - `TestService_HasVoted_NoJWT` (R26), `_NotVoted` (R27), `_Voted` (R28),
    `_AnonymousVote` (R29).
  Expected: `make test-all` green.


- [ ] **T8 — Proto + codegen + stub handlers.**
  Files: `backend/http/proto/main.proto`, `backend/http/handler/vote.go`.
  Add `SubmitVote`, `HasVoted` RPCs and the `SubmitVoteReq`, `Answer`,
  `SubmitVoteRsp`, `Vote`, `HasVotedReq`, `HasVotedRsp` messages. Run
  `make gen`. Add **stub** `SubmitVote`/`HasVoted` methods on `VyborokHandler`
  returning `microerr.NewError(501, "not_implemented")` so the compile-time
  check passes.
  Verify Q2: the micro v3 router accepts `POST` and `GET` on the same path
  `/polls/{id}/vote` (start the server, curl both). If not, switch `HasVoted`
  to `GET /polls/{id}/has-voted` and record the outcome in Notes.
  Expected: `go build ./...` green; `*.pb.go` regenerated (not hand-edited).

- [ ] **T9 — Real handlers + wiring.**
  Files: `backend/http/handler/vote.go`, `backend/http/handler/handler.go`,
  `backend/main.go`.
  Replace the stubs with the thin handlers per design; add `mapVoteError` and
  `toVote`; add `vote *vote.Service` to `VyborokHandler` and a `voteSvc`
  parameter to `handler.New`; construct `repository.NewVoteRepository(db)` and
  `vote.NewService(voteRepo, pollRepo)` in `main.go` and pass them in.
  (Touches three files because the DI wiring is coupled.)
  Expected: `go build ./...` green; `make run` starts.

- [ ] **T10 — Manual verification via curl.**
  Setup: register two users (owner, voter); owner creates (a) a non-anonymous
  poll with one `single` + one `scale` + one `text` question, and (b) an
  anonymous poll with one `single` question.
  Commands (matrix):
    - `POST /polls/{id}/vote` bad UUID → 400 (R1).
    - `POST /polls/{missing}/vote` → 404 (R2).
    - `POST /polls/{closed}/vote` → 409 poll_closed (R3).
    - `POST /polls/{nonanon}/vote` no JWT → 401 (R4).
    - `POST /polls/{nonanon}/vote` voter JWT, valid answers → 200 (R5).
    - `POST /polls/{nonanon}/vote` voter JWT again → 409 already_voted (R8).
    - `POST /polls/{anon}/vote` no JWT, valid → 200 (R6).
    - `POST /polls/{anon}/vote` no JWT again → 200 (R9).
    - `POST /polls/{anon}/vote` voter JWT, valid → 200, `user_id` set (R7).
    - `POST /polls/{anon}/vote` voter JWT again → 409 already_voted (R9b).
    - single with 0 / 2 options → 400 (R10, R11).
    - scale out of range → 400 (R15); in range → 200 (R16).
    - text too long / empty → 400 (R17, R18).
    - missing a question → 400 (R20).
    - `GET /polls/{id}/vote` no JWT → `{has_voted:false}` (R26).
    - `GET /polls/{nonanon}/vote` voter JWT (after voting) → `{has_voted:true}` (R28).
    - `GET /polls/{anon}/vote` voter JWT → `{has_voted:false}` (R29).
  Expected: every line matches `requirements.md`.

- [ ] **T11 — Full test run + bookkeeping.**
  - `go vet ./...`
  - `make test-all` — green with `-race -count=1`.
  - Update `docs/TODO.md`: mark Days 29–35 done.
  - Move spec to `specs/_archive/0028-voting/`.

## Traceability

- R1 → T9, T10
- R2 → T6, T7, T10
- R3 → T6, T7, T10
- R4 → T6, T7, T10
- R5 → T3, T6, T7, T9, T10
- R6 → T3, T6, T7, T9, T10
- R7 → T3, T6, T7, T9, T10
- R8 → T3 (unique), T6, T7, T10
- R9 → T3 (NULL unique), T6, T7, T10
- R10 → T6, T7, T10
- R11 → T6, T7, T10
- R12 → T6, T7, T10
- R13 → T6, T7, T10
- R14 → T6, T7, T10
- R15 → T6, T7, T10
- R16 → T6, T7, T10
- R17 → T6, T7, T10
- R18 → T6, T7, T10
- R19 → T6, T7, T10
- R20 → T6, T7, T10
- R21 → T6, T7, T10
- R22 → T6, T7, T10
- R23 → T6, T7, T10
- R24 → T9, T10
- R25 → T6, T7, T10
- R26 → T6, T7, T9, T10
- R27 → T6, T7, T9, T10
- R28 → T3, T6, T7, T9, T10
- R29 → T3, T6, T7, T9, T10

## Verification

- **Automated:** `make test-all` green.
- **Manual:** curl matrix in T10.
- **DB:** after T10, `SELECT poll_id, user_id, answers FROM votes ORDER BY
  created_at` shows the expected rows (non-anonymous votes have `user_id`,
  anonymous votes have NULL).

## Rollback

- Revert commits T1–T9.
- `make migrate-down` (or drop `votes`) removes the table.
- `SubmitVote` / `HasVoted` are removed from `main.proto` + `make gen` (they do
  not exist before this spec).

## Notes

- **T8 (Q2 path):** the micro v3 HTTP router accepts `POST` and `GET` on the
  same path `/polls/{id}/vote` — no fallback to `/has-voted` needed. Verified
  in T10 (both methods routed correctly).
- **T10 (codec):** `HasVotedRsp` with `has_voted = false` was serialized as
  `{}` (proto3 `omitempty` drops the zero value). Extended the response codec
  in `main.go` (`emptyPollsCodec` → `fixedCodec`) to emit
  `{"has_voted":false}` explicitly, same pattern as spec 0026 R2.
- **T10 (poll service quirk):** a `single` question requires at least 2
  options (poll service validation), which shaped the manual test polls.
- **T10 result:** 36/36 checks passed (R1–R29, incl. R9a/R9b, R23a/R23b).


