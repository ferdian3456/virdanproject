## Overview

API ini digunakan untuk ambil daftar server yang user ikuti. Sorted by `joined_at` descending dengan pagination cursor-based.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/servers/me?limit=10&cursor=...
    BE->>BE: Middleware extract userId
    BE->>BE: Parse limit (default 10, max 50)
    alt Cursor invalid
        BE-->>Client: 400 Invalid cursor
    end
    BE->>Postgres: SELECT server fields + memberCount + myNickname + myAvatar dari server_members JOIN servers JOIN profiles WHERE user_id = $1 (after cursor)
    BE->>BE: Bila len > limit, build nextCursor dari item ke-limit
    BE-->>Client: 200 {data, page}
```

---

## Notes Redis

Tidak pakai Redis (selain middleware auth check).

---

## Notes Postgres/DB

| Tabel                     | Kolom                                    | Aksi   | Keterangan                                        |
| ------------------------- | ---------------------------------------- | ------ | ------------------------------------------------- |
| `server_members`          | server_id, user_id, joined_at            | SELECT | Filter user, sort joined_at DESC, after cursor   |
| `servers`                 | id, name, short_name, category_id, avatar_image_id | SELECT | JOIN ke detail server                       |
| `server_categories`       | id, name                                 | SELECT | JOIN untuk categoryName                          |
| `server_avatar_images`    | object_key                               | SELECT | Build avatarUrl server                            |
| `server_member_profiles`  | nickname, avatar_image_id                | SELECT | Identitas user di server tsb (myNickname, myAvatar) |
| `profile_avatar_images`   | object_key                               | SELECT | Build myAvatarUrl                                  |

---

## Prerequisites

User sudah login dengan access token valid.

---

## Validasi Request

Query parameters:

| Field    | Tipe   | Wajib | Aturan                                                  |
| -------- | ------ | ----- | ------------------------------------------------------- |
| `limit`  | int    | tidak | 1-50, default 10                                        |
| `cursor` | string | tidak | Base64 JSON `{serverId, joinedAt}` dari halaman sebelumnya |

---

## Response

### 200 OK

```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Gaming Squad",
      "shortName": "GS",
      "avatarUrl": "http://.../webp",
      "categoryId": 3,
      "categoryName": "Gaming",
      "memberCount": 42,
      "joinedAt": "2026-05-20T08:00:00Z",
      "myNickname": "GamerX",
      "myAvatarUrl": "http://.../profile/avatar/uuid.webp"
    }
  ],
  "page": {
    "nextCursor": "base64-encoded-cursor"
  }
}
```

### 400 Bad Request

| `error_message`  | Penyebab                          |
| ---------------- | --------------------------------- |
| `Invalid cursor` | Cursor tidak bisa di-decode JSON   |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
