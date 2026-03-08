## Delete Comment Flow

### Overview
Deletes a comment from a post. Only the comment author can delete their own comment. Requires membership in the server where the post belongs.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` and `commentId` from URL path — must be valid UUIDs
3. Check if user is a member of the server where the post belongs
4. Check if user is the author of the comment
5. Delete comment from database

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Invalid commentId → `400` with "Invalid comment id"
- Not a member → `400` with "You are not a member of this server"
- Not the author → `400` with "You are not the author of this comment"

### Flow
```
Request → Auth Middleware → Parse Params → Check Membership (DB) → Check Comment Ownership (DB) → Delete Comment (DB) → Response (no data)
```
