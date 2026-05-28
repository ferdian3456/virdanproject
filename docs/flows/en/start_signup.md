## Overview

This API is used to start the signup process by sending an OTP to the provided email. A session will be created in Redis and the email is sent via SMTP.

---

## Auth

This is a public API, so no authorization header is required.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant Redis
    participant SMTP

    Client->>BE: POST /api/auth/signup/start
    BE->>BE: Validate email (required, min 5, max 255, email format)
    alt Validation Error
        BE-->>Client: Returns a response, e.g.: email is required
    end
    BE->>BE: Normalize email to lowercase
    BE->>Postgres: SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL
    alt Email already registered
        BE-->>Client: 409 Email is already registered
    end
    BE->>Redis: GET signup_email:(email)
    alt There is an active session
        BE->>Redis: DEL signup:(prevSessionId)
        BE->>Redis: DEL signup_email:(email)
    end
    BE->>BE: Generate 6-digit OTP & SHA256 hash
    BE->>SMTP: Send OTP to the user's email (template otp.html)
    SMTP-->>Client: OTP email arrives in the inbox
    BE->>Redis: HSET signup:(sessionId) {email, otp, otp_expires_at, step, created_at}
    BE->>Redis: EXPIRE signup:(sessionId) 30 minutes
    BE->>Redis: SET signup_email:(email) = sessionId, EX 30 minutes
    BE-->>Client: 200 {sessionId, otpExpiresAt}
```

---

## Notes Redis

1. signup session:
   key: `signup:(sessionId)`
   type: HASH
   ttl: 30 minutes
   fields:
   - `email`
   - `otp` (SHA256 hash of the OTP)
   - `otp_expires_at` (unix timestamp, 5 minutes from now)
   - `step` = `start_signup`
   - `created_at` (unix timestamp)

2. signup email session:
   key: `signup_email:(email)`
   value: sessionId
   ttl: 30 minutes

---

## Notes Postgres/DB

| Table   | Column | Action | Notes                                                     |
| ------- | ------ | ------ | --------------------------------------------------------- |
| `users` | email  | SELECT | Check email is unique (filter `deleted_at IS NULL`) before sending the OTP |

---

## Prerequisites

None. The endpoint can be called under any condition. If there is an active signup session for the same email, the old session will be deleted and replaced with a new session.

---

## Request Validation

| Field   | Type   | Required | Rules                                                              |
| ------- | ------ | -------- | ------------------------------------------------------------------- |
| `email` | string | yes      | Required, min 5 characters, max 255 characters, must be a valid email format |

The email is automatically lowercased after validation.

---

## Response

### 200 OK

```json
{
  "sessionId": "550e8400-e29b-41d4-a716-446655440000",
  "otpExpiresAt": 1714829400
}
```

| Field          | Type   | Description                                          |
| -------------- | ------ | ---------------------------------------------------- |
| `sessionId`    | string | UUID session to continue to verify OTP               |
| `otpExpiresAt` | int64  | Unix timestamp OTP expiry (5 minutes from now)       |

### 400 Bad Request

| `error_message`                       | Cause                         |
| ------------------------------------- | ----------------------------- |
| `email is required`                   | Email not filled in           |
| `email must be at least 5 characters` | Email less than 5 characters  |
| `email must be at most 255 characters`| Email more than 255 characters |
| `email must be a valid email address` | Invalid email format          |

### 409 Conflict

| `error_message`                | Cause                                 |
| ------------------------------ | ------------------------------------- |
| `Email is already registered`  | Email already registered in the users table |

---

## Update

This documentation was last updated on 23 May 2026.
