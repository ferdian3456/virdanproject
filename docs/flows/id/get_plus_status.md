## Overview

API ini digunakan untuk mengambil status langganan Virdan Plus sebuah server: apakah sedang aktif, kapan expire-nya, dan rincian harga saat ini untuk beli/perpanjang.

---

## Auth

API ini adalah api protected jadi perlu authorization header `Bearer <accessToken>`.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres

    Client->>BE: GET /api/servers/(serverId)/plus
    BE->>BE: Middleware extract userId
    BE->>BE: Validasi serverId (UUID)
    alt UUID invalid
        BE-->>Client: 400 serverId is not a valid UUID
    end
    BE->>Postgres: COUNT server_members WHERE server_id = $1 AND user_id = $2
    alt Bukan member
        BE-->>Client: 403 You are not a member of this server
    end
    BE->>Postgres: SELECT plus_expires_at FROM server_plus_orders WHERE server_id = $1 AND status = 'PAID' AND plus_expires_at > now() ORDER BY plus_expires_at DESC LIMIT 1
    BE->>BE: Hitung rincian harga (base + tax)
    BE-->>Client: 200 PlusStatusResponse
```

---

## Notes Redis

Tidak pakai Redis.

---

## Notes Postgres/DB

| Tabel                 | Kolom           | Aksi   | Keterangan                                                              |
| --------------------- | --------------- | ------ | ------------------------------------------------------------------------- |
| `server_members`      | (count)         | SELECT | Cek apakah caller adalah member server                                    |
| `server_plus_orders`  | plus_expires_at | SELECT | Order `PAID` terakhir yang masih berlaku (`status = 'PAID' AND plus_expires_at > now()`), yang paling lama expire-nya yang dipakai |

---

## Prerequisites

Caller harus member server yang dituju.

---

## Validasi Request

Path parameter:

| Field      | Tipe   | Wajib | Aturan          |
| ---------- | ------ | ----- | --------------- |
| `serverId` | string | ya    | Required, UUID  |

Tidak ada body.

---

## Response

### 200 OK

```json
{
  "active": false,
  "expiresAt": null,
  "durationDays": 30,
  "price": {
    "baseIdr": 50000,
    "taxIdr": 5500,
    "totalIdr": 55500
  }
}
```

| Field                 | Tipe        | Deskripsi                                                              |
| --------------------- | ----------- | -------------------------------------------------------------------------- |
| `active`              | bool        | `true` kalau ada order `PAID` yang belum expire untuk server ini            |
| `expiresAt`           | string/null | Waktu expire order yang aktif, `null` kalau `active` bernilai `false`       |
| `durationDays`        | int         | Lama langganan tetap dalam hari (saat ini `30`)                             |
| `price.baseIdr`       | int         | Harga dasar dalam IDR (saat ini `50000`)                                    |
| `price.taxIdr`        | int         | Pajak, 11% dari base, dalam IDR (saat ini `5500`)                            |
| `price.totalIdr`      | int         | `baseIdr + taxIdr` (saat ini `55500`)                                        |

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
