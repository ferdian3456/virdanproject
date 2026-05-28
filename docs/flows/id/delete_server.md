## Overview

API ini digunakan untuk hard-delete server. Hanya owner yang boleh. FK CASCADE membersihkan `server_roles`, `server_members`, `server_invites`, `server_member_profiles`, `server_posts`, `server_post_comments`, `server_post_likes`. Object MinIO sengaja dibiarkan orphan di Phase 1 (cleanup job di Phase 2).

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: DELETE /api/servers/(id)
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi serverId (UUID)
    alt UUID invalid
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: SELECT count FROM servers WHERE id = $1 AND owner_id = $2
    alt Bukan owner
        BE-->>Client: 403 You are not the owner of this server
    end
    BE->>Postgres: DELETE FROM servers WHERE id = $1
    note over Postgres: FK CASCADE menghapus roles, members, invites, profiles, posts, comments, likes
    BE-->>Client: 200 {status: "OK"}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel     | Kolom    | Aksi   | Keterangan                                            |
| --------- | -------- | ------ | ----------------------------------------------------- |
| `servers` | owner_id | SELECT | Cek ownership                                         |
| `servers` | id       | DELETE | Hard delete (FK CASCADE → roles, members, profiles, posts, comments, likes, invites) |

---

## Prerequisites

User adalah owner server.

---

## Validasi Request

Path parameter:

| Field | Tipe   | Wajib | Aturan          |
| ----- | ------ | ----- | --------------- |
| `id`  | string | ya    | Required, UUID  |

Tidak ada body.

---

## Response

### 200 OK

```json
{
  "status": "OK"
}
```

### 400 Bad Request

| `error_message`                  | Penyebab     |
| -------------------------------- | ------------ |
| `serverId is not a valid UUID`   | UUID invalid  |

### 403 Forbidden

| `error_message`                          | Penyebab     |
| ---------------------------------------- | ------------ |
| `You are not the owner of this server`   | Bukan owner   |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
