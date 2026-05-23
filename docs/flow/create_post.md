## Overview

API ini digunakan untuk membuat post di server tertentu. Format request multipart, image required (di-validate + convert ke WebP). Hanya member server yang boleh posting.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant MinIO

    Client->>BE: POST /api/servers/(serverId)/posts (multipart)
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi serverId (UUID)
    alt UUID invalid
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: COUNT server_members WHERE server_id = $1 AND user_id = $2
    alt Bukan member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>BE: ExtractAndValidateImage field "image" (max 5MB, jpg/png/gif/webp), convert WebP 512x512
    BE->>BE: Validasi caption (req, max 2000)
    alt Caption invalid
        BE-->>Client: 400 caption is required / caption must be at most 2000 characters
    end
    BE->>Postgres: BEGIN
    BE->>Postgres: INSERT INTO server_post_images
    BE->>Postgres: INSERT INTO server_posts
    BE->>MinIO: PutObject server/post/(postImageId).webp
    BE->>Postgres: COMMIT
    BE->>Postgres: SELECT post detail (join author identity, likeCount, commentCount, userLiked, isOwner)
    BE-->>Client: 200 ServerPostResponse
```

---

## Notes Redis

Tidak pakai Redis (selain middleware auth check).

---

## Notes MinIO

- Bucket: `MINIO_BUCKET_NAME` (default `virdan`)
- Object key: `server/post/{postImageId}.webp`
- Content-Type: `image/webp`
- Aksi: PutObject

---

## Notes Postgres/DB

| Tabel                | Kolom                                              | Aksi   | Keterangan                          |
| -------------------- | -------------------------------------------------- | ------ | ----------------------------------- |
| `server_members`     | (count)                                            | SELECT | Cek apakah user member               |
| `server_post_images` | id, bucket, object_key, mime_type, size, ...       | INSERT | Image row                            |
| `server_posts`       | id, server_id, author_id, post_image_id, caption   | INSERT | Post row                             |

---

## Prerequisites

User adalah member server target. Punya file image valid.

---

## Validasi Request

Path parameter:

| Field      | Tipe   | Wajib | Aturan          |
| ---------- | ------ | ----- | --------------- |
| `serverId` | string | ya    | Required, UUID  |

Multipart body:

| Field     | Tipe   | Wajib | Aturan                                          |
| --------- | ------ | ----- | ----------------------------------------------- |
| `image`   | file   | ya    | Image (jpg/jpeg/png/gif/webp), max 5MB           |
| `caption` | string | ya    | Required, max 2000 karakter                     |

---

## Response

### 200 OK

```json
{
  "id": "post-uuid",
  "serverId": "server-uuid",
  "caption": "Hello world!",
  "imageUrl": "http://.../server/post/imageId.webp",
  "author": {
    "userId": "user-uuid",
    "nickname": "GamerX",
    "username": "gamerx",
    "avatarUrl": "http://.../profile/avatar/uuid.webp",
    "status": "ACTIVE"
  },
  "likeCount": 0,
  "commentCount": 0,
  "userLiked": false,
  "isOwner": true,
  "createdAt": "2026-05-23T10:00:00Z",
  "updatedAt": "2026-05-23T10:00:00Z"
}
```

### 400 Bad Request

| `error_message`                              | Penyebab                       |
| -------------------------------------------- | ------------------------------ |
| `serverId is not a valid UUID`               | UUID invalid                    |
| `image is required`                          | File image tidak ada            |
| `image size exceeded 5MB limit`              | File terlalu besar              |
| `invalid file extension: ...`                | Ekstensi tidak diizinkan        |
| `invalid image type: ...`                    | MIME type tidak diizinkan       |
| `caption is required`                        | Caption kosong                  |
| `caption must be at most 2000 characters`    | Caption terlalu panjang         |

### 403 Forbidden

| `error_message`                          | Penyebab                |
| ---------------------------------------- | ----------------------- |
| `You are not a member of this server`    | User bukan member server |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
