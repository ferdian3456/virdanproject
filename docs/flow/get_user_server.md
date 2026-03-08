## Get User Server Flow

### Overview
Retrieves a paginated list of servers that the authenticated user is a member of. Supports cursor-based pagination.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse query parameters: `limit` (default: system default), `cursor` (optional)
3. Validate `limit` (>= 0 and <= max limit)
4. If cursor provided, decode from base64 and unmarshal to `ServerUserCursor`
5. Query servers where user is a member (limit + 1 for cursor detection)
6. Build MinIO image URLs for server avatars
7. If results > limit, create next cursor from last item
8. Return paginated server list

### Database Operations

#### PostgreSQL — Get User Servers (first page)
```sql
SELECT B.id, B.name, B.short_name, C.object_key, A.joined_datetime
FROM server_members A
INNER JOIN servers B ON A.server_id = B.id
LEFT JOIN server_avatar_images C ON C.id = B.avatar_image_id
WHERE A.user_id = $1
ORDER BY A.joined_datetime DESC, A.server_id DESC
LIMIT $2
```

#### PostgreSQL — Get User Servers (with cursor)
```sql
SELECT B.id, B.name, B.short_name, C.object_key, A.joined_datetime
FROM server_members A
INNER JOIN servers B ON A.server_id = B.id
LEFT JOIN server_avatar_images C ON C.id = B.avatar_image_id
WHERE (A.joined_datetime < $1 OR (A.joined_datetime = $1 AND A.server_id < $2)) AND A.user_id = $3
ORDER BY A.joined_datetime DESC, A.server_id DESC
LIMIT $4
```
- **Tables**: `server_members` (A) INNER JOIN `servers` (B), LEFT JOIN `server_avatar_images` (C)
- **Cursor**: keyset pagination on `(joined_datetime, server_id)` DESC
- **Columns returned**: `id`, `name`, `short_name`, `object_key` (avatar), `joined_datetime`

### Error Cases
- Limit < 0 → `400` with "Limit must be greater or equal than 0"
- Limit > max → `400` with "Limit is exceeded max limit: {max}"

### Flow
```
Request → Auth Middleware → Parse Query Params → Validate Limit → Decode Cursor → Query User Servers (DB) → Build Image URLs → Build Next Cursor → Response
```
