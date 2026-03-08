## Create Server Flow

### Overview
Creates a new server (community). The creator automatically becomes the owner with full permissions. A server role and server member record are also created in a transaction.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse and validate request body
3. Validate `name` (required, 5–40 characters)
4. Validate `shortName` (required, 5–10 characters)
5. If `categoryId` is provided, verify category exists in database
6. Create server record with JSON settings (`isPrivate`)
7. Create owner role with full permissions (`{"*": true}`)
8. Create server member record linking user to server with owner role
9. All operations wrapped in a database transaction
10. Return created server details

### Database Operations

#### PostgreSQL — Check Category Exists (if categoryId provided)
```sql
SELECT 1 FROM server_categories WHERE id = $1
```
- **Table**: `server_categories`
- **Column**: `id`

#### PostgreSQL — Create Server (inside TX)
```sql
INSERT INTO servers (id, owner_id, name, short_name, category_id, avatar_image_id, banner_image_id, description, settings, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
```
- **Table**: `servers`
- **Key columns**: `id` (UUID), `owner_id`, `name`, `short_name`, `category_id` (nullable), `avatar_image_id` (NULL initially), `banner_image_id` (NULL initially), `description` (nullable), `settings` (JSONB, e.g., `{"isPrivate": false}`)

#### PostgreSQL — Create Server Role (inside TX)
```sql
INSERT INTO server_roles (id, server_id, name, permissions, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
```
- **Table**: `server_roles`
- **Key columns**: `id` (UUID), `server_id`, `name` ("owner"), `permissions` (JSONB, `{"*": true}`)

#### PostgreSQL — Create Server Member (inside TX)
```sql
INSERT INTO server_members (id, server_id, user_id, server_role_id, status, joined_datetime, left_datetime, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
```
- **Table**: `server_members`
- **Key columns**: `id` (UUID), `server_id`, `user_id`, `server_role_id`, `status` (1 = active), `joined_datetime`, `left_datetime` (NULL)

### Error Cases
- Name empty → `400` with "Name is required to not be empty"
- Name < 5 chars → `400` with "Name must be at least 4 characters"
- Name > 40 chars → `400` with "Name must be at most 40 characters"
- ShortName empty → `400` with "Short name is required to not be empty"
- ShortName < 5 chars → `400` with "Short name must be at least 5 characters"
- ShortName > 10 chars → `400` with "Short name must be at most 10 characters"
- Invalid categoryId → `400` with "Category id is not found"

### Flow
```
Request → Auth Middleware → Validate Body → Check Category (DB) → Begin TX → Create Server (DB) → Create Role (DB) → Create Member (DB) → Commit TX → Response
```
