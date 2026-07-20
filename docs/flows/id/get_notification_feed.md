## Overview

Mengambil feed notifikasi user untuk sebuah server (per-server, tidak global). Notifikasi adalah arsip interaksi (like/comment/reply) ke konten user. Cursor-based pagination. Requester wajib member server tersebut.

---

## Auth

API protected — perlu header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/servers/(serverId)/notifications?limit=10&cursor=...
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi serverId (UUID)
    BE->>Postgres: Cek requester membership
    alt Bukan member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>BE: Decode cursor bila ada
    BE->>Postgres: SELECT notifications WHERE recipient_user_id = $1 AND server_id = $2 ORDER BY created_at DESC, id DESC
    BE->>BE: Bila len > limit, build nextCursor
    BE-->>Client: 200 {data, page}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel                    | Kolom                          | Aksi   | Keterangan                            |
| ------------------------ | ------------------------------ | ------ | ------------------------------------- |
| `server_members`         | (count)                        | SELECT | Cek requester membership              |
| `notifications`          | recipient_user_id, server_id   | SELECT | Filter notif user di server tsb       |
| `server_member_profiles` | username, avatar_image_id      | SELECT | Identitas actor per server            |
| `profile_avatar_images`  | object_key                     | SELECT | Build actorAvatarUrl                  |

---

## Prerequisites

Requester adalah member server.

---

## Validasi Request

| Field      | Tipe   | Wajib | Aturan          |
| ---------- | ------ | ----- | --------------- |
| `serverId` | string | ya    | Required, UUID  |
| `limit`    | int    | tidak | Default 10 bila tidak dikirim, bukan angka, atau <= 0; dipangkas (clamp) ke 20 bila lebih dari 20 |
| `cursor`   | string | tidak | Base64 `{createdAt, id}`, harus berhasil di-decode atau 400 dikembalikan |

---

## Response

### 200 OK

```json
{
  "data": [ /* NotificationResponse */ ],
  "page": { "nextCursor": "base64-or-empty" }
}
```

### 400 Bad Request

| `error_message`                 | Penyebab                                         |
| -------------------------------- | --------------------------------------------------- |
| `serverId is required`          | Segmen path serverId kosong                        |
| `serverId is not a valid UUID`  | serverId bukan format UUID                         |
| `Invalid cursor`                | Query param `cursor` gagal di-decode base64/JSON   |

### 403 Forbidden

| `error_message`                       | Penyebab               |
| ------------------------------------- | ---------------------- |
| `You are not a member of this server` | Requester bukan member |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini dibuat tanggal 1 Juni 2026 (notifikasi per-server).
Diupdate tanggal 20 Juli 2026 (menambahkan tabel error validasi 400 Bad Request dan memperjelas perilaku `limit`/`cursor`).
