package constant

// Avatar/banner image cap (unchanged — dipakai server avatar/banner & profile avatar).
const MAX_IMAGE_SIZE = 5 * 1024 * 1024 // 5MB

// Post upload size limits — free vs Virdan Plus (per-server, time-limited upgrade).
const MAX_IMAGE_SIZE_FREE = 10 * 1024 * 1024  // 10MB
const MAX_IMAGE_SIZE_PLUS = 100 * 1024 * 1024 // 100MB
const MAX_VIDEO_SIZE_FREE = 50 * 1024 * 1024  // 50MB
const MAX_VIDEO_SIZE_PLUS = 100 * 1024 * 1024 // 100MB

// Legacy alias (jangan dipakai untuk post; biarkan kalau ada referensi lama selain post).
const MAX_VIDEO_SIZE = 100 * 1024 * 1024 // 100MB

// Virdan Plus product (one-time payment via Xendit).
const PLUS_PRICE_IDR int64 = 50000 // base price (Rp)
const PLUS_TAX_PERCENT int64 = 11  // tax percent on base
const PLUS_DURATION_DAYS = 30      // active duration after payment success

const MAX_VIDEO_DURATION = 180 // 3 menit (detik)
const MAX_IMAGE_WIDTH = 1080
const MIN_ASPECT_RATIO = 0.8  // 4:5 portrait
const MAX_ASPECT_RATIO = 1.91 // landscape

const DEFAULT_LIMIT = 10
const MAX_LIMIT = 20
const MIN_SEARCH_QUERY_LENGTH = 2
const MAX_SEARCH_QUERY_LENGTH = 100
const DEFAULT_MAX_MEMORY int64 = 110 << 20 // 110MB (video bisa 100MB + overhead)
const PLATFORM_ANDROID = "android"
const PLATFORM_IOS = "ios"

// DEFAULT_USER_SETTINGS seeds users.settings at signup so notification preferences are explicit
// in the DB (opt-out model), not reliant on COALESCE at read time.
const DEFAULT_USER_SETTINGS = `{"notif_like":true,"notif_comment":true,"notif_reply":true}`

