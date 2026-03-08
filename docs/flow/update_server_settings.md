## Update Server Settings Flow

### Overview
Updates the settings of a server (e.g., privacy settings). Only the server owner can perform this action.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. Parse request body for settings
4. Check server ownership
5. Marshal settings to JSON
6. Update settings in database

### Settings Fields
- `isPrivate` — boolean, whether the server is private (only joinable via invite)

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Invalid request body → `400` with `ERR_INVALID_REQUEST_BODY`
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Parse Body → Check Ownership (DB) → Update Settings (DB) → Response (no data)
```
