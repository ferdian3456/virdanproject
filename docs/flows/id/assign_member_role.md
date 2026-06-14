Mengubah role member antara Admin dan Member. Hanya Owner yang bisa melakukan ini.

**Body:**
```json
{ "role": "Admin" }
```
Nilai yang valid: `"Admin"` atau `"Member"`. Tidak bisa assign `"Owner"` melalui endpoint ini (gunakan transfer_ownership).

**Aturan:**
- Hanya Owner yang bisa assign role
- Tidak bisa mengubah role diri sendiri
- Target wajib sudah member server ini

**Response sukses:** `{"status": "OK"}`
