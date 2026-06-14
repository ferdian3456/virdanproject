Mengeluarkan member dari server (hard delete row server_members). Post lama member yang di-kick tetap ada di server.

**Aturan izin:**
- Owner bisa kick Admin dan Member
- Admin hanya bisa kick Member (tidak bisa kick Admin lain atau Owner)
- Member tidak bisa kick siapapun
- Tidak bisa kick diri sendiri (gunakan leave server)
- Tidak bisa kick Owner

**Response sukses:** `{"status": "OK"}`

**Error:**
- 400 — mencoba kick diri sendiri
- 403 — tidak punya izin, atau target adalah Owner
- 404 — target bukan member server ini
