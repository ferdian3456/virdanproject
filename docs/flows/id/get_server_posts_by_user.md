## Overview

API ini digunakan untuk mengambil daftar post member lain di sebuah server (grid profil mereka). Sama seperti `get_server_post_for_me` tapi target user diambil dari path param, bukan dari token. Requester wajib member server. Pagination cursor-based.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/servers/(serverId)/members/(userId)/posts?limit=10&cursor=...
    BE->>BE: Middleware extract requesterUserId
    BE->>BE: Validasi serverId & userId (UUID), limit (0-20)
    BE->>Postgres: Cek requester membership
    alt Requester bukan member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>BE: Decode cursor bila ada
    alt Cursor invalid
        BE-->>Client: 400 Invalid cursor
    end
    BE->>Postgres: SELECT posts WHERE server_id = $1 AND author_id = $2 (target) ORDER BY created_at DESC
    BE->>BE: Set isOwner relatif ke requester; bila len > limit build nextCursor
    BE-->>Client: 200 {data, page}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel                    | Kolom                               | Aksi   | Keterangan                              |
| ------------------------ | ----------------------------------- | ------ | --------------------------------------- |
| `server_members`         | (count)                             | SELECT | Cek requester membership                |
| `server_posts`           | server_id, author_id                | SELECT | Filter post milik target di server tsb  |
| `server_post_images`     | object_key                          | SELECT | Build imageUrl                          |
| `server_post_likes`      | (count + EXISTS)                    | SELECT | likeCount + userLiked                   |
| `server_post_comments`   | (count)                             | SELECT | commentCount                            |
| `server_member_profiles` | nickname, username, avatar_image_id | SELECT | Author identity per server              |

---

## Prerequisites

Requester adalah member server.

---

## Validasi Request

Path parameter:

| Field      | Tipe   | Wajib | Aturan         |
| ---------- | ------ | ----- | -------------- |
| `serverId` | string | ya    | Required, UUID |
| `userId`   | string | ya    | Required, UUID |

Query parameters:

| Field    | Tipe   | Wajib | Aturan                                                |
| -------- | ------ | ----- | ----------------------------------------------------- |
| `limit`  | int    | tidak | 0-20, default 10                                      |
| `cursor` | string | tidak | Base64 JSON `{id, createdAt}` dari halaman sebelumnya |

---

## Response

### 200 OK

Format sama dengan `get_server_posts`, hanya berisi post milik target user. Field `isOwner` bernilai `false` (kecuali requester melihat dirinya sendiri).

```json
{
  "data": [ /* ServerPostResponse */ ],
  "page": {
    "nextCursor": "base64-encoded"
  }
}
```

### 400 Bad Request

| `error_message`                | Penyebab       |
| ------------------------------ | -------------- |
| `serverId is not a valid UUID` | UUID invalid   |
| `userId is not a valid UUID`   | UUID invalid   |
| `limit must be at most 20`     | Limit > 20     |
| `Invalid cursor`               | Cursor invalid |

### 403 Forbidden

| `error_message`                       | Penyebab               |
| ------------------------------------- | ---------------------- |
| `You are not a member of this server` | Requester bukan member |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini dibuat tanggal 1 Juni 2026.
