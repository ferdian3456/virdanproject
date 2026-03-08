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
13. Generate JWT access token & refresh token
14. Store hashed tokens in Redis cache
15. Return token pair to client

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

#### Redis — Store Auth Tokens
```
SET auth:acccessToken:{userId} {sha256(accessToken)} EX 900
SET auth:refreshToken:{userId} {sha256(refreshToken)} EX 900
```
- **TTL**: 15 minutes

### Error Cases
- Invalid sessionId → `400` with "Invalid session id"
- Password empty → `400` with "Password is required to not be empty"
- Password < 5 chars → `400` with "Password must be at least 5 characters"
- Password > 20 chars → `400` with "Password must be at most 20 characters"
- Session expired/not found → `400` with "Signup session is expired or not exists"
- Wrong step (not username_set) → `400` with "Invalid signup step for this session"
- Username already taken (race condition) → `400` with "Username is already exist"
- Email already taken (race condition) → `400` with "Email is already exist"

### Flow
```
Request → Validate SessionId → Validate Password → Get Session (Redis) → Check Step = username_set → Check Unique (DB) → Delete Session (Redis) → Hash Password → Create User (DB) → Generate JWT → Store Tokens (Redis) → Response
```
