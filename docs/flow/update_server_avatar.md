## Update Server Avatar Flow

### Overview
Updates the avatar image of a server. Only the server owner can perform this action. Accepts multipart form file upload.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. Check server ownership
4. Read `avatar` file from multipart form
5. Validate image file (format, size)
6. Begin database transaction
7. Get current server detail to find old avatar
8. If new avatar provided: create new image record → update server reference → upload to MinIO → delete old image if exists
9. If no new avatar (empty file): update server to remove reference → delete old image if exists
10. Commit transaction

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Not owner → `400` with "You are not the owner of this server"
- Invalid image → `400` validation error

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Ownership (DB) → Read File → Validate Image → Begin TX → Handle Old/New Avatar → Commit TX → Response (no data)
```
