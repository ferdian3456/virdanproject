## Update Server Name Flow

### Overview
Updates the name of a server. Only the server owner can perform this action.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path parameter — must be a valid UUID
3. Validate `name` (required, 5–40 characters)
4. Check server ownership — only the owner can update
5. Update server name in database
6. Fetch and return updated server details

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Name empty → `400` with "Name is required to not be empty"
- Name < 5 chars → `400` with "Name must be at least 4 characters"
- Name > 40 chars → `400` with "Name must be at most 40 characters"
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Validate Name → Check Ownership (DB) → Update Name (DB) → Get Server Detail (DB) → Response
```
