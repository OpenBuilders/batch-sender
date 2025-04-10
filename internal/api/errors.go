package api

type APIErrorCode string

// APIError represents a custom error with a code and description
type APIError struct {
	Code APIErrorCode
}

// Implement the error interface for APIError
func (e *APIError) Error() string {
	return string(e.Code)
}
