## Overview

This API is used to unsave (remove bookmark) a post from the user's private saved list. If the post was never saved, it returns `404 Post belum disimpan`. Returns `userSaved: false`.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: DELETE /api/posts/(postId)/saves
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
    alt Not saved yet
        BE-->>Client: 404 Post belum disimpan
    end
    BE->>Postgres: DELETE FROM server_post_saves WHERE post_id, user_id
    BE-->>Client: 200 {postId, userSaved: false}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table                | Column                 | Action | Notes                                               |
| -------------------- | ---------------------- | ------ | --------------------------------------------------- |
| `server_posts`       | server_id              | SELECT | Fetch server_id for the membership check            |
| `server_members`     | (count)                | SELECT | Check membership                                    |
| `server_post_saves`  | (exists)               | SELECT | Check whether it is actually saved                  |
| `server_post_saves`  | post_id, user_id       | DELETE | Remove the save                                     |

---

## Prerequisites

User is a member of the server where the post resides and the post is already saved.

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
  "userSaved": false
}
```

| Field       | Type   | Description                                     |
| ----------- | ------ | ----------------------------------------------- |
| `postId`    | string | Post UUID                                       |
| `userSaved` | bool   | Always `false` after this endpoint succeeds     |

### 400 Bad Request

| `error_message`               | Cause        |
| ----------------------------- | ------------ |
| `postId is not a valid UUID`  | Invalid UUID |

### 403 Forbidden

| `error_message`                          | Cause        |
| ---------------------------------------- | ------------ |
| `You are not a member of this server`    | Not a member |

### 404 Not Found

| `error_message`        | Cause                   |
| ---------------------- | ----------------------- |
| `Post not found`       | Post does not exist     |
| `Post belum disimpan`  | Post was never saved    |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 2 June 2026.
