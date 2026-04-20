## Get Server Info For Invite Flow

### Overview
Retrieves public information about a server using an invite code. This is a **public endpoint** (no authentication required) — used to show server preview before joining.

### Auth
No authentication required.

### Business Logic
1. Parse `inviteCode` from URL path parameter
2. Query server info associated with the invite code
3. If no server found for invite code, return error
4. Build full MinIO URLs for avatar and banner images
5. Return server info (owner name, server name, description, images)

### Database Operations

#### PostgreSQL — Get Server Info via Invite Code
```sql
SELECT C.username, A.name, A.description, D.object_key, E.object_key
FROM servers A
INNER JOIN server_invites B ON A.id = B.server_id
INNER JOIN users C ON C.id = A.owner_id
LEFT JOIN server_avatar_images D ON A.avatar_image_id = D.id
LEFT JOIN server_banner_images E ON A.banner_image_id = E.id
WHERE B.code = $1
```
- **Tables**: `servers` (A) INNER JOIN `server_invites` (B), `users` (C), LEFT JOIN `server_avatar_images` (D), `server_banner_images` (E)
- **Filter**: `server_invites.code = {inviteCode}`
- **Columns returned**: `username` (owner), server `name`, `description`, avatar `object_key`, banner `object_key`

#### MinIO — Image URL Construction
```
Avatar: {MINIO_FULL_URL}/{object_key}
Banner: {MINIO_FULL_URL}/{object_key}
```

### Error Cases
- Invite code not found → `400` with "Invite code is not exists"

### Flow
```
Request → Parse InviteCode → Query Server Info (DB) → Build Image URLs (MinIO) → Response
```
