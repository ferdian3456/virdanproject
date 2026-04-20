## Update Bio Flow

### Overview
Updates the authenticated user's bio. Sending an empty string will set the bio to NULL in the database.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context (set by auth middleware)
2. Parse request body for `bio` field
3. If bio is empty string, set to NULL in database; otherwise set to provided value
4. Update bio in database
5. Return success with no data

### Database Operations

#### PostgreSQL — Update Bio
```sql
UPDATE users SET bio = $1, update_datetime = $2, update_user_id = $3 WHERE id = $4
```
- **Table**: `users`
- **Columns updated**: `bio` (nullable TEXT), `update_datetime`, `update_user_id`
- **Filter**: `id = {userId}`
- **Note**: `$1` can be NULL (when bio is empty string) or the bio text

### Error Cases
- No/invalid auth token → `404` (handled by auth middleware)
- Invalid request body → `400` with `ERR_INVALID_REQUEST_BODY`

### Flow
```
Request → Auth Middleware → Parse Body → Update Bio (DB) → Response (no data)
```
