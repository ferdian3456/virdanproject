## Overview

This API is used to change the server settings. Currently the settings only contain `isPrivate` (boolean). Only the owner is allowed.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: PUT /api/servers/(id)/settings {isPrivate}
    BE->>BE: Middleware extract userId
    BE->>BE: Validate serverId (UUID)
    alt Validation Error
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: Check ownership
    alt Not owner
        BE-->>Client: 403 You are not the owner of this server
    end
    BE->>BE: Marshal {isPrivate} to JSONB
    BE->>Postgres: UPDATE servers SET settings = $1, updated_at = $2, updated_by = $3 WHERE id = $4
    BE-->>Client: 200 {id, updatedAt}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table     | Column     | Action | Notes                   |
| --------- | ---------- | ------ | ----------------------- |
| `servers` | owner_id   | SELECT | Check ownership         |
| `servers` | settings   | UPDATE | Set new JSONB settings  |
| `servers` | updated_at | UPDATE | UTC now                 |
| `servers` | updated_by | UPDATE | userId                  |

---

## Prerequisites

The user is the server owner.

---

## Request Validation

| Field       | Type   | Required | Rules           |
| ----------- | ------ | -------- | --------------- |
| `id` (path) | string | yes      | Required, UUID  |
| `isPrivate` | bool   | no       | Default `false` |

---

## Response

### 200 OK

```json
{
  "id": "uuid",
  "updatedAt": "2026-05-23T10:00:00Z"
}
```

### 400 Bad Request

| `error_message`                | Cause           |
| ------------------------------ | --------------- |
| `serverId is not a valid UUID` | Invalid UUID    |

### 403 Forbidden

| `error_message`                          | Cause        |
| ---------------------------------------- | ------------ |
| `You are not the owner of this server`   | Not owner    |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 23 May 2026.
