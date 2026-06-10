package constant

const MAX_IMAGE_SIZE = 5 * 1024 * 1024   // 5MB
const MAX_VIDEO_SIZE = 100 * 1024 * 1024 // 100MB
const MAX_VIDEO_DURATION = 180           // 3 menit (detik)
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

