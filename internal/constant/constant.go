package constant

const MAX_FILE_SIZE = 5 * 1024 * 1024 // 5MB
const DEFAULT_LIMIT = 10
const MAX_LIMIT = 20
const DEFAULT_MAX_MEMORY int64 = 32 << 20 // 32MB
const PLATFORM_ANDROID = "android"
const PLATFORM_IOS = "ios"

// DEFAULT_USER_SETTINGS seeds users.settings at signup so notification preferences are explicit
// in the DB (opt-out model), not reliant on COALESCE at read time.
const DEFAULT_USER_SETTINGS = `{"notif_like":true,"notif_comment":true,"notif_reply":true}`
