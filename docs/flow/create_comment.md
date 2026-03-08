## Create Comment Flow

### Overview
Creates a comment on a post. Requires membership in the server where the post belongs. Supports nested replies via `parentId`.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `postId` from URL path — must be a valid UUID
3. Validate `content` (required, 1–1000 characters)
4. If `parentId` provided, parse as UUID
5. Check if user is a member of the server where the post belongs
6. If `parentId` provided, verify parent comment exists and belongs to the same post
7. Create comment record in database
8. Return created comment details

### Error Cases
- Invalid postId → `400` with "Invalid post id"
- Content empty → `400` with "Content is required"
- Content > 1000 chars → `400` with "Content must be at most 1000 characters"
- Invalid parentId format → `400` with "Invalid parent comment id"
- Not a member → `400` with "You are not a member of this server"
- Parent comment not found → `400` with "Parent comment not found"

### Flow
```
Request → Auth Middleware → Parse PostId → Validate Content → Parse ParentId → Check Membership (DB) → Check Parent Exists (DB) → Create Comment (DB) → Response
```
