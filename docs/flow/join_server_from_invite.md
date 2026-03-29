## Join Server From Invite Flow

### Overview
Allows an authenticated user to join a server using an invite code. Works for both public and private servers.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Validate `inviteCode` (required, exactly 8 characters)
3. Verify invite code exists, is active, not expired, and not fully used — retrieve associated serverId
4. Check if user is already a member — return error if already joined
5. **Check if "Member" role already exists for this server** (outside transaction)
6. If role exists, reuse the role ID; otherwise create a new "Member" role
7. Create server member record linking user to server with the role ID
8. All write operations wrapped in a database transaction
9. Return success with no data

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

#### PostgreSQL — Get Role by Name (outside TX)
```sql
SELECT id FROM server_roles WHERE server_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1
```
- **Table**: `server_roles`
- **Filter**: `server_id`, `LOWER(name) = LOWER('Member')`
- Returns `id` (UUID) or no rows if not found
- **Note**: Executed outside transaction to reduce lock time

#### PostgreSQL — Create Server Role (inside TX, only if not exists)
```sql
INSERT INTO server_roles (id, server_id, name, permissions, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
```
- **Table**: `server_roles`
- **Values**: `name = "Member"`, `permissions = {}`
- **Note**: Only executed if "Member" role doesn't exist for this server

#### PostgreSQL — Create Server Member (inside TX)
```sql
INSERT INTO server_members (id, server_id, user_id, server_role_id, status, joined_datetime, left_datetime, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
```
- **Table**: `server_members`
- **Values**: `status = 1` (active), `left_datetime = NULL`, `server_role_id` = existing or new role ID

### Error Cases
- Invite code empty → `400` with "Invite code is required to not be empty"
- Invite code ≠ 8 chars → `400` with "Invite code must be 8 characters"
- Invalid/expired/used invite → `400` with "Invite code is not exists, expired or used up"
- Already a member → `400` with "Unable to join server because user is already a member"

### Flow
```
Request → Auth Middleware → Validate InviteCode → Check Invite Valid (DB) → Get ServerId → Check Not Member (DB) → Get "Member" Role (DB, outside TX) → Begin TX → Create Role IF NOT EXISTS (DB) → Create Member (DB) → Commit TX → Response (no data)
```

### Important Notes
- **Role Reuse**: The "Member" role is shared across all members of a server. It's created once when the first user joins (via any method), and subsequent users reuse the same role.
- **Unique Constraint**: The `idx_roles_uk_01` unique index on `(server_id, name)` ensures only one "Member" role per server.
- **Optimization**: Checking for existing role outside transaction reduces database lock time and improves concurrency.
