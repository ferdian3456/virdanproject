Transfers server ownership to another member. The old owner automatically becomes Admin. The operation is atomic (single database transaction).

**Body:**
```json
{ "newOwnerId": "uuid-of-new-owner" }
```

**Rules:**
- Only the Owner can transfer ownership
- Target must already be a member of this server
- Cannot transfer to yourself

**After transfer:**
- Target becomes Owner (server_members.server_role_id → Owner role, servers.owner_id → target)
- Caller becomes Admin

**Success response:** `{"status": "OK"}`
