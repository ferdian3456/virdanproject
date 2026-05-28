## Overview

API ini digunakan untuk mengubah short name server. Hanya owner yang boleh.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: PUT /api/servers/(id)/shortName {shortName}
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi serverId (UUID), shortName (req, 2-10)
    alt Error Validasi
        BE-->>Client: 400 contohnya: shortName must be at most 10 characters
    end
    BE->>Postgres: SELECT count FROM servers WHERE id = $1 AND owner_id = $2
    alt User bukan owner
        BE-->>Client: 403 You are not the owner of this server
    end
    BE->>Postgres: UPDATE servers SET short_name = $1, updated_at = $2, updated_by = $3 WHERE id = $4
    BE-->>Client: 200 {id, updatedAt}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel     | Kolom       | Aksi   | Keterangan          |
| --------- | ----------- | ------ | ------------------- |
| `servers` | owner_id    | SELECT | Cek ownership       |
| `servers` | short_name  | UPDATE | Set short name baru |
| `servers` | updated_at  | UPDATE | UTC now              |
| `servers` | updated_by  | UPDATE | userId               |

---

## Prerequisites

User adalah owner server.

---

## Validasi Request

| Field       | Tipe   | Wajib | Aturan                            |
| ----------- | ------ | ----- | --------------------------------- |
| `id` (path) | string | ya    | Required, UUID                    |
| `shortName` | string | ya    | Required, min 2, max 10 karakter  |

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

| `error_message`                            | Penyebab                       |
| ------------------------------------------ | ------------------------------ |
| `serverId is not a valid UUID`             | serverId bukan UUID             |
| `shortName is required`                    | ShortName kosong                |
| `shortName must be at least 2 characters`  | Kurang dari 2                   |
| `shortName must be at most 10 characters`  | Lebih dari 10                   |

### 403 Forbidden

| `error_message`                          | Penyebab          |
| ---------------------------------------- | ----------------- |
| `You are not the owner of this server`   | Bukan owner       |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
