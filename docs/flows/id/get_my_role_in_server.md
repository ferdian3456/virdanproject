Mengembalikan nama role caller di server tertentu. Digunakan oleh client untuk menentukan apakah user bisa melihat tombol moderasi (Owner/Admin bisa hapus post orang lain, kick member, dll).

**Response:**
```json
{ "role": "Owner" }
```

Nilai `role` yang mungkin: `"Owner"`, `"Admin"`, `"Member"`. Jika bukan member, endpoint ini mengembalikan 403.
