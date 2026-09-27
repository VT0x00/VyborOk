# Design: <name>

**Spec ID:** NNNN
**Depends on:** `requirements.md`

## Data Model
- New tables / columns / indexes / constraints.
- Migration file names (`000NNN_*.up.sql`, `*.down.sql`).
- Backfill strategy if altering existing data (usually none in MVP).

## API
- New or changed RPCs in `main.proto`.
- Request / response messages.
- Path / HTTP method.
- Error codes returned.

## Service Layer
- Method signatures.
- Validation rules (rune limits, required fields, cross-field invariants).
- Transaction boundaries (which repos are called together).
- Domain errors returned.

## Repository Layer
- New methods per repository.
- SQL sketch (rough, not final).
- Which queries need indexes.

## Alternatives Considered
- **Option A:** ... — rejected because ...
- **Option B:** ... — chosen because ...

## Open Questions
- Things unresolved. Must be empty before moving to `tasks.md`.

