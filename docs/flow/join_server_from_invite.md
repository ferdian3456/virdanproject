## Join Server From Invite Flow

### Overview
Allows an authenticated user to join a server using an invite code. Works for both public and private servers.

### Auth
Requires `Authorization` header with Bearer JWT access token.

### Business Logic
1. Get `userId` from context
2. Validate `inviteCode` (required, exactly 8 characters)
3. Verify invite code exists, is active, not expired, and not fully used — retrieve associated serverId
4. Check if user is already a member
5. Create "member" role for the user
6. Create server member record
7. All operations wrapped in a database transaction

### Error Cases
- Invite code empty → `400` with "Invite code is required to not be empty"
- Invite code ≠ 8 chars → `400` with "Invite code must be 8 characters"
- Invalid/expired/used invite → `400` with "Invite code is not exists, expired or used up"
- Already a member → `400` with "Unable to join server because user is already a member"

### Flow
```
Request → Auth Middleware → Validate InviteCode → Check Invite Valid (DB) → Get ServerId → Check Not Member (DB) → Begin TX → Create Role (DB) → Create Member (DB) → Commit TX → Response (no data)
```
