package model

type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

// UnauthorizedError represents an authentication/authorization error (401)
type UnauthorizedError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
}

func (e *UnauthorizedError) Error() string {
	return e.Message
}

// NotFoundError represents a resource not found error (404)
type NotFoundError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
}

func (e *NotFoundError) Error() string {
	return e.Message
}
