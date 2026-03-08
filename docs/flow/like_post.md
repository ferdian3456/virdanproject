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

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Not a member → `400` with "You are not a member of this server"
- Already liked → `400` with "You already liked this post"

### Flow
```
Request → Auth Middleware → Parse PostId → Check Membership (DB) → Check Not Already Liked (DB) → Create Like (DB) → Get Updated Count (DB) → Response
```
