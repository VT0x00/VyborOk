# Design: GetPoll

**Spec ID:** 0025
**Depends on:** `requirements.md`

## Data Model
No changes. Uses existing tables `polls`, `questions`, `options` from
migration `000002_polls`.

## API
No proto changes — `GetPoll` RPC and `PollRsp` message already exist
(added Day 24). Only the handler body changes.

```
GET /polls/{id}
→ 200 PollRsp { poll: Poll }
→ 400 invalid_input (bad UUID)
→ 404 not_found (missing or hidden)
→ 401 unauthorized (only if middleware rejects a broken token)
```

## Service Layer

New method on `internal/poll.Service`:

```go
// GetByID returns the poll with all questions and options.
// viewerID == uuid.Nil means anonymous caller.
//
// Visibility rules:
//   - owner: full poll, always.
//   - non-owner / anonymous: full poll if active;
//     stripped poll if closed and owner is public;
//     ErrNotFound if closed and owner is private.
func (s *Service) GetByID(ctx context.Context, id, viewerID uuid.UUID) (*models.Poll, error)
```

Validation:

`id == uuid.Nil` → `ErrInvalidInput` (defensive; the handler already
parses the path UUID and returns 400 on parse failure).

New dependency: `Service` currently holds only `*repository.PollRepository`.
It needs `*repository.UserRepository` to check `owner.IsPrivate` for the
R7 rule. Constructor becomes:

```go
func NewService(polls *repository.PollRepository, users *repository.UserRepository) *Service
```

This is a breaking change for `main.go` and existing tests — both must
be updated in this spec.

Visibility algorithm:

```
poll := polls.GetByID(ctx, id)  →  ErrNotFound if missing
if viewerID == poll.UserID:
    return poll

owner := users.GetByID(ctx, poll.UserID)   → ErrNotFound if owner deleted (cascade would prevent this, but defensive)

if poll.Status == active:
    return poll                              // R3

// closed, non-owner
if owner.IsPrivate:
    return ErrNotFound                       // R7
return stripToStructure(poll), nil           // R4
```

Helper `stripToStructure(p *models.Poll) *models.Poll`:

Returns a shallow copy with questions/options intact (structure is public),
but no future fields (votes, stats) that we'll add in Phase 4.

For MVP, this is currently identical to the poll itself. The helper exists
to make the intent explicit and to give Phase 4 a single place to redact.

# Repository Layer
No new methods. `PollRepository.GetByID` already loads questions and options
in two queries (Day 22). `UserRepository.GetByID` already exists.

Alternatives Considered
* Option A: Return 404 for any closed poll to non-owners.
Rejected: users need to review a closed poll's questions to understand
what they (or others) answered — see R4. Hiding them entirely breaks UX.

* Option B: Return 403 for closed polls to non-owners.
Rejected: 403 leaks existence. We return 404 to match the "don't leak"
pattern established in Phase 1 (`GetProfile` with `ErrNotFound`).

* Option C: Always return full poll regardless of status.
Rejected: violates the "closed poll with private owner shouldn't be
discoverable" security boundary (R7).

* Option D (chosen): Full poll to owner, full to non-owner if active,
structure-only to non-owner if closed and owner is public, 404 otherwise.
Balances UX, security, and complexity.

# Open Questions

