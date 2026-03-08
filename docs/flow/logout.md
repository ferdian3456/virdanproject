## Logout Flow

### Overview
Logs out the authenticated user by removing their auth tokens from Redis cache.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context (set by auth middleware)
2. Remove access token and refresh token from Redis cache
3. Return success with no data

### Database Operations

#### Redis — Remove Auth Tokens
```
DEL auth:acccessToken:{userId}
DEL auth:refreshToken:{userId}
```
- Deletes both the access token hash and refresh token hash from Redis
- After deletion, the user's current JWT becomes invalid for future requests

### Error Cases
- No/invalid auth token → `404` (handled by auth middleware)
- Redis error → `500` internal server error

### Flow
```
Request → Auth Middleware (JWT) → Get UserId → Remove Tokens (Redis) → Response (no data)
```
