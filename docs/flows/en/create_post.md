## Create Post Flow

### Overview
Creates a new post in a server. Requires server membership. Posts must include an image (uploaded as multipart form) and a caption.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` from URL path — must be a valid UUID
3. Check if user is a member of the server
4. Read `image` file from multipart form (required, cannot be empty)
5. Validate image file (format, size)
6. Read `caption` from form value (required)
7. Begin database transaction
8. Upload image to MinIO (`server/post/{imageId}.webp`)
9. Create post image record in database
10. Create post record in database
11. Commit transaction
12. Fetch and return full post details including author info and image URL

### Database Operations

#### PostgreSQL — Check Server Member
```sql
SELECT 1 FROM server_members WHERE server_id = $1 AND user_id = $2 AND status = $3
```
- **Table**: `server_members`
- **Filter**: `server_id`, `user_id`, `status = 1` (active)

#### PostgreSQL — Create Post Image (inside TX)
```sql
INSERT INTO server_post_images (id, bucket, object_key, mime_type, size, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
```
- **Table**: `server_post_images`
- **Columns**: `id` (UUID), `bucket`, `object_key` (e.g., `server/post/{imageId}.webp`), `mime_type` (`image/webp`), `size`

#### PostgreSQL — Create Post (inside TX)
```sql
INSERT INTO server_posts (id, server_id, author_id, post_image_id, caption, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
```
- **Table**: `server_posts`
- **Columns**: `id` (UUID), `server_id`, `author_id`, `post_image_id` (FK to `server_post_images`), `caption`

#### PostgreSQL — Get Post Details (after creation)
```sql
SELECT sp.author_id, sp.id, spi.object_key, sp.caption, sp.create_datetime, sp.update_datetime,
       COALESCE(comment_counts.comment_count, 0), COALESCE(like_counts.like_count, 0)
FROM server_posts sp
INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
LEFT JOIN (SELECT post_id, COUNT(*) as comment_count FROM server_post_comments GROUP BY post_id) comment_counts ON sp.id = comment_counts.post_id
LEFT JOIN (SELECT post_id, COUNT(*) as like_count FROM server_post_likes GROUP BY post_id) like_counts ON sp.id = like_counts.post_id
WHERE sp.id = $1
```

#### MinIO — Upload Post Image
```
PUT {bucket}/server/post/{imageId}.webp
Content-Type: image/webp
Cache-Control: public, max-age=31536000, immutable
```

### Error Cases
- Invalid serverId → `400` with "Invalid server id"
- Not a member → `400` with "You are not a member of this server"
- No image file → `400` with "Image is required"
- Invalid image format → `400` validation error
- Caption empty → `400` with "Caption is required"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Membership (DB) → Read Image → Validate Image → Read Caption → Begin TX → Upload Image (MinIO) → Create Image Record (DB) → Create Post (DB) → Commit TX → Get Post (DB) → Response
```
