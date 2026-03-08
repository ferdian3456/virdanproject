## Join Server Flow

### Overview
Allows an authenticated user to join a public (non-private) server directly without an invite code.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Parse `serverId` from URL path parameter — must be a valid UUID
3. Check if server exists and is eligible to join (not private)
4. Check if user is already a member — return error if already joined
5. Create a "member" role for the user in this server
6. Create server member record linking user to server
7. All operations wrapped in a database transaction
8. Return success with no data

### Error Cases
- Invalid serverId → `400` with "Invalid server id"
- Server not found or private → `400` with "Unable to join server because server is not exists or private"
- Already a member → `400` with "Unable to join server because user is already a member"

### Flow
```
Request → Auth Middleware → Parse ServerId → Check Server Eligible (DB) → Check Not Already Member (DB) → Begin TX → Create Role (DB) → Create Member (DB) → Commit TX → Response
```
