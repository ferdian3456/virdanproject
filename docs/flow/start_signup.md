## Start Signup Flow

### Overview
Initiates the multi-step signup process by sending an OTP verification email to the provided email address.

### Auth
No authentication required.

### Business Logic
1. Validate email is not empty
2. Validate email length (16–80 characters)
3. Convert email to lowercase
4. Check if email already exists in users table — return error if taken
5. Check if there's an existing signup session for this email
6. If previous session exists, delete old session data (email session + signup session)
7. Generate 6-digit OTP code
8. Hash OTP with SHA-256
9. Create a new session UUID
10. Set OTP expiration to 5 minutes from now
11. Send OTP email using SMTP (HTML template)
12. Store signup session in Redis: `sessionId → {email, otpHash, otpExpiresAt, step: "start_signup"}`
13. Store email→sessionId mapping in Redis (for duplicate detection)
14. Return `sessionId` and `otpExpiresAt` to client

### Database Operations

#### PostgreSQL — Check Email Unique
```sql
SELECT 1 FROM users WHERE email = $1 LIMIT 1
```
- **Table**: `users`
- **Column**: `email`

#### Redis — Check Existing Email Session
```
GET signup_email:{email}
```
- Returns `sessionId` if exists, used to detect and clean up old sessions

#### Redis — Delete Old Session (if exists)
```
DEL signup:{oldSessionId}
DEL signup_email:{email}
```

#### Redis — Create Signup Session
```
HSET signup:{sessionId}
  email          {email}
  otp            {sha256(otpCode)}
  otp_expires_at {unix_timestamp}
  step           "start_signup"
  create_at      {unix_timestamp}
EXPIRE signup:{sessionId} 1800
```
- **TTL**: 30 minutes

#### Redis — Map Email to Session
```
SET signup_email:{email} {sessionId} EX 1800
```
- **TTL**: 30 minutes

### Error Cases
- Email empty → `400` with "Email is required to not be empty"
- Email < 16 chars → `400` with "email must be at least 16 characters"
- Email > 80 chars → `400` with "Email must be at most 80 characters"
- Email already registered → `400` with "Email is already exists"
- SMTP failure → `500` internal server error

### Flow
```
Request → Validate Email → Check Email Unique (DB) → Check Existing Session (Redis) → Delete Old Session → Generate OTP → Send Email (SMTP) → Store Session (Redis) → Response
```

### Next Step
After receiving the OTP via email, call `POST /api/auth/signup/otp` with the `sessionId` and OTP code.
