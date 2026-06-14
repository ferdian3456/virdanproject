Memindahkan kepemilikan server ke member lain. Owner lama otomatis menjadi Admin. Operasi bersifat atomik (satu transaksi database).

**Body:**
```json
{ "newOwnerId": "uuid-of-new-owner" }
```

**Aturan:**
- Hanya Owner yang bisa transfer ownership
- Target wajib sudah member server ini
- Tidak bisa transfer ke diri sendiri

**Setelah transfer:**
- Target menjadi Owner (server_members.server_role_id → Owner role, servers.owner_id → target)
- Pemanggil menjadi Admin

**Response sukses:** `{"status": "OK"}`
