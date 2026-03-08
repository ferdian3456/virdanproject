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

### Database Operations

#### PostgreSQL — Check Server Ownership
```sql
SELECT 1 FROM servers WHERE id = $1 AND owner_id = $2
```

#### PostgreSQL — Update Description
```sql
UPDATE servers SET description = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4
```
- **Table**: `servers`
- **Columns updated**: `description` (nullable TEXT), `update_datetime`, `update_user_id`

#### PostgreSQL — Get Server Detail (after update)
```sql
SELECT id, owner_id, name, short_name, category_id, avatar_image_id, banner_image_id, description, settings, create_datetime, update_datetime, create_user_id, update_user_id
FROM servers WHERE id = $1
```

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Ownership (DB) → Update Description (DB) → Get Detail (DB) → Response
```
