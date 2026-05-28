## Overview

This API is used to unlike a post. Idempotent — if the user has not liked it yet, this request still returns 200 (no error). Returns the latest likeCount + `userLiked: false`.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: DELETE /api/posts/(postId)/likes
    BE->>BE: Middleware extract userId
    BE->>BE: Validate postId (UUID)
    alt UUID invalid
        BE-->>Client: 400 postId is not a valid UUID
    end
    BE->>Postgres: SELECT server_id from post
    alt Post not found
        BE-->>Client: 404 Post not found
    end
    BE->>Postgres: Check server membership
    alt Not a member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: SELECT EXISTS like (post_id, user_id)
    alt Already liked
        BE->>Postgres: DELETE FROM server_post_likes WHERE post_id = $1 AND user_id = $2
    end
    BE->>Postgres: COUNT likes WHERE post_id = $1
    BE-->>Client: 200 {postId, userLiked: false, likeCount}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table                | Column             | Action | Notes                       |
| -------------------- | ------------------ | ------ | --------------------------- |
| `server_posts`       | server_id          | SELECT | Fetch server_id              |
| `server_members`     | (count)            | SELECT | Check membership             |
| `server_post_likes`  | (exists)           | SELECT | Check whether user already liked |
| `server_post_likes`  | post_id, user_id   | DELETE | Delete like (if present)      |
| `server_post_likes`  | (count)            | SELECT | New likeCount                 |

---

## Prerequisites

The user is a member of the server the post belongs to.

---

## Request Validation

Path parameter:

| Field    | Type   | Required | Rules           |
| -------- | ------ | -------- | --------------- |
| `postId` | string | yes      | Required, UUID  |

No body.

---

## Response

### 200 OK

```json
{
  "postId": "post-uuid",
  "userLiked": false,
  "likeCount": 12
}
```

### 400 Bad Request

| `error_message`               | Cause        |
| ----------------------------- | ------------ |
| `postId is not a valid UUID`  | UUID invalid  |

### 403 Forbidden

| `error_message`                          | Cause        |
| ---------------------------------------- | ------------ |
| `You are not a member of this server`    | Not a member  |

### 404 Not Found

| `error_message`     | Cause               |
| ------------------- | ------------------- |
| `Post not found`    | Post not found       |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 23 May 2026.
