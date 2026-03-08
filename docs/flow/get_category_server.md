## Get Category Server Flow

### Overview
Retrieves a paginated list of server categories. Supports cursor-based pagination.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Parse query parameters: `limit` (default: system default), `cursor` (optional)
2. Validate `limit` (>= 0 and <= max limit)
3. If cursor provided, decode from base64 and unmarshal
4. Query categories from database (limit + 1 for cursor detection)
5. Build next cursor if more data exists

### Query Parameters
- `limit` — number of items per page (optional)
- `cursor` — base64-encoded cursor for next page (optional)

### Error Cases
- Limit < 0 → `400` with "Limit must be greater or equal than 0"
- Limit > max → `400` with "Limit is exceeded max limit: {max}"

### Flow
```
Request → Auth Middleware → Parse Query Params → Validate Limit → Decode Cursor → Query Categories (DB) → Build Next Cursor → Response
```
