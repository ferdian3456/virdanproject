Returns the caller's role in a specific server. Used by the client to determine whether the user can see moderation controls (Owner/Admin can delete others' posts, kick members, etc.).

**Response:**
```json
{ "role": "Owner" }
```

Possible `role` values: `"Owner"`, `"Admin"`, `"Member"`. Returns 403 if the caller is not a member.
