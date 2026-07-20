## Overview

API ini menampilkan conversation direct-message milik caller di dalam satu server, diurutkan dari aktivitas terbaru. Conversation baru tampil di sini setelah ada minimal satu pesan terkirim — conversation kosong yang baru dibuat (lihat `get_or_create_conversation.md`) tidak akan muncul sampai ada yang kirim pesan pertama. Cursor-based pagination.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/servers/(serverId)/conversations?limit&cursor
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi serverId (UUID)
    alt UUID invalid
        BE-->>Client: 400 Bad Request
    end
    BE->>Postgres: Cek caller adalah member serverId
    alt Bukan member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>BE: Clamp limit ke [1, 50], default 20; decode cursor (cursor invalid diabaikan diam-diam, bukan ditolak)
    BE->>Postgres: SELECT dm_conversation_states WHERE user_id, server_id, last_message_at IS NOT NULL ORDER BY last_message_at DESC LIMIT n+1
    BE->>BE: Untuk tiap row, cek apakah peer lagi online (in-memory WS hub)
    BE-->>Client: 200 {data: [...], page: {nextCursor}}
```

---

## Notes Redis

Tidak pakai Redis. Status online/offline (`isOnline`) dihitung dari in-process WebSocket connection hub (lihat `websocket_realtime.md`), bukan dari shared cache — di deployment multi-instance ini cuma merefleksikan koneksi yang dipegang instance yang menangani request itu saja.

---

## Notes Postgres/DB

| Tabel                      | Kolom                                                    | Aksi   | Keterangan                                                                  |
| ---------------------------- | -------------------------------------------------------------- | ------ | --------------------------------------------------------------------------------- |
| `server_members`           | (count)                                                          | SELECT | Cek membership                                                                       |
| `dm_conversation_states`   | conversation_id, unread_count, last_message_preview, last_message_at | SELECT | Difilter `user_id = caller AND server_id AND last_message_at IS NOT NULL`, cursor + limit |
| `server_member_profiles`   | nickname, username                                               | SELECT | Identitas per-server peer (left join)                                                |
| `profile_avatar_images`    | object_key                                                       | SELECT | Avatar peer (left join)                                                               |

---

## Prerequisites

Caller harus member `serverId`.

---

## Validasi Request

Path parameter:

| Field      | Tipe   | Wajib | Aturan          |
| ---------- | ------ | ----- | --------------- |
| `serverId` | string | ya    | Required, UUID  |

Query parameter:

| Field    | Tipe   | Wajib | Aturan                                                     |
| -------- | ------ | ----- | -------------------------------------------------------------- |
| `limit`  | int    | tidak | Default `20`; nilai `<= 0` jatuh ke default, nilai `> 50` di-clamp ke `50` |
| `cursor` | string | tidak | Cursor opaque dari `page.nextCursor` response sebelumnya; kalau gagal di-decode, diabaikan diam-diam (dianggap tidak ada cursor), bukan ditolak |

---

## Response

### 200 OK

```json
{
  "data": [
    {
      "id": "conversation-uuid",
      "serverId": "server-uuid",
      "peerUserId": "peer-user-uuid",
      "peer": {
        "nickname": "GamerX",
        "username": "gamerx",
        "avatarUrl": null
      },
      "unreadCount": 2,
      "isOnline": true,
      "lastMessagePreview": "see you tomorrow",
      "lastMessageAt": "2026-06-01T10:00:00Z"
    }
  ],
  "page": {
    "nextCursor": "base64-cursor-or-empty"
  }
}
```

| Field                 | Tipe        | Deskripsi                                                        |
| ----------------------- | ----------- | --------------------------------------------------------------------- |
| `unreadCount`          | int         | Unread count caller untuk conversation ini                              |
| `isOnline`             | bool        | Apakah peer sedang punya koneksi WebSocket aktif (hanya di instance ini)  |
| `lastMessagePreview`   | string/null | Preview terpotong dari pesan terakhir                                    |

### 400 Bad Request

| `error_message`                | Penyebab     |
| ------------------------------- | ------------ |
| `serverId is not a valid UUID`  | UUID invalid  |

### 403 Forbidden

| `error_message`                        | Penyebab     |
| ---------------------------------------- | ------------ |
| `You are not a member of this server`    | Bukan member  |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 20 Juli 2026.
