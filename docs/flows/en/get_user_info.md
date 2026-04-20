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

### Database Operations

#### PostgreSQL — Get User Info
```sql
SELECT A.id, A.username, A.fullname, A.email, B.object_key, A.bio, A.create_datetime, A.update_datetime
FROM users A
LEFT JOIN user_avatar_images B ON A.id = B.user_id
WHERE A.id = $1
LIMIT 1
```
- **Tables**: `users` (A) LEFT JOIN `user_avatar_images` (B)
- **Join condition**: `users.id = user_avatar_images.user_id`
- **Columns returned**: `id`, `username`, `fullname`, `email`, `object_key` (avatar), `bio`, `create_datetime`, `update_datetime`

#### MinIO — Avatar URL Construction
```
{MINIO_FULL_URL}/{object_key}
```
- If `object_key` is not NULL, full URL is built as: `http(s)://{host}:{port}/{bucket}/{object_key}`
- Object key format: `user/avatar/{imageId}.webp`

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
