Mengembalikan daftar member server dengan role masing-masing. Hanya bisa diakses oleh member server (cegah enumerasi roster server private). Hasil diurutkan berdasarkan waktu bergabung (oldest first) dengan cursor pagination.

**Siapa bisa mengakses:** Member server mana pun (Owner, Admin, Member).

**Parameter query:**
- `limit` — jumlah item per halaman (default 10, max 20)
- `cursor` — cursor untuk halaman berikutnya (dari respons sebelumnya)

**Response:**
```json
{
  "data": [
    {
      "userId": "uuid",
      "role": "Owner",
      "nickname": "GamerX",
      "username": "johndoe",
      "avatarUrl": "http://...",
      "joinedAt": "2024-01-15T10:30:00Z"
    }
  ],
  "page": {
    "nextCursor": "base64-encoded-cursor"
  }
}
```
