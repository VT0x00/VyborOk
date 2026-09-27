# Feature: GetPoll

**Spec ID:** 0025
**Phase:** 2 (Polls CRUD)
**Status:** approved

## Summary
Authenticated and anonymous clients fetch a single poll by its ID, together
with all its questions and options. Visibility depends on poll status and
whether the caller is the owner.

## User Stories
- As the poll owner, I want to view my poll in full (all questions and options)
  so I can review what I created.
- As an anonymous visitor, I want to view an active poll so I can decide
  whether to vote.
- As a non-owner, I want to see a closed poll's structure (title, questions,
  options) so I understand what was asked — but without any of the owner's
  private data.

## Acceptance Criteria

- **R1:** When `GET /polls/{id}` is called with a valid UUID of an existing
  poll, the response is `200` with the full `Poll` object: `id`, `user_id`,
  `title`, `description`, `status`, `anonymous`, `created_at`, `updated_at`,
  and `questions[]` with nested `options[]`.
- **R2:** When the caller is the poll owner (JWT present, `userID == poll.user_id`),
  the response is identical to R1 for both `active` and `closed` polls.
- **R3:** When the caller is NOT the owner and the poll is `active`,
  the response is `200` with the full poll (same as R1).
- **R4:** When the caller is NOT the owner and the poll is `closed`,
  the response is `200` with a **stripped** poll: `id`, `user_id`, `title`,
  `description`, `status`, `anonymous`, `created_at`, `updated_at`, and
  `questions[]` (structure only) — **no votes, no stats** (stats come Phase 4).
- **R5:** When `{id}` is not a valid UUID, the response is `400 invalid_input`.
- **R6:** When the poll does not exist, the response is `404 not_found`.
- **R7:** When the caller is anonymous and the poll is `closed` and belongs
  to a user with `is_private = true`, the response is `404 not_found`
  (do not leak the poll's existence).
- **R8:** When the caller is anonymous and the poll is `closed` and the
  owner is public, the response is R4.

## Out of Scope
- Statistics, vote counts, completion rates (Phase 4).
- Any mutation of the poll (Update / Delete / Close — Day 27).
- Pagination (single poll, no pagination needed).
- Media URLs in questions (Phase 10).
- Geo restrictions (Phase 9).

## Open Questions
None. Resolved:
- Closed poll visible to non-owner only if owner is public (R7 → 404 otherwise).
- `GET /polls/{id}` is public; JWT honored if present.
