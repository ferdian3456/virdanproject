## Overview

This API is used to hard-delete a server. Only the owner is allowed. FK CASCADE cleans up `server_roles`, `server_members`, `server_invites`, `server_member_profiles`, `server_posts`, `server_post_comments`, `server_post_likes`. The MinIO objects are intentionally left orphan in Phase 1 (cleanup job in Phase 2).

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: DELETE /api/servers/(id)
    BE->>BE: Middleware extract userId
    BE->>BE: Validate serverId (UUID)
    alt Invalid UUID
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: SELECT count FROM servers WHERE id = $1 AND owner_id = $2
    alt Not the owner
        BE-->>Client: 403 You are not the owner of this server
    end
    BE->>Postgres: DELETE FROM servers WHERE id = $1
    note over Postgres: FK CASCADE deletes roles, members, invites, profiles, posts, comments, likes
    BE-->>Client: 200 {status: "OK"}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table     | Column   | Action | Notes                                                 |
| --------- | -------- | ------ | ----------------------------------------------------- |
| `servers` | owner_id | SELECT | Check ownership                                       |
| `servers` | id       | DELETE | Hard delete (FK CASCADE → roles, members, profiles, posts, comments, likes, invites) |

---

## Prerequisites

User is the owner of the server.

---

## Request Validation

Path parameter:

| Field | Type   | Required | Rules           |
| ----- | ------ | -------- | --------------- |
| `id`  | string | yes      | Required, UUID  |

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

| `error_message`                  | Cause        |
| -------------------------------- | ------------ |
| `serverId is not a valid UUID`   | Invalid UUID  |

### 403 Forbidden

| `error_message`                          | Cause        |
| ---------------------------------------- | ------------ |
| `You are not the owner of this server`   | Not the owner |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 23 May 2026.
