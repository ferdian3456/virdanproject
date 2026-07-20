## Overview

Ini adalah endpoint WebSocket real-time yang dipakai untuk menerima event DM (pesan baru, read receipt, indikator mengetik, presence online/offline) tanpa perlu polling. Client buka satu koneksi per sesi; server menyebarkan event ke koneksi itu begitu terjadi di tempat lain (dipicu oleh `send_dm_message.md` dan `mark_conversation_read.md`). Berbeda dari endpoint REST, autentikasinya lewat **query parameter** access token (browser tidak bisa set custom header saat handshake WebSocket).

---

## Auth

Protected, tapi cara autentikasinya beda dari REST API: access token dikirim sebagai `?token=<accessToken>` di URL koneksi, bukan header `Authorization`, lalu divalidasi dengan cara yang sama (signature JWT + cek masih live/belum di-revoke).

## Flow

```mermaid
sequenceDiagram
    actor Client
    participant BE
    participant WS Hub

    Client->>BE: GET /api/ws/?token=(accessToken)  (request upgrade WebSocket)
    BE->>BE: Validasi query param token ada
    alt Token tidak ada
        BE-->>Client: 401 Missing token query parameter
    end
    BE->>BE: Validasi JWT + cek masih jadi access token yang live
    alt Token invalid/expired
        BE-->>Client: 401 (standard auth error)
    end
    BE->>BE: Wajib request ini benar-benar request upgrade WebSocket
    alt Bukan upgrade WebSocket (misal GET biasa)
        BE-->>Client: 426 Upgrade Required
    end
    BE->>WS Hub: Register koneksi untuk userId ini
    alt Caller sudah punya 5 koneksi live
        BE-->>Client: WS close, {"type":"error","payload":{"code":"WS_CONN_LIMIT","message":"too many connections"}}
    end
    BE->>WS Hub: Broadcast presence "online" ke peer DM caller (async)
    loop selama koneksi hidup
        WS Hub-->>Client: {"type":"message.new"|"message.read"|"presence"|"typing", "payload": {...}} (server → client, dikirim saat event terjadi di tempat lain)
        Client->>BE: {"type":"typing","payload":{"conversationId","isTyping"}} (client → server, opsional)
        BE->>WS Hub: Relay indikator mengetik ke peer (rate-limited maksimal sekali/detik per conversation)
        BE->>Client: Ping frame tiap 54 detik (client harus balas Pong; koneksi didaur ulang setelah 60 detik tidak ada balasan)
    end
    Client->>BE: Koneksi ditutup
    BE->>WS Hub: Unregister koneksi
    BE->>WS Hub: Broadcast presence "offline" ke peer (cuma kalau ini koneksi live terakhir caller)
```

---

## Notes Redis

Tidak pakai Redis. Delivery sepenuhnya in-process: `WsHub` menyimpan map `userId -> koneksi` di memory instance server tersebut, dan request lain mencapainya lewat interface `WsBroker` (`InProcessWsBroker`). **Di deployment multi-instance, event cuma terkirim ke koneksi yang dipegang oleh instance yang sama dengan yang memicu event itu** — saat ini belum ada fanout lintas-instance (misalnya lewat Redis pub/sub).

---

## Notes Postgres/DB

Tidak langsung. Broadcast presence saat connect/disconnect membaca `dm_conversations` (`GetConversationPeerIds`) untuk tahu peer mana saja yang perlu diberi tahu.

---

## Prerequisites

Caller harus mengirim access token yang valid dan belum expired sebagai query parameter `token`, dan request HTTP yang connect harus benar-benar upgrade WebSocket asli.

---

## Validasi Request

Query parameter:

| Field    | Tipe   | Wajib | Aturan                              |
| -------- | ------ | ----- | -------------------------------------- |
| `token`  | string | ya    | Access token valid (aturan validitas sama seperti header `Authorization` di endpoint REST) |

Batasan koneksi:

| Batasan                       | Nilai  | Perilaku kalau terlampaui                                     |
| -------------------------------- | ------ | ---------------------------------------------------------------- |
| Maks koneksi bersamaan/user      | 5      | Koneksi baru dikirimi frame error `WS_CONN_LIMIT` lalu langsung ditutup |
| Idle timeout                    | 60 detik | Koneksi diputus kalau tidak ada Pong dalam 60 detik sejak ping terakhir |
| Interval ping                   | 54 detik | Server kirim Ping frame sesering ini buat jaga koneksi tetap hidup |
| Ukuran maks frame inbound         | 8 KB   | Frame lebih besar akan menutup koneksi                             |

---

## Server → Client Events

| `type`         | Payload                                                    | Dikirim saat                                                             |
| ---------------- | ----------------------------------------------------------- | -------------------------------------------------------------------------- |
| `message.new`   | `DmMessageResponse` (lihat `send_dm_message.md`)             | Peer mengirim pesan baru di conversation bersama                            |
| `message.read`  | `{conversationId, userId, lastReadAt}`                       | Peer menandai conversation bersama sebagai sudah dibaca (lihat `mark_conversation_read.md`) |
| `typing`        | `{conversationId, userId, isTyping}`                         | Peer mengirim frame `typing` client → server (diteruskan, rate-limited 1/detik per conversation) |
| `presence`      | `{userId, online}`                                           | Peer DM connect (`online: true`) atau koneksi terakhirnya putus (`online: false`) |
| `error`         | `{code: "WS_CONN_LIMIT", message: "too many connections"}`  | Persis sebelum koneksi ditutup, kalau batas koneksi per-user terlampaui       |

## Client → Server Frames

Saat ini cuma satu tipe frame inbound yang ditangani; selain itu diabaikan diam-diam.

| `type`     | Payload                                     | Efek                                                        |
| ------------ | ----------------------------------------------- | ------------------------------------------------------------------ |
| `typing`   | `{conversationId, isTyping}`                    | Diteruskan ke participant lain di conversation itu sebagai event `typing`, maksimal sekali per detik per pasangan `(userId, conversationId)` |

---

## Response

### Saat upgrade berhasil

Handshake WebSocket 101 Switching Protocols standar; tidak ada body JSON.

### 401 Unauthorized

| `error_message`                    | Penyebab                                  |
| -------------------------------------- | -------------------------------------------- |
| `Missing token query parameter`       | Query parameter `token` tidak ada               |
| (standard auth error)                 | Token invalid, expired, atau sudah di-revoke     |

### 426 Upgrade Required

Dikembalikan kalau request mencapai route ini tanpa benar-benar upgrade WebSocket (misalnya GET biasa dari browser).

---

## Update

Dokumentasi ini diupdate tanggal 20 Juli 2026.
