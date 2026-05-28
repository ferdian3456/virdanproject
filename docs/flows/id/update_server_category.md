## Overview

API ini digunakan untuk mengubah kategori server. Hanya owner yang boleh. Backend cek category aktif sebelum update.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: PUT /api/servers/(id)/category {categoryId}
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi serverId (UUID), categoryId (int, positive)
    alt Error Validasi
        BE-->>Client: 400 contohnya: categoryId must be positive
    end
    BE->>Postgres: Cek ownership
    alt Bukan owner
        BE-->>Client: 403 You are not the owner of this server
    end
    BE->>Postgres: SELECT 1 FROM server_categories WHERE id = $1 AND is_active = true
    alt Category tidak ada
        BE-->>Client: 404 Category not found or inactive
    end
    BE->>Postgres: UPDATE servers SET category_id = $1, updated_at = $2, updated_by = $3 WHERE id = $4
    BE-->>Client: 200 {id, updatedAt}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel               | Kolom        | Aksi   | Keterangan                    |
| ------------------- | ------------ | ------ | ----------------------------- |
| `servers`           | owner_id     | SELECT | Cek ownership                 |
| `server_categories` | id, is_active | SELECT | Cek category exists & aktif   |
| `servers`           | category_id  | UPDATE | Set category baru             |
| `servers`           | updated_at   | UPDATE | UTC now                        |
| `servers`           | updated_by   | UPDATE | userId                         |

---

## Prerequisites

User adalah owner server.

---

## Validasi Request

| Field        | Tipe   | Wajib | Aturan                  |
| ------------ | ------ | ----- | ----------------------- |
| `id` (path)  | string | ya    | Required, UUID          |
| `categoryId` | int    | ya    | Required, positif > 0   |

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

| `error_message`                  | Penyebab                       |
| -------------------------------- | ------------------------------ |
| `serverId is not a valid UUID`   | UUID invalid                    |
| `categoryId is required`         | CategoryId kosong               |
| `categoryId must be positive`    | CategoryId <= 0                 |

### 403 Forbidden

| `error_message`                          | Penyebab          |
| ---------------------------------------- | ----------------- |
| `You are not the owner of this server`   | Bukan owner       |

### 404 Not Found

| `error_message`                       | Penyebab                          |
| ------------------------------------- | --------------------------------- |
| `Category not found or inactive`      | Category tidak ada / is_active=false |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
