## Overview

API ini digunakan untuk unlike post. Idempotent — kalau user belum like, request ini tetap return 200 (tidak error). Return likeCount terbaru + `userLiked: false`.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: DELETE /api/posts/(postId)/likes
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi postId (UUID)
    alt UUID invalid
        BE-->>Client: 400 postId is not a valid UUID
    end
    BE->>Postgres: SELECT server_id dari post
    alt Post tidak ada
        BE-->>Client: 404 Post not found
    end
    BE->>Postgres: Cek server membership
    alt Bukan member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: SELECT EXISTS like (post_id, user_id)
    alt Sudah like
        BE->>Postgres: DELETE FROM server_post_likes WHERE post_id = $1 AND user_id = $2
    end
    BE->>Postgres: COUNT likes WHERE post_id = $1
    BE-->>Client: 200 {postId, userLiked: false, likeCount}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel                | Kolom              | Aksi   | Keterangan                  |
| -------------------- | ------------------ | ------ | --------------------------- |
| `server_posts`       | server_id          | SELECT | Ambil server_id              |
| `server_members`     | (count)            | SELECT | Cek membership               |
| `server_post_likes`  | (exists)           | SELECT | Cek apakah user sudah like   |
| `server_post_likes`  | post_id, user_id   | DELETE | Hapus like (kalau ada)        |
| `server_post_likes`  | (count)            | SELECT | likeCount baru                |

---

## Prerequisites

User adalah member server tempat post berada.

---

## Validasi Request

Path parameter:

| Field    | Tipe   | Wajib | Aturan          |
| -------- | ------ | ----- | --------------- |
| `postId` | string | ya    | Required, UUID  |

Tidak ada body.

---

## Response

### 200 OK

```json
{
  "postId": "post-uuid",
  "userLiked": false,
  "likeCount": 12
}
```

### 400 Bad Request

| `error_message`               | Penyebab     |
| ----------------------------- | ------------ |
| `postId is not a valid UUID`  | UUID invalid  |

### 403 Forbidden

| `error_message`                          | Penyebab     |
| ---------------------------------------- | ------------ |
| `You are not a member of this server`    | Bukan member  |

### 404 Not Found

| `error_message`     | Penyebab            |
| ------------------- | ------------------- |
| `Post not found`    | Post tidak ada       |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
