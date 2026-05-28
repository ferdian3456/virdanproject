## Overview

API ini digunakan untuk rotate access token + refresh token. Akan revoke refresh token family yang lama dan generate pasangan baru. Kalau refresh token yang sudah revoked dipakai lagi (token reuse), system anggap sebagai theft dan revoke SEMUA token user (security escalation).

---

## Auth

API ini adalah api public jadi tidak perlu authorization header. Yang diperlukan: body dengan `refreshToken` valid.

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant Postgres
    participant Redis

    Client->>BE: POST /api/auth/refresh {refreshToken}
    BE->>BE: Validasi refreshToken (required)
    alt Error Validasi
        BE-->>Client: 400 refreshToken is required
    end
    BE->>BE: tokenHash = SHA256(refreshToken)
    BE->>Postgres: SELECT FROM refresh_tokens WHERE token_hash = $1
    alt Tidak ada token
        BE-->>Client: 404 Refresh token is not found
    end
    alt Token sudah revoked (REUSE DETECTED)
        BE->>Postgres: UPDATE refresh_tokens SET revoked_at = now WHERE user_id = $1 (ALL tokens user)
        BE-->>Client: 401 Session expired. Please login again.
    end
    alt Token expired (now > expires_at)
        BE-->>Client: 401 Refresh token has expired
    end
    BE->>Postgres: BEGIN
    BE->>Postgres: UPDATE refresh_tokens SET revoked_at = now WHERE user_id = $1 AND token_family = $2 AND revoked_at IS NULL
    BE->>BE: Generate access token (JWT, 15m) + refresh token (UUID, 7d) - new family
    BE->>Postgres: INSERT INTO refresh_tokens (new)
    BE->>Redis: SET auth:accessToken:(userId) = hash(accessToken), EX 15m
    BE->>Postgres: COMMIT
    BE-->>Client: 200 TokenResponse
```

---

## Notes Redis

1. auth access token:
   key: `auth:accessToken:(userId)`
   value: SHA256 hash access token baru
   ttl: 15 menit

---

## Notes Postgres/DB

| Tabel            | Kolom         | Aksi   | Keterangan                                                                |
| ---------------- | ------------- | ------ | ------------------------------------------------------------------------- |
| `refresh_tokens` | (semua)       | SELECT | Cari refresh token by token_hash                                          |
| `refresh_tokens` | revoked_at    | UPDATE | Revoke token family lama (atau ALL token user kalau reuse detected)        |
| `refresh_tokens` | updated_at    | UPDATE | UTC now                                                                   |
| `refresh_tokens` | updated_by    | UPDATE | userId                                                                    |
| `refresh_tokens` | id, ...       | INSERT | Refresh token baru dengan family UUID baru                                |

---

## Prerequisites

User punya refresh token aktif (belum expired, belum revoked).

---

## Validasi Request

| Field          | Tipe   | Wajib | Aturan                |
| -------------- | ------ | ----- | --------------------- |
| `refreshToken` | string | ya    | Required, tidak kosong |

---

## Response

### 200 OK

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "accessTokenExpiresIn": 900,
  "refreshToken": "new-uuid-refresh-token",
  "refreshTokenExpiresIn": 604800,
  "tokenType": "Bearer"
}
```

### 400 Bad Request

| `error_message`            | Penyebab                  |
| -------------------------- | ------------------------- |
| `refreshToken is required` | Refresh token kosong      |

### 401 Unauthorized

| `error_message`                          | Penyebab                                                                |
| ---------------------------------------- | ----------------------------------------------------------------------- |
| `Session expired. Please login again.`   | Token sudah revoked dipakai lagi (token theft escalation - ALL revoked)  |
| `Refresh token has expired`              | Refresh token sudah lewat expiry (7 hari)                                |

### 404 Not Found

| `error_message`                | Penyebab                                       |
| ------------------------------ | ---------------------------------------------- |
| `Refresh token is not found`   | token_hash tidak ditemukan di tabel refresh_tokens |

---

## Update

Dokumentasi ini diupdate tanggal 23 Mei 2026.
