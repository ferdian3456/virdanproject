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

### Database Operations

#### PostgreSQL — Check Server Ownership
```sql
SELECT 1 FROM servers WHERE id = $1 AND owner_id = $2
```
- **Table**: `servers`
- **Filter**: `id` + `owner_id`

#### PostgreSQL — Update Server Name
```sql
UPDATE servers SET name = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4
```
- **Table**: `servers`
- **Columns updated**: `name`, `update_datetime`, `update_user_id`

#### PostgreSQL — Get Server Detail (after update)
```sql
SELECT id, owner_id, name, short_name, category_id, avatar_image_id, banner_image_id, description, settings, create_datetime, update_datetime, create_user_id, update_user_id
FROM servers WHERE id = $1
```
- **Table**: `servers`
- Returns full server detail for response

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
