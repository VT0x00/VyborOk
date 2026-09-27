# Tasks: <name>

**Spec ID:** NNNN
**Depends on:** `requirements.md`, `design.md`

## Checklist
Each task: one file (or one small edit), one commit, one checkbox.
Order matters — later tasks assume earlier ones are done.

- [ ] **T1:** <concrete step + file path>
- [ ] **T2:** ...
- [ ] **T3:** ...

## Traceability
Map every acceptance criterion from `requirements.md` to at least one task.

- R1 → T1, T2
- R2 → T3
- R3 → T4

## Verification
- Automated: `make test-all` (must be green).
- Manual: `curl <command>` → expected output.
- DB check: `psql` query → expected rows.

## Rollback
How to undo if this ships and breaks. For MVP: revert commit + `make migrate-down`.

