package model

// ErrorResponse is the common JSON envelope returned for API errors.
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// APIError contains a stable machine-readable code and a safe message.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewErrorResponse creates a common API error response.
func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{Error: APIError{Code: code, Message: message}}
}
