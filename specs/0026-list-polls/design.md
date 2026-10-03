# Design: ListPolls

**Spec ID:** 0026
**Depends on:** `requirements.md`

## Data Model

### Add `QuestionCount` to `models.Poll`

```go
type Poll struct {
    ID            uuid.UUID
    UserID        uuid.UUID
    Title         string
    Description   string
    Status        PollStatus
    Anonymous     bool
    CreatedAt     time.Time
    UpdatedAt     time.Time
    QuestionCount int        // NEW — populated only by ListByUser
    Questions     []Question
}
```

**Why a scalar field on the model, not a separate `PollCard` type in the
repository.** A separate repository type would duplicate 7 fields and
require a `toPollCard` mapper. The scalar is documented as "populated only
by list methods", is 0 in all other contexts, and the handler uses it
directly. If it ever gets out of sync, it's a single field to audit.

**Not populated by `GetByID`.** `GetByID` loads `Questions` fully — a client
using that path computes the count as `len(Questions)`. Populating
`QuestionCount` there would create two sources of truth.

No migration. No schema change. Index `idx_polls_user_id_created` from
`000002_polls` already covers the query.

## API

No proto changes. `ListPolls` RPC, `ListPollsReq`, `ListPollsRsp`, and
`PollCard` already exist (added Day 24).

```
GET /polls?limit=20&offset=0
  Authorization: Bearer <access>
→ 200 ListPollsRsp { polls: [PollCard, ...] }
→ 401 unauthorized (no JWT / invalid JWT)
```

Query parameters:

| Param    | Default | Min | Max | Behavior on out-of-range |
|----------|---------|-----|-----|--------------------------|
| `limit`  | 20      | 1   | 100 | `<= 0` → 20; `> 100` → 100 |
| `offset` | 0       | 0   | ∞   | `< 0` → 0                |

Normalization happens in **service**, not handler. Repository trusts its
arguments. This keeps the repository dumb and makes normalization testable
without HTTP.

## Service Layer

New method on `internal/poll.Service`:

```go
const (
    defaultListLimit = 20
    maxListLimit     = 100
)

// ListByUser returns the user's polls, newest first, paginated.
// Normalizes limit/offset per the design table above.
// Returned polls have QuestionCount populated and Questions empty.
func (s *Service) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Poll, error)
```

Validation:
- `userID == uuid.Nil` → `ErrInvalidInput` (defensive; middleware should
  have rejected this earlier).

Normalization:
```go
if limit <= 0  { limit = defaultListLimit }
if limit > maxListLimit { limit = maxListLimit }
if offset < 0  { offset = 0 }
```

No transaction needed — single query.

## Repository Layer

### Modify `PollRepository.ListByUser`

Current signature (Day 22):
```go
func (r *PollRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Poll, error)
```

Signature unchanged. Only the SQL and scan change.

SQL sketch:
```sql
SELECT p.id, p.user_id, p.title, p.description, p.status, p.anonymous,
       p.created_at, p.updated_at,
       (SELECT COUNT(*) FROM questions q WHERE q.poll_id = p.id) AS question_count
FROM polls p
WHERE p.user_id = $1
ORDER BY p.created_at DESC
LIMIT $2 OFFSET $3
```

**Why a correlated subquery, not `LEFT JOIN ... GROUP BY`:**
- For 20 rows (typical page size), the subquery runs 20 times against the
  `questions_poll_id_position_key` unique index — each lookup is O(log n).
- `LEFT JOIN ... GROUP BY` would produce the same result but requires a
  `COUNT(q.id)` in the SELECT and a `GROUP BY` over all `p.*` columns,
  which is more error-prone to maintain.
- If profiling later shows a problem, switching to a `LEFT JOIN LATERAL`
  is a one-line change isolated to this method.

Scan: `&p.QuestionCount` added at the end of the existing `rows.Scan(...)`.
All other fields unchanged. `p.Questions` remains empty (`nil`).

No new index. `idx_polls_user_id_created` covers `(user_id, created_at DESC)`.

## Handler Layer

Replace the `not_implemented` stub for `ListPolls` in `http/handler/poll.go`.

```go
func (h *VyborokHandler) ListPolls(ctx context.Context, req *pb.ListPollsReq, rsp *pb.ListPollsRsp) error {
    userID, ok := auth.UserIDFromContext(ctx)
    if !ok {
        return microerr.Unauthorized("unauthorized", "auth required")
    }

    polls, err := h.poll.ListByUser(ctx, userID, int(req.Limit), int(req.Offset))
    if err != nil {
        h.logger.Error("list polls failed", "err", err, "user_id", userID)
        return mapPollError(err)
    }

    rsp.Polls = make([]*pb.PollCard, 0, len(polls))
    for i := range polls {
        rsp.Polls = append(rsp.Polls, toPollCard(&polls[i]))
    }
    return nil
}
```

New helper `toPollCard(p *models.Poll) *pb.PollCard` — mapping lives in
`poll.go` next to `toPoll`.

```go
func toPollCard(p *models.Poll) *pb.PollCard {
    if p == nil {
        return nil
    }
    return &pb.PollCard{
        Id:            p.ID.String(),
        Title:         p.Title,
        Description:   p.Description,
        Status:        string(p.Status),
        Anonymous:     p.Anonymous,
        QuestionCount: int32(p.QuestionCount),
        CreatedAt:     p.CreatedAt.Format(time.RFC3339),
    }
}
```

**Empty list handling.** `rsp.Polls` is initialized as an empty non-nil
slice, so JSON serialization returns `[]` (R2), not `null`.

## Alternatives Considered

- **Option A:** Return full `Poll` with nested questions (like `GetPoll`).
  Rejected: for 20 polls × 10 questions each = 200 nested objects, all
  fetched, all serialized. Wasteful for a list view.

- **Option B:** Separate `PollCard` type in `models` (not just a field).
  Rejected: duplicates 7 fields, needs a mapper in repository, harder to
  keep in sync if `Poll` grows. The scalar `QuestionCount` is enough.

- **Option C:** Add `total` to `ListPollsRsp`.
  Rejected for MVP: extra COUNT query on every request. Revisit in Phase 8.

- **Option D (chosen):** Reuse `models.Poll`, populate `QuestionCount` in
  list query, map to `PollCard` in handler. No new types, one extra SQL
  subquery per row.

## Open Questions
None. All resolved from `requirements.md`.
