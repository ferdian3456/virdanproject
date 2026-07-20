## Overview

API ini digunakan untuk membuat comment pada post. Bisa juga reply comment lain dengan kirim `parentId` (UUID comment parent). User harus member server. Kalau sukses, push notification dikirim ke recipient yang relevan — author post untuk comment top-level, atau author dari parent comment untuk reply — tapi hanya kalau recipient tersebut adalah user yang berbeda dari yang komentar (tidak ada self-notification).

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant Notification

    Client->>BE: POST /api/posts/(postId)/comments {content, parentId?}
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi postId (UUID), content (req, max 1000), parentId (UUID kalau ada)
    alt Error Validasi
        BE-->>Client: 400 contohnya: content must be at most 1000 characters
    end
    BE->>Postgres: SELECT server_id FROM server_posts WHERE id = $1
    alt Post tidak ada
        BE-->>Client: 404 Post not found
    end
    BE->>Postgres: Cek server membership
    alt Bukan member
        BE-->>Client: 403 You are not a member of this server
    end
    alt parentId dikirim
        BE->>Postgres: SELECT 1 FROM server_post_comments WHERE id = parentId AND post_id = postId
        alt Parent tidak ada / beda post
            BE-->>Client: 404 Parent comment not found in this post
        end
    end
    BE->>Postgres: INSERT INTO server_post_comments
    BE->>Postgres: Resolve server_member_profiles.id milik actor
    alt profile actor berhasil di-resolve
        alt parentId nil (comment top-level)
            BE->>Postgres: SELECT author_id FROM server_posts WHERE id = postId
            alt Author post != userId actor
                BE->>Notification: Notify([{type: "comment", recipient: postAuthorId, actor: userId, postId, serverId}])
            end
        else parentId dikirim (reply)
            BE->>Postgres: SELECT author_id FROM server_post_comments WHERE id = parentId
            alt Author parent comment != userId actor
                BE->>Notification: Notify([{type: "reply", recipient: parentAuthorId, actor: userId, postId, commentId, serverId}])
            end
        end
    end
    BE->>Postgres: SELECT comment detail (author identity)
    BE-->>Client: 200 ServerCommentResponse
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel                    | Kolom                                        | Aksi   | Keterangan                              |
| ------------------------ | -------------------------------------------- | ------ | --------------------------------------- |
| `server_posts`           | server_id                                    | SELECT | Ambil server_id                          |
| `server_members`         | (count)                                      | SELECT | Cek membership                           |
| `server_post_comments`   | (count)                                      | SELECT | Cek parent valid (kalau parentId dikirim) |
| `server_post_comments`   | id, post_id, author_id, parent_id, content   | INSERT | Comment baru                              |
| `server_member_profiles` | (profile id)                                 | SELECT | Resolve profile id actor untuk notifikasi |
| `server_posts`           | author_id                                    | SELECT | Resolve author post (hanya untuk notifikasi comment top-level) |
| `server_post_comments`   | author_id                                    | SELECT | Resolve author parent comment (hanya untuk notifikasi reply) |
| `server_member_profiles` | nickname, username, avatar_image_id          | SELECT | Author identity di server                 |

Catatan: notifikasi di-skip kalau recipient (author post atau author parent comment) adalah user yang sama dengan yang komentar.

---

## Prerequisites

User adalah member server tempat post berada. Bila reply, parent comment harus exists di post yang sama.

---

## Validasi Request

Path parameter:

| Field    | Tipe   | Wajib | Aturan          |
| -------- | ------ | ----- | --------------- |
| `postId` | string | ya    | Required, UUID  |

Body JSON:

| Field      | Tipe          | Wajib | Aturan                                    |
| ---------- | ------------- | ----- | ----------------------------------------- |
| `content`  | string        | ya    | Required, max 1000 karakter               |
| `parentId` | string (UUID) | tidak | Kalau diisi, harus UUID + parent harus exist di post yang sama |

---

## Response

### 200 OK

```json
{
  "id": "comment-uuid",
  "postId": "post-uuid",
  "parentId": null,
  "content": "Mantap!",
  "author": {
    "userId": "user-uuid",
    "nickname": "GamerX",
    "username": "gamerx",
    "avatarUrl": "http://.../webp",
    "status": "active"
  },
  "isOwner": true,
  "createdAt": "2026-05-23T10:00:00Z",
  "updatedAt": "2026-05-23T10:00:00Z"
}
```

### 400 Bad Request

| `error_message`                              | Penyebab                       |
| -------------------------------------------- | ------------------------------ |
| `postId is not a valid UUID`                 | postId invalid                  |
| `content is required`                        | Content kosong                  |
| `content must be at most 1000 characters`    | Content terlalu panjang         |
| `parentId is not a valid UUID`               | parentId bukan UUID             |

### 403 Forbidden

| `error_message`                          | Penyebab     |
| ---------------------------------------- | ------------ |
| `You are not a member of this server`    | Bukan member  |

### 404 Not Found

| `error_message`                                 | Penyebab                                |
| ----------------------------------------------- | --------------------------------------- |
| `Post not found`                                | Post tidak ada                          |
| `Parent comment not found in this post`         | parentId tidak ada / parent beda post   |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 20 Juli 2026.
