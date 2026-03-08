## Get Post Flow

### Overview
Retrieves a single post by its ID. Requires membership in the server where the post belongs.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` from URL path — must be a valid UUID
3. Check if user is a member of the server where the post belongs (via joined query)
4. Get post details from database
5. Build MinIO image URLs
6. Return post data

### Database Operations

#### PostgreSQL — Check Post Server Membership
```sql
SELECT 1
FROM server_posts sp
INNER JOIN server_members sm ON sp.server_id = sm.server_id
WHERE sp.id = $1 AND sm.user_id = $2 AND sm.status = $3
```
- **Tables**: `server_posts` JOIN `server_members`
- Checks the post exists AND the user is an active member of that post's server

#### PostgreSQL — Get Post
```sql
SELECT sp.author_id, sp.id, spi.object_key, sp.caption, sp.create_datetime, sp.update_datetime,
       COALESCE(comment_counts.comment_count, 0) as comment_count,
       COALESCE(like_counts.like_count, 0) as like_count
FROM server_posts sp
INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
LEFT JOIN (
    SELECT post_id, COUNT(*) as comment_count FROM server_post_comments GROUP BY post_id
) comment_counts ON sp.id = comment_counts.post_id
LEFT JOIN (
    SELECT post_id, COUNT(*) as like_count FROM server_post_likes GROUP BY post_id
) like_counts ON sp.id = like_counts.post_id
WHERE sp.id = $1
```
- **Columns returned**: `author_id`, `id`, image `object_key`, `caption`, timestamps, `comment_count`, `like_count`

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Not a member → `400` with "You are not a member of this server"

### Flow
```
Request → Auth Middleware → Parse PostId → Check Membership (DB) → Get Post (DB) → Build Image URL → Response
```
