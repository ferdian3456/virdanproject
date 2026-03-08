## Get Server Posts Flow

### Overview
Retrieves a paginated list of posts from a specific server. Requires server membership. Uses cursor-based pagination.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` from URL path — must be a valid UUID
3. Parse query parameters: `limit`, `cursor`
4. Validate `limit` (>= 0 and <= max limit)
5. Check if user is a member of the server
6. If cursor provided, decode and unmarshal
7. Query posts from database (limit + 1 for cursor detection) with author info and image URLs
8. Build next cursor if more data exists

### Query Parameters
- `limit` — number of items per page (optional)
- `cursor` — base64-encoded cursor for next page (optional)

### Error Cases
- Invalid serverId → `400` with "Invalid server id"
- Limit < 0 → `400` with "Limit must be greater or equal than 0"
- Limit > max → `400` with "Limit is exceeded max limit: {max}"
- Not a member → `400` with "You are not a member of this server"

### Flow
```
Request → Auth Middleware → Parse Params → Validate Limit → Check Membership (DB) → Decode Cursor → Query Posts (DB) → Build Next Cursor → Response
```
