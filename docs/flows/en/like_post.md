## Like Post Flow

### Overview
Likes a post. Requires membership in the server where the post belongs. Each user can only like a post once.

**Idempotent**: If the user has already liked the post, the request succeeds and returns the current like count (no error).

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` from URL path — must be a valid UUID
3. Check if user is a member of the server where the post belongs
4. Check if user already liked this post
   - If already liked → **skip insert**, proceed to get like count (idempotent)
   - If not liked → create like record
5. Get updated like count
6. Return updated like count

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
- If row exists → already liked (skip insert, return like count)

#### PostgreSQL — Create Like (if not already liked)
```sql
INSERT INTO server_post_likes (id, post_id, user_id, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
```
- **Table**: `server_post_likes`
- **Columns**: `id` (UUID), `post_id`, `user_id`, timestamps, audit IDs
- Only executed if user hasn't liked yet

#### PostgreSQL — Get Updated Like Count
```sql
SELECT COUNT(*) FROM server_post_likes WHERE post_id = $1
```
- **Table**: `server_post_likes`
- Returns current like count after like operation

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Not a member → `400` with "You are not a member of this server"

### Flow
```
Request → Auth Middleware → Parse PostId → Check Membership (DB) → Check Already Liked (DB) → [if not liked: Create Like (DB)] → Get Updated Count (DB) → Response
```

### Idempotency
- Calling this endpoint multiple times with the same user and post will:
  - **First call**: Create like record, return incremented like count
  - **Subsequent calls**: Skip insert, return current like count (no error)
- This allows for optimistic UI updates on the frontend without worrying about duplicate like requests
