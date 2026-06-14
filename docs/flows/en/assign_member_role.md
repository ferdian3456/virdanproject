Changes a member's role between Admin and Member. Only the Owner can do this.

**Body:**
```json
{ "role": "Admin" }
```
Valid values: `"Admin"` or `"Member"`. Cannot assign `"Owner"` via this endpoint (use transfer_ownership instead).

**Rules:**
- Only the Owner can assign roles
- Cannot change your own role
- Target must already be a member of this server

**Success response:** `{"status": "OK"}`
