## Update Server Description Flow

### Overview
Updates the description of a server. Only the server owner can perform this action.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. Check server ownership
4. Update description in database
5. Return updated server details

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Ownership (DB) → Update Description (DB) → Get Detail (DB) → Response
```
