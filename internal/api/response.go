package api

// SuccessResponse represents a successful API response
type SuccessResponse struct {
	Ok   bool        `json:"ok"`
	Data interface{} `json:"data"`
}

// ErrorResponse represents an error API response
type ErrorResponse struct {
	Ok        bool   `json:"ok"`
	ErrorCode string `json:"errorCode"`
}
