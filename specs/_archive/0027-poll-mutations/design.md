# Design: Poll Mutations

**Spec ID:** 0027
**Depends on:** `requirements.md`

## Data Model
No changes. All three operations use existing tables and columns.
No migrations.

## API

No changes to `main.proto` — RPCs and messages already exist (Day 24):

```
PUT    /polls/{id}        → UpdatePollReq  → PollRsp
DELETE /polls/{id}        → DeletePollReq  → DeletePollRsp
POST   /polls/{id}/close  → ClosePollReq   → PollRsp
```

`{id}` comes from the URL path and populates `req.Id` (micro v3 behavior;
verified already by `GetPoll` in Day 25).

**UpdatePollReq partial-update semantics** — same pattern as `UpdateProfile`
in auth (consistency):

| Field           | Empty value behavior                   |
|-----------------|----------------------------------------|
| `title`         | `""` → leave unchanged                 |
| `description`   | `""` → leave unchanged                 |
| `anonymous` + `anonymous_set` | `_set: false` → leave unchanged |

**Known limitation** (documented, not fixed): a client cannot clear
`description` back to empty via this API. Consistent with `auth.UpdateProfile`,
where `bio` also cannot be cleared. Deferred — a `_set` pair for every string
is future work.

## Service Layer

Three new methods on `internal/poll.Service`. All take `userID` and enforce
ownership through the repository.

### `Update`

```go
type UpdateInput struct {
    Title        string
    Description  string
    Anonymous    bool
    AnonymousSet bool
}

// Update modifies title/description/anonymous on an active poll owned by userID.
// Questions and options are immutable. Only the owner can update.
func (s *Service) Update(ctx context.Context, id, userID uuid.UUID, in UpdateInput) (*models.Poll, error)
```

Algorithm:
```
poll := polls.GetByID(ctx, id)              // loads questions
if poll.UserID != userID  → ErrNotFound     // do not leak existence
if poll.Status == closed  → ErrPollClosed

if in.Title != "":
    validate length 1..maxTitleLen
    poll.Title = strings.TrimSpace(in.Title)

if in.Description != "":
    validate length 0..maxDescriptionLen
    poll.Description = strings.TrimSpace(in.Description)

if in.AnonymousSet:
    poll.Anonymous = in.Anonymous

if err := polls.Update(ctx, poll); err != nil:
    return nil, err

return poll, nil
```

The returned `*models.Poll` already has questions loaded (from the initial
`GetByID`), so we do not re-query.

The repository's `Update` still includes `WHERE id = $1 AND user_id = $2` as
a defensive check — a second line of defense against bugs in the service.

### `Delete`

```go
// Delete removes a poll owned by userID. Cascades to questions and options.
// Works on polls in any status.
func (s *Service) Delete(ctx context.Context, id, userID uuid.UUID) error
```

Algorithm:
```
if err := polls.Delete(ctx, id, userID); err != nil:
    return err   // ErrNotFound
return nil
```

No `GetByID` first — `PollRepository.Delete` already filters by ownership in
SQL (`WHERE id = $1 AND user_id = $2`) and returns `ErrNotFound` on 0 rows.

### `Close`

```go
// Close transitions an active poll to closed. Idempotent: calling it on an
// already-closed poll is a successful no-op. Only the owner can close.
func (s *Service) Close(ctx context.Context, id, userID uuid.UUID) (*models.Poll, error)
```

Algorithm:
```
if err := polls.Close(ctx, id, userID); err != nil:
    return nil, err   // ErrNotFound

poll := polls.GetByID(ctx, id)
if err != nil:
    return nil, err

return poll, nil
```

Two queries (`Close` + `GetByID`), not a transaction. In a monolith with a
single owner per poll, no race matters.

Alternative (rejected): `Close` in the repository returns `*models.Poll`
directly via `UPDATE ... RETURNING` plus the two questions/options queries.
That would embed `loadQuestions` logic into `Close`, duplicating Day 22 code.
Two queries from the service is simpler.

## Repository Layer

### New method: `PollRepository.Close`

```go
// Close sets status='closed' on a poll owned by userID. Idempotent:
// calling it on an already-closed poll is a successful no-op.
func (r *PollRepository) Close(ctx context.Context, id, userID uuid.UUID) error
```

SQL:
```sql
UPDATE polls
SET status = 'closed', updated_at = NOW()
WHERE id = $1 AND user_id = $2
```

No `RETURNING` — service loads the full poll afterward. On 0 rows affected →
`ErrNotFound` (matches `Delete`).

The `status = 'closed'` UPDATE runs even if the poll is already closed — a
no-op at the row level, but `updated_at` is refreshed (see R2 of requirements).

### Reused methods
- `PollRepository.GetByID` — used by `Update` and `Close` (Day 22).
- `PollRepository.Update` — used by `Update` (Day 22). Already filters by
  `user_id`.
- `PollRepository.Delete` — used by `Delete` (Day 22). Already filters by
  `user_id`.

No new indexes. All queries filter by `id` (PK) and `user_id`.

## Handler Layer

Replace three `not_implemented` stubs in `http/handler/poll.go`.

### New helper in `handler.go`

```go
// mustUserID extracts the authenticated user id or returns 401.
// Used by every protected RPC to avoid repeating the two-line dance.
func mustUserID(ctx context.Context) (uuid.UUID, error) {
    userID, ok := auth.UserIDFromContext(ctx)
    if !ok {
        return uuid.Nil, microerr.Unauthorized("unauthorized", "auth required")
    }
    return userID, nil
}
```

`GetMe`, `UpdateProfile`, `CreatePoll` — all currently inline the same code.
Refactoring them to use `mustUserID` is in scope for this spec (small
consistency win).

### `UpdatePoll`

```go
func (h *VyborokHandler) UpdatePoll(ctx, req, rsp) error {
    userID, err := mustUserID(ctx)
    if err != nil { return err }

    id, err := uuid.Parse(req.Id)
    if err != nil {
        return microerr.BadRequest("invalid_input", "invalid poll id")
    }

    p, err := h.poll.Update(ctx, id, userID, poll.UpdateInput{
        Title:        req.Title,
        Description:  req.Description,
        Anonymous:    req.Anonymous,
        AnonymousSet: req.AnonymousSet,
    })
    if err != nil {
        h.logger.Error("update poll failed", "err", err, "user_id", userID, "poll_id", id)
        return mapPollError(err)
    }

    rsp.Poll = toPoll(p)
    return nil
}
```

### `DeletePoll`

```go
func (h *VyborokHandler) DeletePoll(ctx, req, rsp) error {
    userID, err := mustUserID(ctx)
    if err != nil { return err }

    id, err := uuid.Parse(req.Id)
    if err != nil {
        return microerr.BadRequest("invalid_input", "invalid poll id")
    }

    if err := h.poll.Delete(ctx, id, userID); err != nil {
        h.logger.Error("delete poll failed", "err", err, "user_id", userID, "poll_id", id)
        return mapPollError(err)
    }
    return nil
}
```

### `ClosePoll`

```go
func (h *VyborokHandler) ClosePoll(ctx, req, rsp) error {
    userID, err := mustUserID(ctx)
    if err != nil { return err }

    id, err := uuid.Parse(req.Id)
    if err != nil {
        return microerr.BadRequest("invalid_input", "invalid poll id")
    }

    p, err := h.poll.Close(ctx, id, userID)
    if err != nil {
        h.logger.Error("close poll failed", "err", err, "user_id", userID, "poll_id", id)
        return mapPollError(err)
    }

    rsp.Poll = toPoll(p)
    return nil
}
```

### `mapPollError` — no change for R7

The existing `ErrPollClosed → 400 poll_closed` mapping stays. We do not
introduce `ErrForbidden` for R6/R11/R16 — those go through `ErrNotFound`
(same as `GetPoll` R7), by design (constitution §7).

If R1 of requirements decides on 409 instead of 400, only `mapPollError`
changes — one line.

## Alternatives Considered

- **Option A: separate specs for Update, Delete, Close.**
  Rejected: three specs × three files = nine documents for what is one
  cohesive mutation layer. Shared decisions (ownership, status checks,
  partial-update semantics) would be duplicated.

- **Option B: `PATCH /polls/{id}` for status changes instead of `POST /close`.**
  Rejected: PATCH support in micro v3 is not confirmed; `POST /close` is
  unambiguous and matches REST conventions for actions (like GitHub's
  `POST /repos/{owner}/{repo}/issues/{n}/lock`).

- **Option C: soft delete (`deleted_at TIMESTAMPTZ`).**
  Rejected for MVP: adds filtering to every query (`WHERE deleted_at IS NULL`),
  complicates indexes, no user-facing benefit yet. Revisit if we ever need
  "restore deleted poll".

- **Option D (chosen): three RPCs, one spec, ownership in SQL, no transactions.**
  Consistent with the rest of the codebase, minimal surface area, clear
  semantics.

## Open Questions
None. All resolved from `requirements.md`.
