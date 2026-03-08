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

### Database Operations

#### PostgreSQL — Check Post Server Membership
```sql
SELECT 1
FROM server_posts sp
INNER JOIN server_members sm ON sp.server_id = sm.server_id
WHERE sp.id = $1 AND sm.user_id = $2 AND sm.status = $3
```

#### PostgreSQL — Get Comments (first page)
```sql
SELECT id, author_id, parent_id, content, create_datetime, update_datetime
FROM server_post_comments
WHERE post_id = $1
ORDER BY create_datetime DESC, id DESC
LIMIT $2
```

#### PostgreSQL — Get Comments (with cursor)
```sql
SELECT id, author_id, parent_id, content, create_datetime, update_datetime
FROM server_post_comments
WHERE post_id = $1
AND (create_datetime < $2 OR (create_datetime = $2 AND id < $3))
ORDER BY create_datetime DESC, id DESC
LIMIT $4
```
- **Table**: `server_post_comments`
- **Columns returned**: `id` (UUID), `author_id`, `parent_id` (nullable, for nested replies), `content`, `create_datetime`, `update_datetime`
- **Cursor**: keyset pagination on `(create_datetime, id)` DESC

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
