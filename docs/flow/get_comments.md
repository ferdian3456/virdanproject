## Get Comments Flow

### Overview
Retrieves a paginated list of comments for a specific post. Requires membership in the server where the post belongs. Uses cursor-based pagination.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` from URL path — must be a valid UUID
3. Parse query parameters: `limit`, `cursor`
4. Validate `limit` (>= 0 and <= max limit)
5. Check if user is a member of the server where the post belongs
6. If cursor provided, decode and unmarshal
7. Query comments from database (limit + 1 for cursor detection)
8. Build next cursor if more data exists

### Query Parameters
- `limit` — number of items per page (optional)
- `cursor` — base64-encoded cursor for next page (optional)

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Limit < 0 → `400`
- Limit > max → `400`
- Not a member → `400` with "You are not a member of this server"

### Flow
```
Request → Auth Middleware → Parse PostId → Validate Limit → Check Membership (DB) → Decode Cursor → Query Comments (DB) → Build Next Cursor → Response
```
