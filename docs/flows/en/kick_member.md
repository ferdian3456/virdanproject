Removes a member from the server (hard delete of server_members row). The kicked member's past posts remain in the server.

**Permission rules:**
- Owner can kick Admin and Member
- Admin can only kick Member (cannot kick other Admins or Owner)
- Member cannot kick anyone
- Cannot kick yourself (use leave server instead)
- Cannot kick the Owner

**Success response:** `{"status": "OK"}`

**Errors:**
- 400 — trying to kick yourself
- 403 — insufficient permission, or target is Owner
- 404 — target is not a member of this server
