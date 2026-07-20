## Overview

This API lists the caller's direct-message conversations within a single server, ordered by most recent activity first. A conversation only appears here once at least one message has been sent in it — a freshly created empty conversation (see `get_or_create_conversation.md`) is not listed until someone sends the first message. Cursor-based pagination.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/servers/(serverId)/conversations?limit&cursor
    BE->>BE: Middleware extract userId
    BE->>BE: Validate serverId (UUID)
    alt UUID invalid
        BE-->>Client: 400 Bad Request
    end
    BE->>Postgres: Check caller is a member of serverId
    alt Not a member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>BE: Clamp limit to [1, 50], default 20; decode cursor (invalid cursor is silently ignored, not rejected)
    BE->>Postgres: SELECT dm_conversation_states WHERE user_id, server_id, last_message_at IS NOT NULL ORDER BY last_message_at DESC LIMIT n+1
    BE->>BE: For each row, look up whether the peer is currently online (in-memory WS hub)
    BE-->>Client: 200 {data: [...], page: {nextCursor}}
```

---

## Notes Redis

Does not use Redis. Online/offline status (`isOnline`) is computed from the in-process WebSocket connection hub (see `websocket_realtime.md`), not from a shared cache — in a multi-instance deployment this only reflects connections held by the instance serving the request.

---

## Notes Postgres/DB

| Table                      | Column                                                    | Action | Notes                                                                  |
| ---------------------------- | -------------------------------------------------------------- | ------ | --------------------------------------------------------------------------- |
| `server_members`           | (count)                                                          | SELECT | Membership check                                                             |
| `dm_conversation_states`   | conversation_id, unread_count, last_message_preview, last_message_at | SELECT | Filtered to `user_id = caller AND server_id AND last_message_at IS NOT NULL`, cursor + limit |
| `server_member_profiles`   | nickname, username                                               | SELECT | Peer's per-server identity (left join)                                       |
| `profile_avatar_images`    | object_key                                                       | SELECT | Peer's avatar (left join)                                                     |

---

## Prerequisites

Caller must be a member of `serverId`.

---

## Request Validation

Path parameter:

| Field      | Type   | Required | Rules           |
| ---------- | ------ | -------- | --------------- |
| `serverId` | string | yes      | Required, UUID  |

Query parameter:

| Field    | Type   | Required | Rules                                                     |
| -------- | ------ | -------- | -------------------------------------------------------------- |
| `limit`  | int    | no       | Defaults to `20`; values `<= 0` fall back to the default, values `> 50` are clamped to `50` |
| `cursor` | string | no       | Opaque cursor from a previous response's `page.nextCursor`; if it fails to decode it is silently ignored (treated as no cursor), not rejected |

---

## Response

### 200 OK

```json
{
  "data": [
    {
      "id": "conversation-uuid",
      "serverId": "server-uuid",
      "peerUserId": "peer-user-uuid",
      "peer": {
        "nickname": "GamerX",
        "username": "gamerx",
        "avatarUrl": null
      },
      "unreadCount": 2,
      "isOnline": true,
      "lastMessagePreview": "see you tomorrow",
      "lastMessageAt": "2026-06-01T10:00:00Z"
    }
  ],
  "page": {
    "nextCursor": "base64-cursor-or-empty"
  }
}
```

| Field                 | Type        | Description                                                     |
| ----------------------- | ----------- | ----------------------------------------------------------------- |
| `unreadCount`          | int         | Caller's unread count for this conversation                        |
| `isOnline`             | bool        | Whether the peer currently has a live WebSocket connection open (this instance only) |
| `lastMessagePreview`   | string/null | Truncated preview of the most recent message                       |

### 400 Bad Request

| `error_message`                | Cause        |
| ------------------------------- | ------------ |
| `serverId is not a valid UUID`  | Invalid UUID |

### 403 Forbidden

| `error_message`                        | Cause        |
| ---------------------------------------- | ------------ |
| `You are not a member of this server`    | Not a member |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 20 July 2026.
