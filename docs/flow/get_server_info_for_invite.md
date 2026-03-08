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

### Error Cases
- Invite code not found → `400` with "Invite code is not exists"

### Flow
```
Request → Parse InviteCode → Query Server Info (DB) → Build Image URLs (MinIO) → Response
```
