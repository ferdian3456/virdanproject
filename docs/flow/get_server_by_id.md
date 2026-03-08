## Get Server By ID Flow

### Overview
Retrieves detailed information about a specific server. Access is controlled: public servers are visible to all authenticated users, private servers are only visible to members.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `id` from URL path parameter — must be a valid UUID
3. Query server details with a single query that checks both existence AND membership for private servers
4. If server not found (or private and user is not a member), return error
5. Build full MinIO URLs for avatar and banner images
6. Return server detail

### Error Cases
- Invalid server id → `400` with "Invalid server id"
- Server not found or no permission → `400` with "Server not found or you don't have permission to access it"

### Flow
```
Request → Auth Middleware → Parse ServerId → Query Server (DB, checks visibility + membership) → Build Image URLs → Response
```
