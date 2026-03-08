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
