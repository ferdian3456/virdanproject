## Delete Server Flow

### Overview
Deletes a server permanently. Only the server owner can perform this action.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. Check server ownership
4. Delete server from database (CASCADE deletes related records)

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Ownership (DB) → Delete Server (DB, CASCADE) → Response (no data)
```
