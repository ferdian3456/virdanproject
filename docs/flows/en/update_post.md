## Update Post Flow

### Overview
Updates the caption of an existing post. Only the post author and server member can update the post.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` and `postId` from URL path — must be valid UUIDs
3. Parse request body for `caption` (required)
4. Check if user is a member of the server
5. Check if user is the author of the post
6. Update post caption in database
7. Fetch and return updated post details

### Database Operations

#### PostgreSQL — Check Server Member
```sql
SELECT 1 FROM server_members WHERE server_id = $1 AND user_id = $2 AND status = $3
```

#### PostgreSQL — Check Post Ownership
```sql
SELECT 1 FROM server_posts WHERE id = $1 AND author_id = $2
```
- **Table**: `server_posts`
- **Filter**: `id` (post UUID) + `author_id` (user UUID)

#### PostgreSQL — Update Caption
```sql
UPDATE server_posts SET caption = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4
```
- **Table**: `server_posts`
- **Columns updated**: `caption`, `update_datetime`, `update_user_id`

#### PostgreSQL — Get Post (after update)
```sql
SELECT sp.author_id, sp.id, spi.object_key, sp.caption, sp.create_datetime, sp.update_datetime,
       COALESCE(comment_counts.comment_count, 0), COALESCE(like_counts.like_count, 0)
FROM server_posts sp
INNER JOIN server_post_images spi ON sp.post_image_id = spi.id
LEFT JOIN (...) comment_counts ON sp.id = comment_counts.post_id
LEFT JOIN (...) like_counts ON sp.id = like_counts.post_id
WHERE sp.id = $1
```

### Error Cases
- Invalid serverId → `400` with "Invalid server id"
- Invalid postId → `400` with "Invalid post id"
- Invalid request body → `400` with `ERR_INVALID_REQUEST_BODY`
- Caption empty → `400` with "Caption is required"
- Not a member → `400` with "You are not a member of this server"
- Not the author → `400` with "You are not the author of this post"

### Flow
```
Request → Auth Middleware → Parse Params → Parse Body → Check Membership (DB) → Check Ownership (DB) → Update Caption (DB) → Get Post (DB) → Response
```
