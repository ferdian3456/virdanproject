## Login Flow

### Overview
Authenticates a user with username and password, returns JWT token pair (access + refresh).

### Auth
No authentication required.

### Business Logic
1. Validate request payload — `username` and `password` are required
2. Validate username length (4–22 characters)
3. Validate password length (5–20 characters)
4. Convert username to lowercase
5. Look up user by username in database — return validation error if not found
6. Compare password with bcrypt hash — return validation error if incorrect
7. Generate JWT access token & refresh token
8. Store hashed tokens in Redis cache
9. Return token pair to client

### Database Operations

#### PostgreSQL — Get User Auth
```sql
SELECT id, password FROM users WHERE username = $1 LIMIT 1
```
- **Table**: `users`
- **Columns read**: `id` (UUID), `password` (bcrypt hash)
- **Filter**: `username` (case-insensitive, lowered before query)

#### Redis — Store Auth Tokens
```
SET auth:acccessToken:{userId} {sha256(accessToken)} EX 900
SET auth:refreshToken:{userId} {sha256(refreshToken)} EX 900
```
- **Keys**: `auth:acccessToken:{userId}`, `auth:refreshToken:{userId}`
- **TTL**: 15 minutes (900 seconds)
- **Value**: SHA-256 hash of the raw JWT token

### Error Cases
- Missing/invalid request body → `400` with `ERR_INVALID_REQUEST_BODY`
- Username empty → `400` with "Username is required to not be empty"
- Username < 4 chars → `400` with "Username must be at least 4 characters"
- Username > 22 chars → `400` with "username must be at most 22 characters"
- Password empty → `400` with "Password is required to not be empty"
- Password < 5 chars → `400` with "Password must be at least 5 characters"
- Password > 20 chars → `400` with "Password must be at most 20 characters"
- Username not found in database → `400` validation error
- Password incorrect → `400` with "Password is incorrect"

### Flow
```
Request → Validate Payload → Validate Username → Validate Password → Lookup User (DB) → Compare Password (bcrypt) → Generate JWT Pair → Store Tokens (Redis) → Response
```

### Token Info
- **Access Token** — short-lived, used for authenticating API requests
- **Refresh Token** — long-lived, used to generate new access tokens
- **Token Type** — Bearer