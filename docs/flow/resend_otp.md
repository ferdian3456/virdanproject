## Resend OTP Flow

### Overview
Resends a new OTP code to the user's email during the signup process. This endpoint allows users to request a fresh OTP if the previous one has expired or they didn't receive it.

### Auth
No authentication required.

### Business Logic
1. Parse and validate `sessionId` — must be a valid UUID
2. Retrieve email and current OTP expiration from Redis using sessionId
3. Check if session exists — return error if session expired/not found
4. Check if current OTP has not expired yet — return error with remaining wait time
5. Generate new 6-digit OTP code
6. Hash new OTP with SHA-256
7. Set new OTP expiration to 5 minutes from now
8. Send new OTP email using SMTP (HTML template)
9. Update signup session in Redis with new OTP and expiration
10. Return `sessionId` and new `otpExpiresAt` to client

### Database Operations

#### Redis — Get Email and OTP Expiration
```
HMGET signup:{sessionId} email otp_expires_at
```
- Retrieves `email` (string) and `otp_expires_at` (unix timestamp)
- Used to validate session existence and check cooldown period

#### Redis — Update OTP
```
HSET signup:{sessionId}
  otp            {sha256(newOtpCode)}
  otp_expires_at {new_unix_timestamp}
```
- **Note**: Does NOT extend session TTL (session remains at original 30-minute expiry)
- Only updates the OTP hash and expiration timestamp

### Error Cases
- Invalid sessionId format → `400` with "Invalid session id"
- Session expired/not found → `400` with "Signup session is expired or not exists"
- OTP not yet expired (cooldown active) → `400` with formatted wait time:
  - `< 60 seconds`: "Please wait X seconds before requesting another OTP"
  - `≥ 60 seconds`: "Please wait X minutes and Y seconds before requesting another OTP"
- SMTP failure → `500` internal server error

### Flow
```
Request → Validate SessionId → Get Session Data (Redis) → Check OTP Expiration (Cooldown) → Generate New OTP → Send Email (SMTP) → Update OTP in Session (Redis) → Response
```

### Cooldown Logic
The cooldown mechanism uses the OTP expiration timestamp:
- When a new OTP is generated, it expires in 5 minutes
- Users cannot request another OTP until the current one expires
- This prevents spamming of OTP requests while allowing reasonable retry time
- Example:
  - `00:00` — Start signup → OTP expires at `00:05`
  - `00:02` — Resend attempt → Blocked (must wait 3 minutes)
  - `00:06` — Resend attempt → Success (new OTP expires at `00:11`)

### Notes
- Session TTL is not extended during resend — original 30-minute signup window applies
- Email session (`signup_email:{email}`) also not extended (relies on original session TTL)
- Cooldown is enforced via OTP expiry timestamp, not a separate rate limit counter
- Each successful resend generates a completely new OTP code

### Previous Step
User must have called `POST /api/auth/signup/start` to initiate the signup process.

### Next Step
After receiving the new OTP via email, call `POST /api/auth/signup/otp` with the `sessionId` and new OTP code.
