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

### Database Operations

#### PostgreSQL — Check Server Ownership
```sql
SELECT 1 FROM servers WHERE id = $1 AND owner_id = $2
```

#### PostgreSQL — Delete Server
```sql
DELETE FROM servers WHERE id = $1
```
- **Table**: `servers`
- **CASCADE**: Database constraints cascade-delete related records: `server_members`, `server_roles`, `server_invites`, `server_posts` (which cascade-delete `server_post_likes`, `server_post_comments`), `server_avatar_images`, `server_banner_images`

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Ownership (DB) → Delete Server (DB, CASCADE) → Response (no data)
```
