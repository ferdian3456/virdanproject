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
