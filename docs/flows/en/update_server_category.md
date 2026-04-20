## Update Server Category Flow

### Overview
Updates the category of a server. Only the server owner can perform this action. Set `categoryId` to `null` to remove the category.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path — must be a valid UUID
3. If `categoryId` is provided (not null), verify the category exists in database
4. Check server ownership
5. Update category in database
6. Return updated server details

### Database Operations

#### PostgreSQL — Check Category Exists (if categoryId not null)
```sql
SELECT 1 FROM server_categories WHERE id = $1
```
- **Table**: `server_categories`

#### PostgreSQL — Check Server Ownership
```sql
SELECT 1 FROM servers WHERE id = $1 AND owner_id = $2
```

#### PostgreSQL — Update Category
```sql
UPDATE servers SET category_id = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4
```
- **Table**: `servers`
- **Columns updated**: `category_id` (nullable INT, FK to `server_categories`), `update_datetime`, `update_user_id`

#### PostgreSQL — Get Server Detail (after update)
```sql
SELECT id, owner_id, name, short_name, category_id, avatar_image_id, banner_image_id, description, settings, create_datetime, update_datetime, create_user_id, update_user_id
FROM servers WHERE id = $1
```

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Category not found → `400` with "Category id is not found"
- Not owner → `400` with "You are not the owner of this server"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Category Exists (DB) → Check Ownership (DB) → Update (DB) → Get Detail (DB) → Response
```
