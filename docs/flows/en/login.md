## Login Flow

### Overview
Authenticates a user with username and password, returns JWT token pair (access + refresh). Access token is cached in Redis, refresh token is stored in PostgreSQL with token rotation support.

### Auth
No authentication required.

### Business Logic
1. Validate request payload — `username` and `password` are required
2. Validate username length (4–22 characters)
3. Validate password length (5–20 characters)
4. Convert username to lowercase
5. Look up user by username in database — return validation error if not found
6. Compare password with bcrypt hash — return validation error if incorrect
7. Generate JWT access token (15 minutes expiry)
8. Generate refresh token (UUID)
9. Create refresh token record in PostgreSQL with token family
10. Store access token hash in Redis cache
11. Return token pair to client

### Database Operations

#### PostgreSQL — Get User Auth
```sql
SELECT id, password FROM users WHERE username = $1 LIMIT 1
```
- **Table**: `users`
- **Columns read**: `id` (UUID), `password` (bcrypt hash)
- **Filter**: `username` (case-insensitive, lowered before query)

#### PostgreSQL — Create Refresh Token
```sql
INSERT INTO refresh_tokens
(id, user_id, token_hash, token_family, expires_at, created_at, updated_at, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
```
- **Table**: `refresh_tokens`
- **Columns**: `id` (UUID), `user_id`, `token_hash` (SHA-256), `token_family` (UUID), `expires_at` (7 days), timestamps, audit
- **Note**: `created_by` and `updated_by` use the same value (user ID)

#### Redis — Store Access Token
```
SET auth:accessToken:{userId} {sha256(accessToken)} EX 900
```
- **Key**: `auth:accessToken:{userId}`
- **TTL**: 15 minutes (900 seconds)
- **Value**: SHA-256 hash of the access token

### Error Cases
- Missing/invalid request body → `400` with `ERR_INVALID_REQUEST_BODY`
- Username empty → `400` with "Username is required to not be empty"
- Username < 4 chars → `400` with "Username must be at least 4 characters"
- Username > 22 chars → `400` with "Username must be at most 22 characters"
- Password empty → `400` with "Password is required to not be empty"
- Password < 5 chars → `400` with "Password must be at least 5 characters"
- Password > 20 chars → `400` with "Password must be at most 20 characters"
- Username not found in database → `400` validation error
- Password incorrect → `400` with "Password is incorrect"
- Failed to create refresh token → `500` internal server error
- Failed to store access token → `500` internal server error

### Flow
```
Request → Validate Payload → Validate Username → Validate Password → Lookup User (DB) → Compare Password (bcrypt) → Generate Access Token → Generate Refresh Token → Store Refresh Token (PostgreSQL) → Store Access Token (Redis) → Response
```

### Token Info
- **Access Token** — JWT, short-lived (15 minutes), used for API authentication, cached in Redis
- **Refresh Token** — UUID, long-lived (7 days), used to get new access tokens, stored in PostgreSQL
- **Token Family** — UUID group identifier for token rotation security
- **Token Type** — Bearer

### Durations
- **Access Token**: 15 minutes (900 seconds)
- **Refresh Token**: 7 days (604800 seconds)
