# Search Server Posts

Mencari post di dalam satu server berdasarkan caption.

## Alur

1. Validasi `serverId` (UUID) dan `limit` (0..20).
2. Validasi `q`: di-trim, panjang 2..100 karakter. Kurang dari 2 -> 400.
3. Cek membership: requester wajib member server (kalau bukan -> 403).
4. Decode cursor (kalau ada). Cursor invalid -> 400.
5. Query post di server tersebut dengan `caption ILIKE '%q%'`, urut `created_at DESC, id DESC`, cursor-paginated.
6. Kembalikan `data` + `page.nextCursor`.

## Catatan

- Scope per active server saja (tidak lintas server).
- Hanya field caption yang dicari.
- Index `pg_trgm` (GIN) dipakai supaya ILIKE tidak seq-scan.
