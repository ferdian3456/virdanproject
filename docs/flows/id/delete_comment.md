## Overview

API ini digunakan untuk hard-delete comment. Hanya author comment yang boleh. FK CASCADE menghapus reply (anak comment) bila ada.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: DELETE /api/posts/(postId)/comments/(commentId)
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi postId (UUID), commentId (UUID)
    alt Error Validasi
        BE-->>Client: 400 contohnya: commentId is not a valid UUID
    end
    BE->>Postgres: SELECT server_id dari post
    alt Post tidak ada
        BE-->>Client: 404 Post not found
    end
    BE->>Postgres: Cek server membership
    alt Bukan member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: Cek comment ownership (author_id = userId)
    alt Bukan author
        BE-->>Client: 403 You are not the author of this comment
    end
    BE->>Postgres: SELECT 1 FROM server_post_comments WHERE id = commentId AND post_id = postId
    alt Comment tidak ada / beda post
        BE-->>Client: 404 Comment not found in this post
    end
    BE->>Postgres: DELETE FROM server_post_comments WHERE id = $1
    note over Postgres: FK CASCADE → anak comment (reply) ikut terhapus
    BE-->>Client: 200 {status: "OK"}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel                  | Kolom              | Aksi   | Keterangan                                  |
| ---------------------- | ------------------ | ------ | ------------------------------------------- |
| `server_posts`         | server_id          | SELECT | Ambil server_id buat membership check        |
| `server_members`       | (count)            | SELECT | Cek membership                                |
| `server_post_comments` | author_id          | SELECT | Cek ownership                                 |
| `server_post_comments` | id, post_id        | SELECT | Cek comment exists & belong to post           |
| `server_post_comments` | id                 | DELETE | Hard-delete (FK CASCADE → reply)               |

---

## Prerequisites

User adalah member server dan author comment.

---

## Validasi Request

Path parameter:

| Field       | Tipe   | Wajib | Aturan          |
| ----------- | ------ | ----- | --------------- |
| `postId`    | string | ya    | Required, UUID  |
| `commentId` | string | ya    | Required, UUID  |

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

| `error_message`                   | Penyebab     |
| --------------------------------- | ------------ |
| `postId is not a valid UUID`      | UUID invalid  |
| `commentId is not a valid UUID`   | UUID invalid  |

### 403 Forbidden

| `error_message`                              | Penyebab                |
| -------------------------------------------- | ----------------------- |
| `You are not a member of this server`        | Bukan member             |
| `You are not the author of this comment`     | Bukan author comment     |

### 404 Not Found

| `error_message`                          | Penyebab                              |
| ---------------------------------------- | ------------------------------------- |
| `Post not found`                         | Post tidak ada                        |
| `Comment not found in this post`         | Comment tidak ada / beda post         |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
