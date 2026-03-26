# ADR-005: Cursor-Based Over Offset Pagination

## Status
Accepted

## Context
The user listing endpoint needs pagination. Offset-based pagination (`OFFSET N LIMIT M`) is simple but degrades on large datasets because the database must scan and discard `N` rows. It also produces inconsistent results when rows are inserted or deleted between page fetches.

## Decision
We use bidirectional cursor-based pagination. The cursor is a base64-encoded JSON object containing `last_id` and `direction` (next/prev). The query fetches `limit + 1` rows to determine if more pages exist. Forward pagination uses `WHERE id < cursor ORDER BY id DESC`; backward pagination reverses the condition.

## Consequences
- **Pro:** Consistent O(1) performance regardless of page depth -- always uses an index seek
- **Pro:** Stable results even when rows are inserted or deleted
- **Pro:** Bidirectional navigation (next and prev cursors in every response)
- **Con:** Cannot jump to an arbitrary page number
- **Con:** More complex implementation than simple offset/limit
- **Con:** Cursor encoding/decoding adds a small layer of abstraction
