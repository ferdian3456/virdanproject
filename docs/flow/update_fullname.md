## Update Fullname Flow

### Overview
Updates the authenticated user's display name (fullname).

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context (set by auth middleware)
2. Parse request body for `fullname` field
3. Validate fullname is not empty
4. Validate fullname length (4–40 characters)
5. Update fullname in database
6. Return success with no data

### Error Cases
- No/invalid auth token → `404` (handled by auth middleware)
- Invalid request body → `400` with `ERR_INVALID_REQUEST_BODY`
- Fullname empty → `400` with "Fullname is required to not be empty"
- Fullname < 4 chars → `400` with "Fullname must be at least 4 characters"
- Fullname > 40 chars → `400` with "Fullname must be at most 40 characters"

### Flow
```
Request → Auth Middleware → Parse Body → Validate Fullname → Update (DB) → Response (no data)
```
