## Delete Post Flow

### Overview
Deletes a post from a server. Only the post author and server member can delete the post. Cascade deletes comments, likes, and also removes the image from MinIO.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` and `postId` from URL path — must be valid UUIDs
3. Check if user is a member of the server
4. Check if user is the author of the post
5. Begin database transaction
6. Get post image info (imageId, objectKey)
7. Delete post from database (CASCADE deletes comments and likes)
8. Delete post image record from database
9. Commit transaction
10. Delete image file from MinIO

### Database Operations

#### PostgreSQL — Check Server Member
```sql
SELECT 1 FROM server_members WHERE server_id = $1 AND user_id = $2 AND status = $3
```

#### PostgreSQL — Check Post Ownership
```sql
SELECT 1 FROM server_posts WHERE id = $1 AND author_id = $2
```

#### PostgreSQL — Get Post Image Info (inside TX)
```sql
SELECT sp.post_image_id, spi.object_key
FROM server_posts sp
INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
WHERE sp.id = $1
```
- **Tables**: `server_posts` JOIN `server_post_images`
- **Returns**: `post_image_id` (UUID), `object_key` (MinIO path)

#### PostgreSQL — Delete Post (inside TX)
```sql
DELETE FROM server_posts WHERE id = $1
```
- **CASCADE**: deletes related `server_post_comments` and `server_post_likes`

#### PostgreSQL — Delete Post Image Record (inside TX)
```sql
DELETE FROM server_post_images WHERE id = $1
```

#### MinIO — Delete Post Image (after TX commit)
```
DELETE {bucket}/{object_key}
```
- Example: `DELETE {bucket}/server/post/{imageId}.webp`

### Error Cases
- Invalid serverId → `400` with "Invalid server id"
- Invalid postId → `400` with "Invalid post id"
- Not a member → `400` with "You are not a member of this server"
- Not the author → `400` with "You are not the author of this post"
- Post not found (no image) → `400` with "Post not found"

### Flow
```
Request → Auth Middleware → Parse Params → Check Membership (DB) → Check Ownership (DB) → Begin TX → Get Image Info → Delete Post (DB, CASCADE) → Delete Image Record (DB) → Commit TX → Delete Image (MinIO) → Response (no data)
```
