## Update Server Short Name Flow

### Overview
Updates the short name of a server. Only the server owner can perform this action.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. Validate `shortName` (required, 5–10 characters)
4. Check server ownership
5. Update short name in database
6. Return updated server details

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- ShortName empty → `400` with "Short name is required to not be empty"
- ShortName < 5 chars → `400` with "Short name must be at least 5 characters"
- ShortName > 10 chars → `400` with "Short name must be at most 10 characters"
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Validate ShortName → Check Ownership (DB) → Update (DB) → Get Detail (DB) → Response
```
