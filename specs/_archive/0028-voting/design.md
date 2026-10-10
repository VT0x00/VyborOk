# Design: Voting

**Spec ID:** 0028
**Depends on:** `requirements.md`

## Data Model

New table `votes`. Migration files: `000003_votes.up.sql`,
`000003_votes.down.sql`.

```sql
CREATE TABLE votes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id    UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    -- NULL for anonymous polls (identity not recorded).
    user_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    answers    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Repeat-vote backstop. Plain UNIQUE (NOT NULLS NOT DISTINCT) so that any
    -- number of NULL user_id rows (anonymous votes) are allowed.
    UNIQUE (poll_id, user_id)
);

CREATE INDEX idx_votes_poll_id ON votes(poll_id);
CREATE INDEX idx_votes_user_id ON votes(user_id);
```

Notes:
- `UNIQUE (poll_id, user_id)` is the repeat-vote backstop for non-anonymous
  polls. In PostgreSQL a plain `UNIQUE` treats NULLs as distinct, so any number
  of anonymous (`user_id IS NULL`) votes are allowed — exactly the desired
  behavior. We deliberately do **not** use `NULLS NOT DISTINCT` (PG15+), which
  would collapse all anonymous votes into one.
- `user_id ... ON DELETE SET NULL`: if a user is deleted, their votes remain
  (counts are preserved) but become anonymous.
- `answers` is JSONB; format below.
- No `updated_at` — a vote is immutable once submitted.

### `answers` JSON format

A JSON array with one element per answered question, in submission order:

```json
[
  { "question_id": "<uuid>", "option_ids": ["<uuid>", "..."] },
  { "question_id": "<uuid>", "value": 5 },
  { "question_id": "<uuid>", "text": "free text answer" }
]
```

- `option_ids` — present for `single` (length 1) and `multiple` (length ≥ 1).
- `value` — present (non-null) for `scale`.
- `text` — present (non-empty) for `text`.
- Omitted fields use `omitempty` so the stored JSON stays compact.

## API

Two new RPCs in `main.proto`:

```
POST /polls/{id}/vote  →  SubmitVoteReq  →  SubmitVoteRsp
GET  /polls/{id}/vote  →  HasVotedReq    →  HasVotedRsp
```

Both use the same path with different HTTP methods (Q2). `{id}` is the poll id
path param, populating `req.Id`.

### Messages

```proto
message SubmitVoteReq {
    string id = 1; // path param: poll id
    repeated Answer answers = 2;
}

message Answer {
    string question_id = 1;
    // type="single" → exactly 1; type="multiple" → 1+.
    repeated string option_ids = 2;
    // type="scale" only. value_set must be true for a scale answer.
    int32 value = 3;
    bool value_set = 4;
    // type="text" only.
    string text = 5;
}

message SubmitVoteRsp {
    Vote vote = 1;
}

message Vote {
    string id = 1;
    string poll_id = 2;
    // empty string for anonymous votes.
    string user_id = 3;
    repeated Answer answers = 4;
    string created_at = 5; // RFC3339
}

message HasVotedReq {
    string id = 1; // path param: poll id
}

message HasVotedRsp {
    bool has_voted = 1;
}
```

`value` + `value_set` follows the `_set` companion convention (constitution §9):
proto3 cannot distinguish "value not sent" from "value = 0", and a scale may
legitimately be 0.

### Error codes

| Domain error      | HTTP | Code            |
|-------------------|------|-----------------|
| `ErrInvalidInput` | 400  | `invalid_input` |
| `ErrNotFound`     | 404  | `not_found`     |
| `ErrAuthRequired` | 401  | `unauthorized`  |
| `ErrPollClosed`   | 409  | `poll_closed`   |
| `ErrAlreadyVoted` | 409  | `already_voted` |


## Service Layer

New domain `internal/vote` (own errors, per constitution §6).

### `internal/vote/errors.go`

```go
var (
    ErrInvalidInput = errors.New("invalid input")
    ErrNotFound     = errors.New("not found")
    ErrAuthRequired = errors.New("auth required")
    ErrPollClosed   = errors.New("poll closed")
    ErrAlreadyVoted = errors.New("already voted")
)
```

### `internal/vote/service.go`

```go
type Service struct {
    votes *repository.VoteRepository
    polls *repository.PollRepository
}

func NewService(votes *repository.VoteRepository, polls *repository.PollRepository) *Service

type AnswerInput struct {
    QuestionID uuid.UUID
    OptionIDs  []uuid.UUID
    Value      int
    ValueSet   bool
    Text       string
}

type SubmitInput struct {
    PollID  uuid.UUID
    UserID  *uuid.UUID // nil when the caller has no JWT
    Answers []AnswerInput
}

// Submit validates the answers against the poll and stores the vote.
func (s *Service) Submit(ctx context.Context, in SubmitInput) (*models.Vote, error)

// HasVoted reports whether userID has voted on the poll.
// A nil userID (anonymous caller) always yields false.
func (s *Service) HasVoted(ctx context.Context, pollID uuid.UUID, userID *uuid.UUID) (bool, error)
```

### `Submit` algorithm

```
poll := polls.GetByID(ctx, in.PollID)        // loads questions + options
if err == repository.ErrNotFound → ErrNotFound
if poll.Status == closed → ErrPollClosed
if !poll.Anonymous && in.UserID == nil → ErrAuthRequired

// Index questions and their valid option ids.
qByID   := map[uuid.UUID]models.Question
validOpts := map[uuid.UUID]map[uuid.UUID]struct{}  // question id → option ids
for q in poll.Questions:
    qByID[q.ID] = q
    validOpts[q.ID] = { o.ID for o in q.Options }

seen := map[uuid.UUID]struct{}
for a in in.Answers:
    q, ok := qByID[a.QuestionID]
    if !ok → ErrInvalidInput ("unknown question")
    if a.QuestionID in seen → ErrInvalidInput ("duplicate question")
    seen[a.QuestionID] = struct{}{}
    switch q.Type:
    case single:
        require len(a.OptionIDs) == 1
        require a.OptionIDs[0] in validOpts[q.ID]
        require !a.ValueSet && a.Text == ""
    case multiple:
        require len(a.OptionIDs) >= 1
        require all a.OptionIDs in validOpts[q.ID] and unique
        require !a.ValueSet && a.Text == ""
    case scale:
        require a.ValueSet
        require *q.ScaleMin <= a.Value <= *q.ScaleMax
        require len(a.OptionIDs) == 0 && a.Text == ""
    case text:
        t := strings.TrimSpace(a.Text)
        require t != ""
        require utf8.RuneCountInString(t) <= *q.TextMaxLength
        require len(a.OptionIDs) == 0 && !a.ValueSet

if len(seen) != len(poll.Questions) → ErrInvalidInput ("missing answers")

// Identity: recorded whenever a JWT is present, on both anonymous and
// non-anonymous polls (hybrid semantics, requirements "Design note").
effectiveUserID := in.UserID

// Repeat check (only when identity is recorded).
if effectiveUserID != nil:
    if votes.HasVoted(ctx, in.PollID, *effectiveUserID) → ErrAlreadyVoted

vote := &models.Vote{PollID: in.PollID, UserID: effectiveUserID,
                     Answers: toModels(in.Answers)}
if err := votes.Create(ctx, vote); err != nil:
    if errors.Is(err, repository.ErrAlreadyVoted) → ErrAlreadyVoted  // race backstop
    return err
return vote
```

Validation order is deterministic: all answers are validated before the repeat
check, so a bad payload always yields `400` even if the user already voted.

### `HasVoted` algorithm

```
if userID == nil → return false, nil
poll := polls.GetByID(ctx, pollID)   // confirm the poll exists
if err == repository.ErrNotFound → ErrNotFound
return votes.HasVoted(ctx, pollID, *userID)
```



## Repository Layer

New `internal/repository/vote_repository.go`.

```go
type VoteRepository struct{ db *sql.DB }
func NewVoteRepository(db *sql.DB) *VoteRepository

// Create inserts a vote. On a (poll_id, user_id) unique violation it returns
// ErrAlreadyVoted (lib/pq error code 23505).
func (r *VoteRepository) Create(ctx context.Context, v *models.Vote) error

// HasVoted reports whether userID has a vote row for the poll.
func (r *VoteRepository) HasVoted(ctx context.Context, pollID, userID uuid.UUID) (bool, error)

// ListByPoll returns every vote for the poll, oldest first (Phase 4 stats).
func (r *VoteRepository) ListByPoll(ctx context.Context, pollID uuid.UUID) ([]models.Vote, error)
```

SQL sketch:
```sql
-- Create
INSERT INTO votes (poll_id, user_id, answers)
VALUES ($1, $2, $3)
RETURNING id, created_at;

-- HasVoted
SELECT 1 FROM votes WHERE poll_id = $1 AND user_id = $2 LIMIT 1;

-- ListByPoll
SELECT id, poll_id, user_id, answers, created_at
FROM votes WHERE poll_id = $1 ORDER BY created_at;
```

- `answers` is passed as `[]byte` (JSON) for the JSONB column; lib/pq handles
  the binding. On read, the JSON is unmarshaled back into `[]models.Answer`.
- `user_id` is bound as `*uuid.UUID` (nil → NULL).
- `Create` detects the unique violation by matching the lib/pq `*pq.Error` code
  `23505` and returns `repository.ErrAlreadyVoted`.

New repository sentinel: `repository.ErrAlreadyVoted` (alongside the existing
`repository.ErrNotFound` declared in `user_repository.go`).

## Handler Layer

New `http/handler/vote.go`.

```go
func (h *VyborokHandler) SubmitVote(ctx, req *pb.SubmitVoteReq, rsp *pb.SubmitVoteRsp) error
func (h *VyborokHandler) HasVoted(ctx, req *pb.HasVotedReq, rsp *pb.HasVotedRsp) error
func mapVoteError(err error) error
func toVote(v *models.Vote) *pb.Vote
```

- Both handlers read the **optional** JWT via `auth.UserIDFromContext` (NOT
  `mustUserID` — anonymous callers are valid here) into a `*uuid.UUID`.
- `req.Id` is parsed with `uuid.Parse` → `400 invalid_input` on failure.
- `req.Answers` is mapped to `[]vote.AnswerInput` (string UUIDs → `uuid.UUID`).
- `mapVoteError` maps the five domain errors to the table above.
- `toVote` maps `*models.Vote` → `*pb.Vote` (nil `UserID` → `""`).

`VyborokHandler` gains a `vote *vote.Service` field; `handler.New` gains a
`voteSvc *vote.Service` parameter. `main.go` constructs the repository and
service and passes them in.

## Alternatives Considered

- **Option A: normalized `vote_answers` child table.**
  Rejected for MVP: the TODO specifies a single `votes` table; JSONB keeps the
  schema simple and Phase 4 can aggregate at MVP scale. Revisit if stats
  queries become painful.

- **Option B: "true anonymity" — always discard identity on anonymous polls.**
  Rejected (was the original proposal): a logged-in voter on an anonymous poll
  would not be deduplicated, so the same person could vote twice (logged in,
  then logged out). The hybrid semantics from the requirements "Design note"
  are chosen instead: identity is recorded whenever a JWT is present.

- **Option C: `google.protobuf.Int32Value` wrapper for the scale value.**
  Rejected: the codebase uses `_set` companion fields (constitution §9), not
  wrapper types (the wrappers import is commented out in `main.proto`).

- **Option D (chosen): single `votes` table + JSONB answers, `_set` for the
  scale value, `UNIQUE (poll_id, user_id)` as the repeat backstop, hybrid
  `anonymous` semantics (identity recorded when a JWT is present).**
  Matches the TODO, minimal schema, clear semantics.

## Open Questions
None. Q1–Q4 are resolved in `requirements.md`. Q2 (HasVoted path) is verified
against the micro v3 router at T8; fallback is `GET /polls/{id}/has-voted`.

