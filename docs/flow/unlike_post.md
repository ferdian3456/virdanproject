## Unlike Post Flow

### Overview
Removes a like from a post. Requires membership in the server where the post belongs.

**Idempotent**: If the user hasn't liked the post, the request succeeds and returns the current like count (no error).

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` from URL path — must be a valid UUID
3. Check if user is a member of the server where the post belongs
4. Check if user has liked this post
   - If liked → delete like record
   - If not liked → **skip delete**, proceed to get like count (idempotent)
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
- If no row → hasn't liked yet (skip delete, return like count)

#### PostgreSQL — Delete Like (if already liked)
```sql
DELETE FROM server_post_likes WHERE post_id = $1 AND user_id = $2
```
- **Table**: `server_post_likes`
- Only executed if user has already liked

#### PostgreSQL — Get Updated Like Count
```sql
SELECT COUNT(*) FROM server_post_likes WHERE post_id = $1
```
- **Table**: `server_post_likes`
- Returns current like count after unlike operation

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Not a member → `400` with "You are not a member of this server"

### Flow
```
Request → Auth Middleware → Parse PostId → Check Membership (DB) → Check Already Liked (DB) → [if liked: Delete Like (DB)] → Get Updated Count (DB) → Response
```

### Idempotency
- Calling this endpoint multiple times with the same user and post will:
  - **First call** (when liked): Delete like record, return decremented like count
  - **Subsequent calls**: Skip delete, return current like count (no error)
- This allows for optimistic UI updates on the frontend without worrying about duplicate unlike requests
