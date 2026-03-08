## Get Discovery Server Flow

### Overview
Retrieves a paginated list of discoverable (public) servers. Supports cursor-based pagination and optional filtering by category.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse query parameters: `limit` (default: system default), `categoryId` (default: 0 = all), `cursor` (optional)
3. Validate `limit` (>= 0 and <= max limit)
4. If cursor provided, decode from base64 URL-safe encoding and unmarshal to `ServerDiscoveryCursor`
5. Query discoverable servers from database (limit + 1 for cursor detection)
6. Build MinIO image URLs for avatar and banner
7. If results > limit, create next cursor from last item's ID and createDatetime
8. Return paginated server list

### Database Operations

#### PostgreSQL — Get Discovery Servers (first page)
```sql
SELECT A.id, A.name, A.short_name, B.name, C.object_key, D.object_key, A.description, A.create_datetime
FROM servers A
LEFT JOIN server_categories B ON A.category_id = B.id
LEFT JOIN server_avatar_images C ON A.avatar_image_id = C.id
LEFT JOIN server_banner_images D ON A.banner_image_id = D.id
WHERE ($1::int IS NULL OR B.id = $1)
AND (A.settings->>'isPrivate')::boolean = false
ORDER BY A.create_datetime DESC, A.id DESC
LIMIT $2
```

#### PostgreSQL — Get Discovery Servers (with cursor)
```sql
SELECT A.id, A.name, A.short_name, B.name, C.object_key, D.object_key, A.description, A.create_datetime
FROM servers A
LEFT JOIN server_categories B ON A.category_id = B.id
LEFT JOIN server_avatar_images C ON A.avatar_image_id = C.id
LEFT JOIN server_banner_images D ON A.banner_image_id = D.id
WHERE (A.create_datetime < $1 OR (A.create_datetime = $1 AND A.id < $2))
AND ($3::int IS NULL OR B.id = $3)
AND (A.settings->>'isPrivate')::boolean = false
ORDER BY A.create_datetime DESC, A.id DESC
LIMIT $4
```
- **Tables**: `servers` (A) LEFT JOIN `server_categories` (B), `server_avatar_images` (C), `server_banner_images` (D)
- **Filter**: Only public servers (`settings->>'isPrivate' = false`)
- **Cursor**: keyset pagination on `(create_datetime, id)` DESC
- **Category filter**: Optional, filters by `server_categories.id`

#### MinIO — Image URL Construction
```
{MINIO_FULL_URL}/{object_key}
```
- Avatar: `{MINIO_FULL_URL}/server/avatar/{imageId}.webp`
- Banner: `{MINIO_FULL_URL}/server/banner/{imageId}.webp`

### Query Parameters
- `limit` — number of items per page (optional, has default)
- `categoryId` — filter by category (optional, 0 = all categories)
- `cursor` — base64-encoded cursor for next page (optional)

### Error Cases
- Limit < 0 → `400` with "Limit must be greater or equal than 0"
- Limit > max → `400` with "Limit is exceeded max limit: {max}"

### Flow
```
Request → Auth Middleware → Parse Query Params → Validate Limit → Decode Cursor → Query Servers (DB) → Build Image URLs → Build Next Cursor → Response
```
