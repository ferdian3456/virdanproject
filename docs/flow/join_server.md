## Join Server Flow

### Overview
Allows an authenticated user to join a public (non-private) server directly without an invite code.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` from URL path parameter — must be a valid UUID
3. Check if server exists and is eligible to join (not private)
4. Check if user is already a member — return error if already joined
5. Create a "member" role for the user in this server
6. Create server member record linking user to server
7. All operations wrapped in a database transaction
8. Return success with no data

### Database Operations

#### PostgreSQL — Check Server Eligible (public)
```sql
SELECT 1 FROM servers WHERE id = $1 AND (settings->>'isPrivate')::boolean = false
```
- **Table**: `servers`
- **Filter**: `id` + `settings->>'isPrivate' = false`

#### PostgreSQL — Check Server Member
```sql
SELECT 1 FROM server_members WHERE server_id = $1 AND user_id = $2 AND status = 1
```
- **Table**: `server_members`
- **Filter**: `server_id`, `user_id`, `status = 1` (active)

#### PostgreSQL — Create Server Role (inside TX)
```sql
INSERT INTO server_roles (id, server_id, name, permissions, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
```
- **Table**: `server_roles`
- **Values**: `name` = "member", `permissions` = `{}`

#### PostgreSQL — Create Server Member (inside TX)
```sql
INSERT INTO server_members (id, server_id, user_id, server_role_id, status, joined_datetime, left_datetime, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
```
- **Table**: `server_members`
- **Values**: `status = 1` (active), `left_datetime = NULL`

### Error Cases
- Invalid serverId → `400` with "Invalid server id"
- Server not found or private → `400` with "Unable to join server because server is not exists or private"
- Already a member → `400` with "Unable to join server because user is already a member"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Server Eligible (DB) → Check Not Already Member (DB) → Begin TX → Create Role (DB) → Create Member (DB) → Commit TX → Response
```
