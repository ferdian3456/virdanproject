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
