## Overview

API ini mengupdate preferensi notifikasi push milik user (per-type: like, comment, reply). Mengikuti model IG: row notifikasi tetap selalu dibuat (feed = arsip lengkap), preferensi ini HANYA menggerbang apakah push dikirim ke device, bukan keberadaan row di feed.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: PUT /api/users/me/notification-preferences
    BE->>BE: Middleware extract userId
    BE->>BE: Parse body (notifLike, notifComment, notifReply)
    alt Body invalid
        BE-->>Client: 400 Bad Request
    end
    BE->>Postgres: UPDATE users.settings (notification prefs) WHERE id = userId
    BE-->>Client: 200 {status: OK}
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel   | Kolom                          | Aksi   | Keterangan                                  |
| ------- | ------------------------------ | ------ | ------------------------------------------- |
| `users` | settings (notification prefs)  | UPDATE | Simpan toggle push per-type milik user      |

---

## Prerequisites

User sudah login (punya access token valid).

---

## Validasi Request

Body (JSON):

| Field          | Tipe | Wajib | Aturan          |
| -------------- | ---- | ----- | --------------- |
| `notifLike`    | bool | ya    | true/false       |
| `notifComment` | bool | ya    | true/false       |
| `notifReply`   | bool | ya    | true/false       |

---

## Response

### 200 OK

```json
{
  "status": "OK"
}
```

| Field    | Tipe   | Deskripsi              |
| -------- | ------ | ---------------------- |
| `status` | string | Selalu `OK` saat sukses |

### 400 Bad Request

| `error_message`     | Penyebab        |
| ------------------- | --------------- |
| Body tidak valid    | Payload salah    |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 3 Juni 2026.
