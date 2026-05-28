## Overview

API ini digunakan untuk mengubah settings server. Saat ini settings hanya berisi `isPrivate` (boolean). Hanya owner yang boleh.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: PUT /api/servers/(id)/settings {isPrivate}
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi serverId (UUID)
    alt Error Validasi
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: Cek ownership
    alt Bukan owner
        BE-->>Client: 403 You are not the owner of this server
    end
    BE->>BE: Marshal {isPrivate} ke JSONB
    BE->>Postgres: UPDATE servers SET settings = $1, updated_at = $2, updated_by = $3 WHERE id = $4
    BE-->>Client: 200 {id, updatedAt}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel     | Kolom      | Aksi   | Keterangan              |
| --------- | ---------- | ------ | ----------------------- |
| `servers` | owner_id   | SELECT | Cek ownership           |
| `servers` | settings   | UPDATE | Set JSONB settings baru |
| `servers` | updated_at | UPDATE | UTC now                  |
| `servers` | updated_by | UPDATE | userId                   |

---

## Prerequisites

User adalah owner server.

---

## Validasi Request

| Field       | Tipe   | Wajib | Aturan          |
| ----------- | ------ | ----- | --------------- |
| `id` (path) | string | ya    | Required, UUID  |
| `isPrivate` | bool   | tidak | Default `false` |

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

| `error_message`                | Penyebab        |
| ------------------------------ | --------------- |
| `serverId is not a valid UUID` | UUID invalid    |

### 403 Forbidden

| `error_message`                          | Penyebab     |
| ---------------------------------------- | ------------ |
| `You are not the owner of this server`   | Bukan owner   |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
