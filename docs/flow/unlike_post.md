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

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Not a member → `400` with "You are not a member of this server"
- Not liked yet → `400` with "You haven't liked this post yet"

### Flow
```
Request → Auth Middleware → Parse PostId → Check Membership (DB) → Check Already Liked (DB) → Delete Like (DB) → Get Updated Count (DB) → Response
```
