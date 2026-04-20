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

### Database Operations

#### PostgreSQL — Check Invite Code Unique (up to 10 retries)
```sql
SELECT 1 FROM server_invites WHERE code = $1
```
- **Table**: `server_invites`
- Retries with new code if code already exists

#### PostgreSQL — Create Invite
```sql
INSERT INTO server_invites (id, server_id, code, max_uses, used_count, expires_datetime, is_active, create_user_id, update_user_id, create_datetime, update_datetime)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
```
- **Table**: `server_invites`
- **Columns**: `id` (UUID), `server_id`, `code` (8-char random string), `max_uses`, `used_count` (0 initially), `expires_datetime` (now + expiresInMinutes), `is_active` (true), timestamps, audit IDs

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
