# Feature: ListPolls

**Spec ID:** 0026
**Phase:** 2 (Polls CRUD)
**Status:** approved

## Summary
An authenticated user fetches a paginated list of their own polls, newest
first. Each item is a lightweight card: metadata plus a question count, but
no nested questions or options (that would be wasteful for a list view).

## User Stories
- As a poll owner, I want to see all my polls so I can manage them.
- As a poll owner with many polls, I want pagination so the response stays
  small and the UI can scroll.
- As a poll owner, I want the list ordered newest-first so my recent activity
  is at the top.

## Acceptance Criteria

- **R1:** `GET /polls` without `Authorization` → `401 unauthorized`.
- **R2:** `GET /polls` with a valid JWT and the user has no polls →
  `200 { "polls": [] }` (empty array, not `null`, not 404).
- **R3:** `GET /polls` with a valid JWT and the user has N polls (N ≤ default
  limit) → `200` with all N items, ordered by `created_at DESC`.
  Each item contains: `id`, `title`, `description`, `status`, `anonymous`,
  `question_count`, `created_at`. No `questions` field, no `options`.
- **R4:** `GET /polls?limit=2&offset=0` → first 2 (newest).
- **R5:** `GET /polls?limit=2&offset=2` → next 2 (older).
- **R6:** `GET /polls?limit=500` → `limit` is clamped to 100 server-side;
  response contains at most 100 items. No error.
- **R7:** `GET /polls?limit=-5` → `limit` normalizes to default (20).
  No error.
- **R8:** `GET /polls?offset=-1` → `offset` normalizes to 0. No error.
- **R9:** User A never sees User B's polls, even with a valid JWT and correct
  pagination. The list is implicitly scoped to the authenticated user.
- **R10:** `question_count` equals the number of rows in `questions` where
  `poll_id = poll.id`. For a poll with 3 questions → `question_count: 3`.
- **R11:** A poll with 0 questions (should not exist due to creation
  validation, but defensive) → `question_count: 0`, still returned.

## Out of Scope
- Filtering by status (`?status=active`). Deferred to a later phase if needed.
- Sorting options other than `created_at DESC`.
- Public feed of all polls (users' public polls). Not in MVP.
- Total count / total pages. Not in MVP — see Open Questions.
- Cursor-based pagination. Limit/offset is enough for MVP.

## Open Questions
1. **`total` field?** Would require an extra `COUNT(*)` query. MVP: no.
   The client knows it's at the end when `len(polls) < limit`. Revisit in
   Phase 8 (frontend) if the paginator UX demands it.
2. **Offset vs cursor pagination.** Offset has the "shifting window" problem
   if polls are inserted between page fetches (a new poll pushes the whole
   list down by one). For MVP with low write volume, this is acceptable.
   Revisit if it becomes a real problem.
3. **`GET /polls?username=...` for public profiles.** Deferred. If we add it,
   it will be a separate spec.
