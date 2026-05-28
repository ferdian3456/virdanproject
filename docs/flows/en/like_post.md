## Overview

This API is used to like a post. Idempotent — if the user has already liked it before, the second request does not error (uses `ON CONFLICT (post_id, user_id) DO NOTHING`). Returns the latest likeCount + `userLiked: true`.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: POST /api/posts/(postId)/likes
    BE->>BE: Middleware extract userId
    BE->>BE: Validate postId (UUID)
    alt UUID invalid
        BE-->>Client: 400 postId is not a valid UUID
    end
    BE->>Postgres: SELECT server_id FROM server_posts WHERE id = $1
    alt Post does not exist
        BE-->>Client: 404 Post not found
    end
    BE->>Postgres: Check server membership
    alt Not a member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: INSERT INTO server_post_likes ... ON CONFLICT (post_id, user_id) DO NOTHING
    BE->>Postgres: COUNT likes WHERE post_id = $1
    BE-->>Client: 200 {postId, userLiked: true, likeCount}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table                | Column                                 | Action | Notes                                               |
| -------------------- | -------------------------------------- | ------ | --------------------------------------------------- |
| `server_posts`       | server_id                              | SELECT | Fetch server_id for the membership check            |
| `server_members`     | (count)                                | SELECT | Check membership                                    |
| `server_post_likes`  | id, post_id, user_id, created_at, ... | INSERT | Idempotent with `ON CONFLICT (post_id, user_id) DO NOTHING` |
| `server_post_likes`  | (count)                                | SELECT | New likeCount                                       |

---

## Prerequisites

User is a member of the server where the post resides.

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
  "userLiked": true,
  "likeCount": 13
}
```

| Field       | Type   | Description                                     |
| ----------- | ------ | ----------------------------------------------- |
| `postId`    | string | Post UUID                                       |
| `userLiked` | bool   | Always `true` after this endpoint succeeds      |
| `likeCount` | int    | Latest total likes                              |

### 400 Bad Request

| `error_message`               | Cause        |
| ----------------------------- | ------------ |
| `postId is not a valid UUID`  | Invalid UUID |

### 403 Forbidden

| `error_message`                          | Cause        |
| ---------------------------------------- | ------------ |
| `You are not a member of this server`    | Not a member |

### 404 Not Found

| `error_message`     | Cause                 |
| ------------------- | --------------------- |
| `Post not found`    | Post does not exist   |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 23 May 2026.
