## Overview

API ini mengirim pesan direct message di conversation yang sudah ada. Idempotent per `(conversationId, senderId, clientMessageId)` — kirim ulang pesan yang sama (misalnya setelah timeout di sisi client) dengan `clientMessageId` yang sama akan mengembalikan pesan aslinya, bukan bikin duplikat. Untuk pesan yang benar-benar baru, endpoint ini juga menyebarkan pesan ke koneksi WebSocket live milik peer (kalau ada, lihat `websocket_realtime.md`) dan memicu push notification best-effort.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant WS Hub
    participant FCM

    Client->>BE: POST /api/conversations/(conversationId)/messages {content, clientMessageId}
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi conversationId, clientMessageId (UUID); content wajib, max 4000 karakter
    alt Validasi gagal
        BE-->>Client: 400 Bad Request
    end
    BE->>Postgres: SELECT conversation by id
    alt Tidak ditemukan
        BE-->>Client: 404 Conversation not found
    end
    alt Caller bukan user_low/user_high di conversation itu
        BE-->>Client: 403 Not a participant of this conversation
    end
    BE->>Postgres: Cek caller masih member server-nya conversation
    alt Sudah bukan member lagi
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: BEGIN
    BE->>Postgres: INSERT INTO dm_messages ... ON CONFLICT (conversation_id, sender_id, client_message_id) DO NOTHING
    alt clientMessageId sudah pernah dipakai (duplikat/retry)
        BE->>Postgres: SELECT row pesan yang sudah ada
        BE->>Postgres: COMMIT
        BE-->>Client: 200 DmMessageResponse (pesan aslinya, tidak berubah)
    else pesan baru
        BE->>Postgres: UPDATE dm_conversations SET last_message_at
        BE->>Postgres: UPDATE dm_conversation_states (bump preview + tambah unread cuma buat peer)
        BE->>Postgres: COMMIT
        BE->>WS Hub: Publish "message.new" ke peer (best-effort, error tidak ditampilkan ke client)
        BE->>FCM: Push notification ke device peer (async, best-effort)
        BE-->>Client: 200 DmMessageResponse
    end
```

---

## Notes Redis

Tidak pakai Redis. Delivery real-time lewat in-process WebSocket hub (`shared.WsHub`/`WsBroker`), bukan pub/sub broker — lihat `websocket_realtime.md`.

---

## Notes Postgres/DB

| Tabel                     | Kolom                                                          | Aksi   | Keterangan                                                                       |
| --------------------------- | ---------------------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------- |
| `dm_conversations`        | (lookup)                                                                  | SELECT | Ambil conversation buat cek participant + membership server                              |
| `server_members`          | (count)                                                                    | SELECT | Cek ulang caller masih member server-nya conversation                                     |
| `dm_messages`             | id, conversation_id, sender_id, type, content, client_message_id, created_at | INSERT | Idempotent lewat `ON CONFLICT (conversation_id, sender_id, client_message_id) DO NOTHING` |
| `dm_messages`             | (lookup by client_message_id)                                              | SELECT | Cuma dijalankan kalau insert di atas nemu duplikat                                        |
| `dm_conversations`        | last_message_at                                                            | UPDATE | Cuma untuk pesan yang benar-benar baru                                                     |
| `dm_conversation_states`  | last_message_at, last_message_preview, unread_count                       | UPDATE | Cuma untuk pesan baru; `unread_count` cuma naik untuk row milik penerima, bukan pengirim  |

---

## Prerequisites

Caller harus salah satu dari dua participant `conversationId`, dan masih member server tempat conversation itu berada.

---

## Validasi Request

Path parameter:

| Field             | Tipe   | Wajib | Aturan          |
| ------------------- | ------ | ----- | --------------- |
| `conversationId`   | string | ya    | Required, UUID  |

Body JSON:

| Field              | Tipe   | Wajib | Aturan                             |
| -------------------- | ------ | ----- | -------------------------------------- |
| `content`            | string | ya    | Wajib, max 4000 karakter                |
| `clientMessageId`    | string | ya    | UUID, dipakai sebagai idempotency key    |

---

## Response

### 200 OK

```json
{
  "id": "message-uuid",
  "conversationId": "conversation-uuid",
  "senderId": "sender-uuid",
  "sender": {
    "nickname": "GamerX",
    "username": "gamerx",
    "avatarUrl": null
  },
  "type": "text",
  "content": "hello!",
  "clientMessageId": "client-generated-uuid",
  "createdAt": "2026-06-01T10:00:00Z"
}
```

`type` saat ini selalu `"text"`.

### 400 Bad Request

| `error_message`                              | Penyebab                     |
| ------------------------------------------------ | ------------------------------------ |
| `conversationId is not a valid UUID`             | `conversationId` invalid              |
| `clientMessageId is not a valid UUID`            | `clientMessageId` invalid              |
| `content is required`                            | Content kosong                          |
| `content must be at most 4000 characters`        | Content kepanjangan                     |

### 403 Forbidden

| `error_message`                            | Penyebab                                                     |
| ---------------------------------------------- | ------------------------------------------------------------------ |
| `Not a participant of this conversation`      | Caller bukan participant conversation itu                              |
| `You are not a member of this server`         | Caller sudah keluar dari server tempat conversation itu berada           |

### 404 Not Found

| `error_message`             | Penyebab                        |
| -------------------------------- | ------------------------------------ |
| `Conversation not found`        | `conversationId` tidak ada             |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 20 Juli 2026.
