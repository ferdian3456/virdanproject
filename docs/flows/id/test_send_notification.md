## Overview

API ini digunakan untuk mengirim test push notification ke semua device yang terdaftar milik user yang sedang login. Berguna untuk memverifikasi bahwa pipa FCM berjalan end-to-end. Token FCM yang sudah tidak valid (unregistered/invalid) dibersihkan otomatis setelah pengiriman.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

---

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant FCM

    Client->>BE: POST /api/notifications/test-send
    BE->>BE: Middleware extract userId
    BE->>Postgres: SELECT token FROM device_tokens WHERE user_id = $1
    alt Tidak ada device terdaftar
        BE-->>Client: 404 No device registered for this user
    end
    BE->>FCM: SendEachForMulticast (semua token user)
    alt FCM error (network/server)
        BE->>BE: Log error, lanjut (tidak return error ke client)
    end
    BE->>BE: Loop response, kumpulkan token Unregistered/InvalidArgument
    alt Ada token invalid
        BE->>Postgres: DELETE FROM device_tokens WHERE token = ANY($1)
    end
    BE-->>Client: 200 {"status": "OK"}
```

---

## Notes Postgres/DB

| Tabel           | Kolom  | Aksi   | Keterangan                                         |
| --------------- | ------ | ------ | -------------------------------------------------- |
| `device_tokens` | token  | SELECT | Ambil semua token milik user                       |
| `device_tokens` | token  | DELETE | Hapus token invalid setelah FCM response (cleanup) |

---

## Notes FCM

- Menggunakan `SendEachForMulticast` — 1 HTTP call ke FCM untuk semua device user.
- Payload: `notification{Title:"Virdan", Body:"Test notification berhasil."}` + `data{type:"test"}`.
- Priority Android: `high`.
- Kegagalan FCM (network error, server error) tidak dikembalikan sebagai error ke client — hanya di-log.

---

## Response

### 200 OK

```json
{
  "status": "OK"
}
```

### 404 Not Found

| `error_message`                        | Penyebab                          |
| -------------------------------------- | --------------------------------- |
| `No device registered for this user`   | User belum pernah register device |

### 401 Unauthorized

Standard auth errors.

---

## Update

Dokumentasi ini diupdate tanggal 30 Mei 2026.
