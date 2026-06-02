# Search Server Posts

Search posts inside a single server by caption.

## Flow

1. Validate `serverId` (UUID) and `limit` (0..20).
2. Validate `q`: trimmed, length 2..100. Below 2 -> 400.
3. Membership check: requester must be a member (else 403).
4. Decode cursor if present. Invalid cursor -> 400.
5. Query posts in that server with `caption ILIKE '%q%'`, ordered `created_at DESC, id DESC`, cursor-paginated.
6. Return `data` + `page.nextCursor`.

## Notes

- Scoped to the active server only (no cross-server).
- Caption is the only searched field.
- A `pg_trgm` (GIN) index backs the ILIKE so it does not seq-scan.
