package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/openbuilders/batch-sender/internal/errors"
)

// WithMethod is a middleware that checks if the endpoint was called using a
// specific HTTP method and rejects it otherwise.
func WithMethod(next http.HandlerFunc, method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, fmt.Sprintf("Only %s method is allowed", method), http.StatusMethodNotAllowed)
			return
		}

		next.ServeHTTP(w, r)
	}
}

// WithJSONResponse wraps an APIHandler and handles JSON response formatting
func WithJSONResponse(handler APIHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Call the handler to get data or error
		data, err := handler(w, r)

		// Set the Content-Type header
		w.Header().Set("Content-Type", "application/json")

		if err != nil {
			var errorResponse *ErrorResponse
			switch e := err.(type) {
			case errors.ServiceError:
				errorResponse = &ErrorResponse{
					Ok:        false,
					ErrorCode: string(e.Code),
				}
				slog.Debug("ServiceError", "error", e, "stack", e.Err)
			default:
				errorResponse = &ErrorResponse{
					Ok:        false,
					ErrorCode: err.Error(),
				}
			}

			slog.Debug("API error", "error", err)

			// Encode and send the error response
			if err := json.NewEncoder(w).Encode(*errorResponse); err != nil {
				http.Error(w, `{"ok": false, "errorCode": "internal_error", "errorDescription": "Failed to encode error response"}`, http.StatusInternalServerError)
			}
			return
		}

		// Create the success response
		successResponse := SuccessResponse{
			Ok:   true,
			Data: data,
		}

		// Encode and send the success response
		if err := json.NewEncoder(w).Encode(successResponse); err != nil {
			http.Error(w, `{"ok": false, "errorCode": "internal_error", "errorDescription": "Failed to encode success response"}`, http.StatusInternalServerError)
			return
		}
	}
}
