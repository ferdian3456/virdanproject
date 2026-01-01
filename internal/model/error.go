package model

type ValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
}

func (e *ValidationError) Error() string {
	return e.Message
}
