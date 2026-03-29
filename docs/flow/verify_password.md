## Verify Password Flow

### Overview
Completes the multi-step signup process by setting the password, creating the user account, and returning JWT tokens. This is the final step (step 4) of signup.

### Auth
No authentication required.

### Business Logic
1. Parse and validate `sessionId` — must be a valid UUID
2. Validate password is not empty
3. Validate password length (5–20 characters)
4. Get all session data from Redis
5. Verify session exists
6. Verify session step is `username_set` (previous steps must be completed)
7. Final check: verify username and email are still unique in the database
8. If email is already taken at this point, delete the entire session and return error
9. Delete signup session from Redis
10. Delete email→session mapping from Redis
11. Hash password with bcrypt
12. Create user record in database (username, fullname = titleCase(username), email, hashed password)
13. Generate JWT access token (15 minutes expiry)
14. Generate refresh token (UUID)
15. Create refresh token record in PostgreSQL with token family
16. Store access token hash in Redis cache
17. Return token pair to client

### Database Operations

#### Redis — Get All Session Data
```
HGETALL signup:{sessionId}
```
- Returns map with keys: `email`, `step`, `username`, `create_at`, `otp_verified_at`

#### PostgreSQL — Check Username & Email Unique
```sql
SELECT username, email FROM users WHERE username = $1 OR email = $2 LIMIT 1
```
- **Table**: `users`
- **Columns**: `username`, `email`

#### Redis — Delete Session
```
DEL signup:{sessionId}
DEL signup_email:{email}
```

#### PostgreSQL — Create User
```sql
INSERT INTO users (id, username, fullname, bio, avatar_image_id, email, password, settings, create_datetime, update_datetime, create_user_id, update_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
```
- **Table**: `users`
- **Columns**: `id` (UUID), `username`, `fullname` (titleCase of username), `bio` (NULL), `avatar_image_id` (NULL), `email`, `password` (bcrypt hash), `settings` (JSON), timestamps, audit user IDs

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
- Invalid sessionId → `400` with "Invalid session id"
- Password empty → `400` with "Password is required to not be empty"
- Password < 5 chars → `400` with "Password must be at least 5 characters"
- Password > 20 chars → `400` with "Password must be at most 20 characters"
- Session expired/not found → `400` with "Signup session is expired or not exists"
- Wrong step (not username_set) → `400` with "Invalid signup step for this session"
- Username already taken (race condition) → `400` with "Username is already exist"
- Email already taken (race condition) → `400` with "Email is already exist"
- Failed to create refresh token → `500` internal server error
- Failed to store access token → `500` internal server error

### Flow
```
Request → Validate SessionId → Validate Password → Get Session (Redis) → Check Step = username_set → Check Unique (DB) → Delete Session (Redis) → Hash Password → Create User (DB) → Generate Access Token → Generate Refresh Token → Store Refresh Token (PostgreSQL) → Store Access Token (Redis) → Response
```

### Token Info
- **Access Token** — JWT, short-lived (15 minutes), used for API authentication, cached in Redis
- **Refresh Token** — UUID, long-lived (7 days), used to get new access tokens, stored in PostgreSQL
- **Token Family** — UUID group identifier for token rotation security
- **Token Type** — Bearer

### Durations
- **Access Token**: 15 minutes (900 seconds)
- **Refresh Token**: 7 days (604800 seconds)
