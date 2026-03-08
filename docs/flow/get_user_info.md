## Get User Info Flow

### Overview
Retrieves the authenticated user's profile information including avatar URL.

### Auth
Requires `Authorization` header with Bearer JWT access token. The `userId` is extracted from the token by the auth middleware.

### Business Logic
1. Get `userId` from context (set by auth middleware)
2. Query user info from database by userId
3. If user has an avatar image, construct the full MinIO URL for the avatar
4. Return user profile data

### Error Cases
- No/invalid auth token → `404` (handled by auth middleware)
- Token expired → `404` with "Authorization token is expired"
- User not found → `404` validation error

### Flow
```
Request → Auth Middleware (JWT) → Get UserId → Query User (DB) → Build Avatar URL (MinIO) → Response
```

### Response Fields
- `id` — user UUID
- `username` — unique username
- `fullname` — display name
- `email` — user email
- `avatarImage` — full URL to avatar image (nullable)
- `bio` — user bio (nullable)
- `createDatetime` — account creation timestamp
- `updateDatetime` — last update timestamp
