Returns the list of server members with their roles. Accessible only to existing members (prevents enumeration of private server rosters). Results are sorted by join time (oldest first) with cursor pagination.

**Who can access:** Any server member (Owner, Admin, Member).

**Query parameters:**
- `limit` — items per page (default 10, max 20)
- `cursor` — cursor for the next page (from previous response)

**Response:**
```json
{
  "data": [
    {
      "userId": "uuid",
      "role": "Owner",
      "nickname": "GamerX",
      "username": "johndoe",
      "avatarUrl": "http://...",
      "joinedAt": "2024-01-15T10:30:00Z"
    }
  ],
  "page": {
    "nextCursor": "base64-encoded-cursor"
  }
}
```
