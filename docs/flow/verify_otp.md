## Verify OTP Flow

### Overview
Verifies the OTP code sent to the user's email during signup. This is step 2 of the multi-step signup process.

### Auth
No authentication required.

### Business Logic
1. Parse and validate `sessionId` — must be a valid UUID
2. Validate OTP is not empty and at least 6 characters
3. Retrieve OTP data from Redis using sessionId
4. Check OTP and expiration data exist — return error if session expired
5. Compare provided OTP hash with stored hash using constant-time comparison
6. Check if OTP has expired (compare timestamps)
7. Delete OTP state from session (otpHash, otpExpiresAt)
8. Set verification state: `{step: "otp_verified", verifiedAt: timestamp}`

### Database Operations

#### Redis — Get OTP Data
```
HMGET signup:{sessionId} otp otp_expires_at
```
- Retrieves `otp` (SHA-256 hash) and `otp_expires_at` (unix timestamp)
- If both are `nil`, session has expired or doesn't exist

#### Redis — Delete OTP State
```
HDEL signup:{sessionId} otp otp_expires_at
```

#### Redis — Set Verification State
```
HSET signup:{sessionId}
  step            "otp_verified"
  otp_verified_at {unix_timestamp}
```

### Error Cases
- Invalid sessionId format → `400` with "Invalid session id"
- OTP empty → `400` with "OTP is required to not be empty"
- OTP < 6 chars → `400` with "OTP must be at least 6 characters"
- Session expired or not found → `400` with "OTP does not exists or expired"
- OTP mismatch → `400` with "Otp does not match"
- OTP expired → `400` with "Otp is expired"

### Flow
```
Request → Validate SessionId → Validate OTP → Get Session Data (Redis) → Compare OTP Hash → Check Expiration → Update Session State (Redis) → Response (no data)
```

### Next Step
After OTP verification, call `POST /api/auth/signup/username` with `sessionId` and desired `username`.
