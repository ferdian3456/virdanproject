## Overview

This API is used to save (bookmark) a post to the user's private saved list. Save is private, per-server, and does not send a notification to the post owner. If the post is already saved, it returns `409 Post sudah disimpan` (explicit validation, not idempotent). Returns `userSaved: true`.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: POST /api/posts/(postId)/saves
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
    BE->>Postgres: SELECT EXISTS save WHERE post_id, user_id
    alt Already saved
        BE-->>Client: 409 Post sudah disimpan
    end
    BE->>Postgres: INSERT INTO server_post_saves ...
    BE-->>Client: 200 {postId, userSaved: true}
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
| `server_post_saves`  | (exists)                               | SELECT | Check whether already saved (reject duplicate)      |
| `server_post_saves`  | id, post_id, user_id, created_at, ... | INSERT | Insert new save. Unique index (post_id, user_id) as safety net |

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
  "userSaved": true
}
```

| Field       | Type   | Description                                     |
| ----------- | ------ | ----------------------------------------------- |
| `postId`    | string | Post UUID                                       |
| `userSaved` | bool   | Always `true` after this endpoint succeeds      |

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

### 409 Conflict

| `error_message`        | Cause                   |
| ---------------------- | ----------------------- |
| `Post sudah disimpan`  | Post was already saved  |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 2 June 2026.
