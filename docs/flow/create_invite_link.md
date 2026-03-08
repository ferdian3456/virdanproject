## Create Invite Link Flow

### Overview
Creates a new invite link for a server. The invite code can be shared with others to join the server.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` from URL path — must be a valid UUID
3. Validate `expiresInMinutes` (1–10080, max 1 week)
4. Validate `maxUses` (1–100)
5. Generate unique 8-character invite code (retry up to 10 times for uniqueness)
6. Create invite record in database with code, max uses, expiration time
7. Return invite code and expiration timestamp

### Error Cases
- Invalid request body → `400` with `ERR_INVALID_REQUEST_BODY`
- expiresInMinutes ≤ 0 → `400` with "Expires in minutes must be greater than 0"
- expiresInMinutes > 10080 → `400` with "Expires in minutes must be lower or equal than 10080 or one week"
- maxUses ≤ 0 → `400` with "Max uses must be greater than 0"
- maxUses > 100 → `400` with "Max uses must be lower or equal than 100"

### Flow
```
Request → Auth Middleware → Parse ServerId → Validate Params → Generate Invite Code → Check Uniqueness (DB) → Create Invite (DB) → Response
```
