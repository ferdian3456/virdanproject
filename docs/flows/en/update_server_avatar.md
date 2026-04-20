## Update Server Avatar Flow

### Overview
Updates the avatar image of a server. Only the server owner can perform this action. Accepts multipart form file upload.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. Check server ownership
4. Read `avatar` file from multipart form
5. Validate image file (format, size)
6. Begin database transaction
7. Get current server detail to find old avatar
8. If new avatar provided: create new image record → update server reference → upload to MinIO → delete old image if exists
9. If no new avatar (empty file): update server to remove reference → delete old image if exists
10. Commit transaction

### Database Operations

#### PostgreSQL — Check Server Ownership
```sql
SELECT 1 FROM servers WHERE id = $1 AND owner_id = $2
```

#### PostgreSQL — Get Server Detail (inside TX, for old avatar)
```sql
SELECT id, owner_id, name, short_name, category_id, avatar_image_id, banner_image_id, description, settings, create_datetime, update_datetime, create_user_id, update_user_id
FROM servers WHERE id = $1
```
- Used to get `avatar_image_id` for old avatar cleanup

#### PostgreSQL — Create New Avatar Image (inside TX)
```sql
INSERT INTO server_avatar_images (id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
```
- **Table**: `server_avatar_images`
- **Columns**: `id` (UUID), `bucket`, `object_key` (e.g., `server/avatar/{imageId}.webp`), `mime_type` (`image/webp`), `size`

#### PostgreSQL — Update Server Avatar Reference (inside TX)
```sql
UPDATE servers SET avatar_image_id = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4
```
- Sets `avatar_image_id` to new image UUID or NULL (when removing)

#### PostgreSQL — Get Old Avatar Object Key (inside TX)
```sql
SELECT object_key FROM server_avatar_images WHERE id = $1 LIMIT 1
```

#### PostgreSQL — Delete Old Avatar Image Record (inside TX)
```sql
DELETE FROM server_avatar_images WHERE id = $1
```

#### MinIO — Upload New Avatar
```
PUT {bucket}/server/avatar/{imageId}.webp
Content-Type: image/webp
Cache-Control: public, max-age=31536000, immutable
```

#### MinIO — Delete Old Avatar (if exists)
```
DELETE {bucket}/{old_object_key}
```

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Not owner → `400` with "You are not the owner of this server"
- Invalid image → `400` validation error

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Ownership (DB) → Read File → Validate Image → Begin TX → Handle Old/New Avatar → Commit TX → Response (no data)
```
