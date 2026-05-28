## Overview

This API is used to change the server short name. Only the owner is allowed.

---

## Auth

This is a protected API, so it requires the authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: PUT /api/servers/(id)/shortName {shortName}
    BE->>BE: Middleware extract userId
    BE->>BE: Validate serverId (UUID), shortName (req, 2-10)
    alt Validation Error
        BE-->>Client: 400 e.g.: shortName must be at most 10 characters
    end
    BE->>Postgres: SELECT count FROM servers WHERE id = $1 AND owner_id = $2
    alt User not owner
        BE-->>Client: 403 You are not the owner of this server
    end
    BE->>Postgres: UPDATE servers SET short_name = $1, updated_at = $2, updated_by = $3 WHERE id = $4
    BE-->>Client: 200 {id, updatedAt}
```

---

## Notes Redis

Does not use Redis.

---

## Notes Postgres/DB

| Table     | Column      | Action | Notes               |
| --------- | ----------- | ------ | ------------------- |
| `servers` | owner_id    | SELECT | Check ownership     |
| `servers` | short_name  | UPDATE | Set new short name  |
| `servers` | updated_at  | UPDATE | UTC now             |
| `servers` | updated_by  | UPDATE | userId              |

---

## Prerequisites

The user is the server owner.

---

## Request Validation

| Field       | Type   | Required | Rules                             |
| ----------- | ------ | -------- | --------------------------------- |
| `id` (path) | string | yes      | Required, UUID                    |
| `shortName` | string | yes      | Required, min 2, max 10 characters  |

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

| `error_message`                            | Cause                          |
| ------------------------------------------ | ------------------------------ |
| `serverId is not a valid UUID`             | serverId is not a UUID         |
| `shortName is required`                    | ShortName is empty             |
| `shortName must be at least 2 characters`  | Less than 2                    |
| `shortName must be at most 10 characters`  | More than 10                   |

### 403 Forbidden

| `error_message`                          | Cause             |
| ---------------------------------------- | ----------------- |
| `You are not the owner of this server`   | Not owner         |

### 401 Unauthorized

Standard auth errors.

---

## Update

This documentation was last updated on 23 May 2026.
