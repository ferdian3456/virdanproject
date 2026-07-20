## Overview

API ini digunakan untuk logout. Backend revoke SEMUA refresh token user dan hapus access token cache di Redis. Setelah logout, user harus login ulang.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant Redis

    Client->>BE: POST /api/auth/logout (Bearer token)
    BE->>BE: Middleware extract userId dari JWT
    BE->>Redis: GET auth:accessToken:(userId)
    alt Token invalid
        BE-->>Client: 401 Unauthorized
    end
    BE->>Postgres: UPDATE refresh_tokens SET revoked_at=now, updated_at=now, updated_by=userId WHERE user_id = $1 AND revoked_at IS NULL
    BE->>Redis: DEL auth:accessToken:(userId)
    BE->>BE: Tutup koneksi WebSocket yang masih ada untuk user ini (Hub.CloseUser)
    BE-->>Client: 200 {status: "OK"}
```

---

## Notes Redis

1. auth access token:
   key: `auth:accessToken:(userId)`
   aksi: DEL

---

## Notes Postgres/DB

| Tabel            | Kolom      | Aksi   | Keterangan                                          |
| ---------------- | ---------- | ------ | --------------------------------------------------- |
| `refresh_tokens` | revoked_at | UPDATE | Set timestamp revoke untuk semua token aktif user   |
| `refresh_tokens` | updated_at | UPDATE | UTC now                                             |
| `refresh_tokens` | updated_by | UPDATE | userId yang melakukan logout                        |

---

## Prerequisites

User sudah login dan punya access token valid.

---

## Validasi Request

Endpoint ini tidak menerima body. Otentikasi via header `Authorization: Bearer <accessToken>`.

---

## Response

### 200 OK

```json
{
  "status": "OK"
}
```

### 401 Unauthorized

| `error_message`                              | Penyebab                                |
| -------------------------------------------- | --------------------------------------- |
| `Authorization header is missing`            | Header tidak ada                        |
| `Invalid authorization scheme`               | Tidak pakai Bearer prefix               |
| `Authentication token is expired`            | JWT sudah expired                       |
| `Authentication token is invalid`            | JWT invalid                              |
| `Authorization token not found or expired`   | Token tidak ada di cache Redis           |

---

## Update

Dokumentasi ini diupdate tanggal 20 Juli 2026.
