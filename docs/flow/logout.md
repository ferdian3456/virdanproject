## Logout Flow

### Overview
Logs out the authenticated user by revoking all refresh tokens in PostgreSQL and removing the access token from Redis cache.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context (set by auth middleware)
2. Revoke all active refresh tokens for this user in PostgreSQL
3. Remove access token from Redis cache
4. Return success with no data

### Database Operations

#### PostgreSQL — Revoke All Refresh Tokens
```sql
UPDATE refresh_tokens
SET revoked_at = $1, updated_at = $2, updated_by = $3
WHERE user_id = $4 AND revoked_at IS NULL
```
- **Table**: `refresh_tokens`
- **Columns updated**: `revoked_at`, `updated_at`, `updated_by`
- **Filter**: `user_id`, `revoked_at IS NULL` (only active tokens)
- **Effect**: All refresh tokens for this user become invalid

#### Redis — Remove Access Token
```
DEL auth:accessToken:{userId}
```
- Deletes the access token hash from Redis
- After deletion, the user's current access token becomes invalid for future requests

### Error Cases
- No/invalid auth token → `404` (handled by auth middleware)
- Failed to revoke refresh tokens → `500` internal server error
- Failed to remove access token → `500` internal server error

### Flow
```
Request → Auth Middleware (JWT) → Get UserId → Revoke Refresh Tokens (PostgreSQL) → Remove Access Token (Redis) → Response (no data)
```

### Security Notes
- All refresh tokens for the user are revoked on logout
- User must login again to get new tokens
- Access token is immediately invalidated
- Refresh token rotation prevents token reuse
