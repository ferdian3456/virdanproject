## Overview

This API is used to hard-delete a post. Only the author is allowed. FK CASCADE deletes the related comments + likes. The image in MinIO is intentionally left orphan (cleanup job Phase 2).

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: DELETE /api/servers/(serverId)/posts/(postId)
    BE->>BE: Middleware extract userId
    BE->>BE: Validate serverId (UUID), postId (UUID)
    alt Validation Error
        BE-->>Client: 400 e.g.: postId is not a valid UUID
    end
    BE->>Postgres: Check server membership
    alt Not a member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: Check post ownership
    alt Not the author
        BE-->>Client: 403 You are not the author of this post
    end
    BE->>Postgres: DELETE FROM server_posts WHERE id = $1
    note over Postgres: FK CASCADE → server_post_comments, server_post_likes
    BE-->>Client: 200 {status: "OK"}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table            | Column             | Action | Notes                                        |
| ---------------- | ------------------ | ------ | -------------------------------------------- |
| `server_members` | (count)            | SELECT | Check membership                              |
| `server_posts`   | author_id          | SELECT | Check ownership                               |
| `server_posts`   | id                 | DELETE | Hard-delete (FK CASCADE → comments & likes)   |

---

## Prerequisites

User is a member of the server and the author of the post.

---

## Request Validation

Path parameter:

| Field      | Type   | Required | Rules           |
| ---------- | ------ | -------- | --------------- |
| `serverId` | string | yes      | Required, UUID  |
| `postId`   | string | yes      | Required, UUID  |

No body.

---

## Response

### 200 OK

```json
{
  "status": "OK"
}
```

### 400 Bad Request

| `error_message`                | Cause           |
| ------------------------------ | --------------- |
| `serverId is not a valid UUID` | Invalid UUID    |
| `postId is not a valid UUID`   | Invalid UUID    |

### 403 Forbidden

| `error_message`                          | Cause                   |
| ---------------------------------------- | ----------------------- |
| `You are not a member of this server`    | Not a member             |
| `You are not the author of this post`    | Not the author           |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 23 May 2026.
