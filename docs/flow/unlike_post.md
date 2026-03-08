## Unlike Post Flow

### Overview
Removes a like from a post. Requires membership in the server where the post belongs. User must have previously liked the post.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` from URL path — must be a valid UUID
3. Check if user is a member of the server where the post belongs
4. Check if user has liked this post — return error if not liked
5. Delete like record from database
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
- If no row → hasn't liked yet, return error

#### PostgreSQL — Delete Like
```sql
DELETE FROM server_post_likes WHERE post_id = $1 AND user_id = $2
```
- **Table**: `server_post_likes`

#### PostgreSQL — Get Updated Like Count (via GetPost)
Uses the same aggregated query as Get Post.

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Not a member → `400` with "You are not a member of this server"
- Not liked yet → `400` with "You haven't liked this post yet"

### Flow
```
Request → Auth Middleware → Parse PostId → Check Membership (DB) → Check Already Liked (DB) → Delete Like (DB) → Get Updated Count (DB) → Response
```
