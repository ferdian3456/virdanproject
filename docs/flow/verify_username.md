## Verify Username Flow

### Overview
Sets the username for a signup session. This is step 3 of the multi-step signup process. The session must have completed OTP verification first.

### Auth
No authentication required.

### Business Logic
1. Parse and validate `sessionId` — must be a valid UUID
2. Validate username is not empty
3. Validate username length (4–22 characters)
4. Get current signup session state from Redis
5. Verify session exists and is not expired
6. Verify session step is NOT `start_signup` (OTP must be verified first)
7. Check if username is already taken in the database
8. Store username in session state and update step to `username_set`

### Error Cases
- Invalid sessionId → `400` with "Invalid session id"
- Username empty → `400` with "Username is required to not be empty"
- Username < 4 chars → `400` with "Username must be at least 4 characters"
- Username > 22 chars → `400` with "username must be at most 22 characters"
- Session expired/not found → `400` with "Signup session is expired or not exists"
- OTP not yet verified → `400` with "Invalid signup step for this session"
- Username already taken → `400` with "Username is already taken"

### Flow
```
Request → Validate SessionId → Validate Username → Get Session (Redis) → Check Step ≠ start_signup → Check Username Unique (DB) → Update Session (Redis) → Response (no data)
```

### Next Step
After setting username, call `POST /api/auth/signup/password` with `sessionId` and `password` to complete registration.
