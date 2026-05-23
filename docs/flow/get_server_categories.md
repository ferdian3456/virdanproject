## Overview

API ini digunakan untuk ambil daftar kategori server. Pagination cursor-based (cursor = id integer terakhir).

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/servers/categories?limit=50&cursor=3
    BE->>BE: Middleware extract userId (route protected)
    BE->>BE: Parse limit (default 50, max 100)
    BE->>BE: Parse cursor (int)
    alt Cursor invalid
        BE-->>Client: 400 Invalid cursor
    end
    BE->>Postgres: SELECT id, name FROM server_categories WHERE id > $cursor AND is_active = true ORDER BY id LIMIT $1
    BE->>BE: Bila len > limit, build nextCursor (id terakhir), drop sisa
    BE-->>Client: 200 {data, page}
```

---

## Notes Redis

Tidak pakai Redis (selain middleware auth check).

---

## Notes Postgres/DB

| Tabel               | Kolom        | Aksi   | Keterangan                          |
| ------------------- | ------------ | ------ | ----------------------------------- |
| `server_categories` | id, name, is_active | SELECT | Filter aktif, ORDER BY id ASC |

---

## Prerequisites

User sudah login dengan access token valid.

---

## Validasi Request

Query parameters:

| Field    | Tipe   | Wajib | Aturan                                          |
| -------- | ------ | ----- | ----------------------------------------------- |
| `limit`  | int    | tidak | 1-100, default 50 (di luar range → 50)          |
| `cursor` | int    | tidak | Id terakhir dari halaman sebelumnya             |

---

## Response

### 200 OK

```json
{
  "data": [
    { "id": 1, "categoryName": "Education" },
    { "id": 2, "categoryName": "Music" },
    { "id": 3, "categoryName": "Gaming" }
  ],
  "page": {
    "nextCursor": "3"
  }
}
```

| Field          | Tipe   | Deskripsi                                          |
| -------------- | ------ | -------------------------------------------------- |
| `id`           | int    | Category ID                                        |
| `categoryName` | string | Nama kategori                                      |
| `nextCursor`   | string | Id terakhir untuk halaman berikut (kosong = habis) |

### 400 Bad Request

| `error_message`     | Penyebab                                       |
| ------------------- | ---------------------------------------------- |
| `Invalid cursor`    | Cursor bukan integer valid                     |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
