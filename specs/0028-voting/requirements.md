# Feature: Voting (SubmitVote / HasVoted)

**Spec ID:** 0028
**Phase:** 3 (Voting)
**Status:** approved

## Summary
Let a caller submit answers to a poll's questions and check whether they have
already voted. A vote is an atomic submission of exactly one answer per
question, accepted only on `active` polls.

Voting has two modes, controlled by the poll's `anonymous` flag:

- **Anonymous poll (`anonymous = true`):** login is optional.
  - **No JWT:** vote stored with `user_id = NULL`; repeat voting by the
    same caller is not prevented (we cannot identify them).
  - **JWT present:** vote stored with `user_id` set to the caller; repeat
    voting returns `409 already_voted`.
- **Non-anonymous poll (`anonymous = false`):** login required; vote tied
  to the user; repeat voting returns `409 already_voted`.

This is the first feature that writes to the `votes` table and the first to
accept unauthenticated writes.

### Design note: semantics of `poll.anonymous`

`anonymous = true` means "login is not required". It does **not** mean "we
discard the caller's identity if they happen to be logged in". If a caller
authenticates, we know who they are, and we use that to prevent double
voting — the same way we would on a non-anonymous poll.

Trade-off: a user can vote while logged in (identity recorded, dedup applies),
then log out and vote again anonymously (identity not recorded, no dedup
possible). This hole is inherent to "allow anonymous voting" and is
acknowledged. Closing it requires IP or fingerprint tracking, which is
Post-MVP work.

If a future product decision flips this to "true anonymity" (discard identity
even when present), it's a small, localized change: stop writing `user_id`
on anonymous polls. See R7.

## User Stories
- As a visitor, I want to answer a poll's questions and submit, so that my
  response is recorded.
- As a visitor on an anonymous poll, I want to vote without creating an account.
- As a logged-in user, I want my vote tied to my account so I cannot vote twice,
  on both anonymous and non-anonymous polls.
- As a visitor, I want to know whether I have already voted, so the UI can show
  "You already voted" instead of the form.
- As a poll owner, I want voting to stop once the poll is closed.

## Acceptance Criteria

### SubmitVote — `POST /polls/{id}/vote`

- **R1:** When `{id}` is not a valid UUID → `400 invalid_input`.
- **R2:** When the poll does not exist → `404 not_found`.
- **R3:** When the poll is `closed` → `409 poll_closed`.
- **R4:** When the poll is non-anonymous and there is no valid JWT →
  `401 unauthorized`.
- **R5:** When the poll is non-anonymous, a valid JWT is present, and the
  answers are valid → `200` with the created `Vote`; the stored vote has
  `user_id` set.
- **R6:** When the poll is anonymous and there is no JWT, with valid answers
  → `200`; the stored vote has `user_id = NULL`.
- **R7:** When the poll is anonymous and a valid JWT is present, with valid
  answers → `200`; the stored vote has `user_id` set to the caller (identity
  is recorded; see "Design note" above).
- **R8:** When a non-anonymous poll is voted on twice by the same user →
  the second call is `409 already_voted`.
- **R9:** When an anonymous poll is voted on more than once:
  - **R9a (no JWT):** every call succeeds (`200`); each stored vote has
    `user_id = NULL`. Repeat voting is not prevented.
  - **R9b (same JWT):** the first call succeeds; the second call is
    `409 already_voted`.
- **R10:** A `single` question answered with 0 options → `400 invalid_input`.
- **R11:** A `single` question answered with 2+ options → `400 invalid_input`.
- **R12:** A `single`/`multiple` answer references an option that does not
  belong to the question → `400 invalid_input`.
- **R13:** A `multiple` question answered with 0 options → `400 invalid_input`.
- **R14:** A `multiple` answer contains a duplicate option → `400 invalid_input`.
- **R15:** A `scale` answer with a value outside `[scale_min, scale_max]` →
  `400 invalid_input`.
- **R16:** A `scale` answer with a value inside the range → `200`.
- **R17:** A `text` answer longer than the question's `text_max_length`
  (runes) → `400 invalid_input`.
- **R18:** A `text` answer that is empty or whitespace-only → `400 invalid_input`.
- **R19:** A `text` answer within the limit → `200`.
- **R20:** A question is missing from the answers (not all questions covered)
  → `400 invalid_input`.
- **R21:** The same question appears twice in the answers → `400 invalid_input`.
- **R22:** An answer references a `question_id` that is not part of the poll →
  `400 invalid_input`.
- **R23:** An answer carries the wrong payload for the question type (e.g.
  `option_ids` for a `scale` question, or `value` for a `single` question) →
  `400 invalid_input`.

### HasVoted — `GET /polls/{id}/vote`

- **R24:** When `{id}` is not a valid UUID → `400 invalid_input`.
- **R25:** When the poll does not exist → `404 not_found`.
- **R26:** When there is no JWT → `200` with `has_voted = false`
  (anonymous caller cannot be checked; UI shows the form).
- **R27:** When a JWT is present and the user has not voted → `200` with
  `has_voted = false`.
- **R28:** When a JWT is present and the user has voted (on a non-anonymous
  poll, or on an anonymous poll while identified) → `200` with
  `has_voted = true`.
- **R29:** When a JWT is present but the caller previously voted on this poll
  without a JWT (their vote has `user_id = NULL`), the call returns
  `200` with `has_voted = false`. The prior anonymous vote is not linked to
  the now-identified caller — that information does not exist.

## Out of Scope
- Statistics / vote counts / percentages (Phase 4, `GetStats`).
- Editing or withdrawing a vote (a vote is final once submitted).
- CAPTCHA / anti-fraud for anonymous repeat voting (Post-MVP backlog).
- IP- or fingerprint-based deduplication of anonymous votes (Post-MVP backlog).
- Geo restrictions on voting (Phase 9).
- Partial voting (submitting a subset of questions) — MVP requires all.
- Returning the caller's prior vote payload from `HasVoted` (a future
  `GET /polls/{id}/my-vote` may supersede it).

## Open Questions

**Resolved:**

- **Q1 (semantics of `poll.anonymous`):** hybrid. `anonymous = true` means
  "login is optional"; if a JWT is present, `user_id` is recorded and dedup
  applies. See "Design note" above. Chosen over "discard identity always"
  (loses dedup) and over "always require login" (breaks the flag's purpose).
- **Q2 (`HasVoted` path):** `GET /polls/{id}/vote` (same path as
  `SubmitVote`, different HTTP method). Fallback if the micro v3 router does
  not accept two methods on one path: `GET /polls/{id}/has-voted`. To be
  verified in design (T1).
- **Q3 (repeat-vote HTTP code):** `409 already_voted` (`microerr.Conflict`
  exists in micro v3, verified in spec 0027 T1).

**New:**

- **Q4 (owner voting on own poll):** no restriction. An owner can vote on
  their own poll, subject to the same dedup rules. Rationale: in small
  communities the owner is a legitimate participant; preventing it would
  need explicit product intent we don't have. Flagged here so the decision
  is visible; revisit if abused.
