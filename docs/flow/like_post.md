## Like Post Flow

### Overview
Likes a post. Requires membership in the server where the post belongs. Each user can only like a post once.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` from URL path — must be a valid UUID
3. Check if user is a member of the server where the post belongs
4. Check if user already liked this post — return error if already liked
5. Create like record in database
6. Fetch updated post to get new like count
7. Return updated like count

### Database Operations

#### PostgreSQL — Check Post Server Membership
```sql
SELECT 1
FROM server_posts sp
INNER JOIN server_members sm ON sp.server_id = sm.server_id
WHERE sp.id = $1 AND sm.user_id = $2 AND sm.status = $3
```

#### PostgreSQL — Check Already Liked
```sql
SELECT 1 FROM server_post_likes WHERE post_id = $1 AND user_id = $2
```
- **Table**: `server_post_likes`
- If row exists → already liked

#### PostgreSQL — Create Like
```sql
INSERT INTO server_post_likes (id, post_id, user_id, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
```
- **Table**: `server_post_likes`
- **Columns**: `id` (UUID), `post_id`, `user_id`, timestamps, audit IDs

#### PostgreSQL — Get Updated Like Count (via GetPost)
Uses the same aggregated query as Get Post to fetch the updated like count.

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Not a member → `400` with "You are not a member of this server"
- Already liked → `400` with "You already liked this post"

### Flow
```
Request → Auth Middleware → Parse PostId → Check Membership (DB) → Check Not Already Liked (DB) → Create Like (DB) → Get Updated Count (DB) → Response
```
