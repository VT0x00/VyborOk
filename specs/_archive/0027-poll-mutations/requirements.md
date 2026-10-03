# Feature: Poll Mutations (Update / Delete / Close)

**Spec ID:** 0027
**Phase:** 2 (Polls CRUD)
**Status:** approved

## Summary
Three mutation operations on a poll, all owner-scoped:
- **UpdatePoll** — change `title`, `description`, `anonymous` on an active poll.
- **DeletePoll** — permanently remove a poll (and cascade to questions/options).
- **ClosePoll** — transition `status: active → closed`. Idempotent.

Questions and options are immutable after creation (constitution §8). Status
changes only via `ClosePoll`. There is no "reopen" — a closed poll stays closed.

## User Stories
- As a poll owner, I want to fix a typo in my poll's title/description before
  it collects votes.
- As a poll owner, I want to toggle "anonymous" before voting starts.
- As a poll owner, I want to close a poll so it stops accepting votes.
- As a poll owner, I want to delete a poll I no longer need.
- As an API client, I want `ClosePoll` to be retry-safe (idempotent) so a
  network hiccup doesn't leave the UI in an inconsistent state.

## Acceptance Criteria

### UpdatePoll — `PUT /polls/{id}`

- **R1:** Without `Authorization` → `401 unauthorized`.
- **R2:** Owner sends `{ "title": "new title" }` → `200`; `title` changed,
  `description` and `anonymous` unchanged, `questions[]` unchanged.
- **R3:** Owner sends `{ "title": "", "description": "", "anonymous_set": false }`
  → `200`; nothing changes (partial-update semantics: empty string means
  "leave alone", false `_set` means "leave alone").
- **R4:** Owner sends `{ "anonymous": true, "anonymous_set": true }` →
  `anonymous` set to `true`. Same with `false` + `_set: true` → set to `false`.
- **R5:** Owner sends `{ "title": "<256 chars>" }` → `400 invalid_input`.
  Title limit is 255 runes.
- **R6:** Non-owner (valid JWT, different user) → `404 not_found`.
  Same for a random UUID → `404 not_found`.
- **R7:** Owner sends an update to a `closed` poll → `409 poll_closed`.
  UpdatePoll only works on `active` polls. (Per the resolved Open Question:
  `microerr.Conflict` exists in micro v3, verified at T1 → 409.)

### DeletePoll — `DELETE /polls/{id}`

- **R8:** Without `Authorization` → `401 unauthorized`.
- **R9:** Owner deletes an `active` poll → `200` with empty body `{}`.
  Row is gone; cascade removes questions and options.
- **R10:** Owner deletes a `closed` poll → `200`. Allowed regardless of status.
- **R11:** Non-owner → `404 not_found`. Random UUID → `404 not_found`.
- **R12:** Calling `DELETE` twice on the same poll → first `200`, second `404`.

### ClosePoll — `POST /polls/{id}/close`

- **R13:** Without `Authorization` → `401 unauthorized`.
- **R14:** Owner closes an `active` poll → `200` with the full `Poll` object
  (questions and options present), `status: "closed"`.
- **R15:** Owner calls `ClosePoll` on an already `closed` poll → `200` with
  the same full `Poll`, `status: "closed"`. **Idempotent.**
- **R16:** Non-owner → `404 not_found`. Random UUID → `404 not_found`.
- **R17:** After `ClosePoll`, `GetPoll` by owner still returns the full poll;
  `GetPoll` by a non-owner returns the stripped structure (R4 of spec 0025).

## Out of Scope
- Reopen (transition `closed → active`).
- Editing `questions` or `options` after creation.
- Soft delete. Deletion is physical.
- Changing status via `UpdatePoll` — status only via `ClosePoll`.
- Bulk operations (close all my polls).
- Audit log of who deleted what and when.

## Open Questions
None. Resolved:
- UpdatePoll on closed poll → 409 `poll_closed` if `microerr.Conflict` exists,
  else 400 (verified at T1).
- Repeated `ClosePoll` refreshes `updated_at` (simple UPDATE).
- Empty-string / false `_set` = "leave unchanged" (partial update semantics).