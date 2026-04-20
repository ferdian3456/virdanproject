## Get Server By ID Flow

### Overview
Retrieves detailed information about a specific server. Access is controlled: public servers are visible to all authenticated users, private servers are only visible to members.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path parameter — must be a valid UUID
3. Query server details with a single query that checks both existence AND membership for private servers
4. If server not found (or private and user is not a member), return error
5. Build full MinIO URLs for avatar and banner images
6. Return server detail

### Database Operations

#### PostgreSQL — Get Server By ID (with access control)
```sql
SELECT A.id, A.name, A.short_name, B.name, C.object_key, D.object_key,
       A.description, A.create_datetime, E.username, (A.settings->>'isPrivate')::boolean as is_private
FROM servers A
LEFT JOIN server_categories B ON A.category_id = B.id
LEFT JOIN server_avatar_images C ON A.avatar_image_id = C.id
LEFT JOIN server_banner_images D ON A.banner_image_id = D.id
LEFT JOIN users E ON A.create_user_id = E.id
WHERE A.id = $1
AND (
    (A.settings->>'isPrivate')::boolean = false
    OR
    EXISTS (
        SELECT 1 FROM server_members F
        WHERE F.server_id = A.id AND F.user_id = $2 AND F.status = 1
    )
)
```
- **Tables**: `servers` (A) LEFT JOIN `server_categories` (B), `server_avatar_images` (C), `server_banner_images` (D), `users` (E)
- **Subquery**: checks `server_members` for active membership if server is private
- **Columns returned**: `id`, `name`, `short_name`, `category_name`, avatar `object_key`, banner `object_key`, `description`, `create_datetime`, `username` (creator), `is_private`

#### MinIO — Image URL Construction
```
Avatar: {MINIO_FULL_URL}/{object_key}  (e.g., server/avatar/{imageId}.webp)
Banner: {MINIO_FULL_URL}/{object_key}  (e.g., server/banner/{imageId}.webp)
```

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Server not found or no permission → `400` with "Server not found or you don't have permission to access it"

### Flow
```
Request → Auth Middleware → Parse ServerId → Query Server (DB, checks visibility + membership) → Build Image URLs → Response
```
