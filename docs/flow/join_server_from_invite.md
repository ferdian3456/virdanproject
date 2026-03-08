## Join Server From Invite Flow

### Overview
Allows an authenticated user to join a server using an invite code. Works for both public and private servers.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Validate `inviteCode` (required, exactly 8 characters)
3. Verify invite code exists, is active, not expired, and not fully used — retrieve associated serverId
4. Check if user is already a member
5. Create "member" role for the user
6. Create server member record
7. All operations wrapped in a database transaction

### Database Operations

#### PostgreSQL — Check Invite Code & Get Server ID
```sql
SELECT server_id FROM server_invites WHERE code = $1 AND is_active = true AND used_count < max_uses
```
- **Table**: `server_invites`
- **Filter**: `code` match + `is_active = true` + `used_count < max_uses`
- Returns `server_id` (UUID) or no rows if invalid/expired/used up

#### PostgreSQL — Check Server Member
```sql
SELECT 1 FROM server_members WHERE server_id = $1 AND user_id = $2 AND status = 1
```
- **Table**: `server_members`
- Checks if user is already an active member

#### PostgreSQL — Create Server Role (inside TX)
```sql
INSERT INTO server_roles (id, server_id, name, permissions, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
```
- **Values**: `name = "member"`, `permissions = {}`

#### PostgreSQL — Create Server Member (inside TX)
```sql
INSERT INTO server_members (id, server_id, user_id, server_role_id, status, joined_datetime, left_datetime, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
```
- **Values**: `status = 1` (active), `left_datetime = NULL`

### Error Cases
- Invite code empty → `400` with "Invite code is required to not be empty"
- Invite code ≠ 8 chars → `400` with "Invite code must be 8 characters"
- Invalid/expired/used invite → `400` with "Invite code is not exists, expired or used up"
- Already a member → `400` with "Unable to join server because user is already a member"

### Flow
```
Request → Auth Middleware → Validate InviteCode → Check Invite Valid (DB) → Get ServerId → Check Not Member (DB) → Begin TX → Create Role (DB) → Create Member (DB) → Commit TX → Response (no data)
```
