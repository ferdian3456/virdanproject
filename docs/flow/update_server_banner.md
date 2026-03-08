## Update Server Banner Flow

### Overview
Updates the banner image of a server. Only the server owner can perform this action. Accepts multipart form file upload.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. Check server ownership
4. Read `banner` file from multipart form
5. Validate image file (format, size)
6. Begin database transaction
7. Get current server detail to find old banner
8. If new banner provided: create new image record → update server reference → upload to MinIO → delete old image if exists
9. If no new banner: update server to remove reference → delete old image if exists
10. Commit transaction

### Database Operations

#### PostgreSQL — Check Server Ownership
```sql
SELECT 1 FROM servers WHERE id = $1 AND owner_id = $2
```

#### PostgreSQL — Create New Banner Image (inside TX)
```sql
INSERT INTO server_banner_images (id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
```
- **Table**: `server_banner_images`
- **Columns**: `id` (UUID), `bucket`, `object_key` (e.g., `server/banner/{imageId}.webp`), `mime_type` (`image/webp`), `size`

#### PostgreSQL — Update Server Banner Reference (inside TX)
```sql
UPDATE servers SET banner_image_id = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4
```

#### PostgreSQL — Get Old Banner Object Key (inside TX)
```sql
SELECT object_key FROM server_banner_images WHERE id = $1 LIMIT 1
```

#### PostgreSQL — Delete Old Banner Image Record (inside TX)
```sql
DELETE FROM server_banner_images WHERE id = $1
```

#### MinIO — Upload New Banner
```
PUT {bucket}/server/banner/{imageId}.webp
Content-Type: image/webp
Cache-Control: public, max-age=31536000, immutable
```

#### MinIO — Delete Old Banner (if exists)
```
DELETE {bucket}/{old_object_key}
```

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Not owner → `400` with "You are not the owner of this server"
- Invalid image → `400` validation error

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Ownership (DB) → Read File → Validate Image → Begin TX → Handle Old/New Banner → Commit TX → Response (no data)
```
