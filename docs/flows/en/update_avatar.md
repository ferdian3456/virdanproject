## Update Avatar Flow

### Overview
Updates the authenticated user's avatar image. Accepts a multipart form file upload. The image is validated, converted to WebP, and stored in MinIO object storage.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context (set by auth middleware)
2. Read `avatar` field from multipart form data
3. Validate the image file (format, size)
4. Begin database transaction
5. Check if user already has an avatar image
6. If old avatar exists:
   - Delete old avatar record from database
   - Delete old avatar file from MinIO
7. Create new avatar image record in database (with new UUID, bucket, object key)
8. Upload new avatar file to MinIO (`user/avatar/{imageId}.webp`)
9. Commit transaction
10. Return success with no data

### Database Operations

#### PostgreSQL — Get Existing Avatar (inside TX)
```sql
SELECT object_key FROM user_avatar_images WHERE user_id = $1 LIMIT 1
```
- **Table**: `user_avatar_images`
- **Column**: `object_key` (path in MinIO)

#### PostgreSQL — Delete Old Avatar Record (inside TX)
```sql
DELETE FROM user_avatar_images WHERE user_id = $1
```

#### PostgreSQL — Insert New Avatar Record (inside TX)
```sql
INSERT INTO user_avatar_images (id, user_id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
```
- **Table**: `user_avatar_images`
- **Columns**: `id` (UUID), `user_id`, `bucket` (MinIO bucket name), `object_key` (e.g., `user/avatar/{imageId}.webp`), `mime_type` (`image/webp`), `size` (bytes), timestamps, audit IDs

#### MinIO — Upload Avatar
```
PUT {bucket}/user/avatar/{imageId}.webp
Content-Type: image/webp
Cache-Control: public, max-age=31536000, immutable
```

#### MinIO — Delete Old Avatar (if exists)
```
DELETE {bucket}/{old_object_key}
```

### Error Cases
- No/invalid auth token → `404` (handled by auth middleware)
- Missing avatar file → `400` with "Avatar is required to not be empty"
- Invalid image format/size → `400` validation error
- MinIO upload failure → `500` internal server error

### Flow
```
Request → Auth Middleware → Get Avatar File → Validate Image → Begin TX → Check Old Avatar → Delete Old (DB + MinIO) → Create New Record (DB) → Upload (MinIO) → Commit TX → Response
```
