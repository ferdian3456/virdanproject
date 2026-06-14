package constant

const (
	// Validation & Payload Errors (400)
	ERR_BAD_REQUEST_CODE                = "BAD_REQUEST_ERROR"
	ERR_VALIDATION_CODE                 = "VALIDATION_ERROR"
	ERR_INVALID_REQUEST_BODY_ERROR_CODE = "INVALID_REQUEST_BODY_ERROR"
	ERR_INVALID_CONTENT_TYPE_MESSAGE    = "Invalid Content-Type header. Endpoint requires multipart/form-data."
	ERR_INVALID_REQUEST_BODY_MESSAGE    = "The request is invalid or malformed"

	// Upload size enforcement (400) — FE branches on this code to show tailored message.
	ERR_UPLOAD_SIZE_EXCEEDED_CODE = "UPLOAD_SIZE_EXCEEDED_ERROR"

	// Server Errors (500)
	ERR_INTERNAL_SERVER_ERROR_CODE      = "INTERNAL_SERVER_ERROR"
	ERR_INTENRAL_SERVER_ERROR_MESSAGE   = "Something went wrong. If the problem persists, please contact support"
	
	// Auth Errors (401)
	ERR_UNAUTHORIZED_ERROR = "UNAUTHORIZED_ERROR"
	ERR_UNAUTHORIZED_CODE  = "UNAUTHORIZED_ERROR"
	// Not Found Errors (404)
	ERR_NOT_FOUND_ERROR = "NOT_FOUND_ERROR"
	ERR_NOT_FOUND_CODE  = "NOT_FOUND_ERROR"
	// Forbidden Errors (403)
	ERR_FORBIDDEN_CODE = "FORBIDDEN_ERROR"
	// Conflict Errors (409)
	ERR_DUPLICATE_CODE = "DUPLICATE_ERROR"
	ERR_CONFLICT_CODE  = "CONFLICT_ERROR"
)
