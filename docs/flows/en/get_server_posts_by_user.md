## Overview

This API is used to fetch another member's posts in a server (their profile grid). Same as `get_server_post_for_me` but the target user comes from the path param instead of the token. The requester must be a member of the server. Cursor-based pagination.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/servers/(serverId)/members/(userId)/posts?limit=10&cursor=...
    BE->>BE: Middleware extract requesterUserId
    BE->>BE: Validate serverId & userId (UUID), limit (0-20)
    BE->>Postgres: Check requester membership
    alt Requester not a member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>BE: Decode cursor if provided
    alt Cursor invalid
        BE-->>Client: 400 Invalid cursor
    end
    BE->>Postgres: SELECT posts WHERE server_id = $1 AND author_id = $2 (target) ORDER BY created_at DESC
    BE->>BE: Set isOwner relative to requester; if len > limit build nextCursor
    BE-->>Client: 200 {data, page}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table                    | Column                              | Action | Notes                                    |
| ------------------------ | ----------------------------------- | ------ | ---------------------------------------- |
| `server_members`         | (count)                             | SELECT | Check requester membership               |
| `server_posts`           | server_id, author_id                | SELECT | Filter posts owned by target in server   |
| `server_post_images`     | object_key                          | SELECT | Build imageUrl                           |
| `server_post_likes`      | (count + EXISTS)                    | SELECT | likeCount + userLiked                    |
| `server_post_comments`   | (count)                             | SELECT | commentCount                             |
| `server_member_profiles` | nickname, username, avatar_image_id | SELECT | Author identity per server               |

---

## Prerequisites

The requester is a member of the server.

---

## Request Validation

Path parameter:

| Field      | Type   | Required | Rules          |
| ---------- | ------ | -------- | -------------- |
| `serverId` | string | yes      | Required, UUID |
| `userId`   | string | yes      | Required, UUID |

Query parameters:

| Field    | Type   | Required | Rules                                                |
| -------- | ------ | -------- | ---------------------------------------------------- |
| `limit`  | int    | no       | 0-20, default 10                                     |
| `cursor` | string | no       | Base64 JSON `{id, createdAt}` from the previous page |

---

## Response

### 200 OK

Same format as `get_server_posts`, containing only the target user's posts. The `isOwner` field is `false` (unless the requester is viewing themselves).

```json
{
  "data": [ /* ServerPostResponse */ ],
  "page": {
    "nextCursor": "base64-encoded"
  }
}
```

### 400 Bad Request

| `error_message`                | Cause          |
| ------------------------------ | -------------- |
| `serverId is not a valid UUID` | UUID invalid   |
| `userId is not a valid UUID`   | UUID invalid   |
| `limit must be at most 20`     | Limit > 20     |
| `Invalid cursor`               | Cursor invalid |

### 403 Forbidden

| `error_message`                       | Cause                  |
| ------------------------------------- | ---------------------- |
| `You are not a member of this server` | Requester not a member |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was created on 1 June 2026.
