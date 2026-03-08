## Get Signup Status Flow

### Overview
Retrieves the current step/status of an active signup session.

### Auth
No authentication required.

### Business Logic
1. Parse `sessionId` from URL path parameter — must be a valid UUID
2. Get session state from Redis
3. Check if session exists — return error if expired/not found
4. Return session ID and current step

### Possible Steps
- `start_signup` — email submitted, OTP sent, waiting for OTP verification
- `otp_verified` — OTP verified, waiting for username
- `username_set` — username set, waiting for password

### Error Cases
- Invalid sessionId format → `400` with "Invalid session id"
- Session expired/not found → `400` with "Signup session is expired or not exists"

### Flow
```
Request → Parse SessionId → Get Session State (Redis) → Return Step
```
