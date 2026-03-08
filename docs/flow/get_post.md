## Get Post Flow

### Overview
Retrieves a single post by ID. Requires membership in the server where the post belongs.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` from URL path — must be a valid UUID
3. Check if user is a member of the server where the post belongs (single query)
4. Fetch post details with author info, image URL, comment count, and like count

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Not a member of the server → `400` with "You are not a member of this server"

### Flow
```
Request → Auth Middleware → Parse PostId → Check Server Membership via Post (DB) → Get Post (DB) → Response
```
