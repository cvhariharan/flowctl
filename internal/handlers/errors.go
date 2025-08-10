package handlers

import (
	"net/http"
	"runtime"
	"strings"

	"github.com/labstack/echo/v4"
)

// Simplified error codes for standardized API responses
const (
	// Validation errors (400 Bad Request)
	ErrValidationFailed     = "VALIDATION_FAILED"
	ErrRequiredFieldMissing = "REQUIRED_FIELD_MISSING"
	ErrInvalidInput         = "INVALID_INPUT"
	ErrInvalidPagination    = "INVALID_PAGINATION"

	// Authentication errors (401 Unauthorized)
	ErrAuthenticationFailed = "AUTHENTICATION_FAILED"
	ErrInvalidCredentials   = "INVALID_CREDENTIALS"

	// Authorization errors (403 Forbidden)
	ErrUnauthorized = "UNAUTHORIZED"
	ErrForbidden    = "FORBIDDEN"

	// Not found errors (404)
	ErrResourceNotFound = "RESOURCE_NOT_FOUND"

	// Server errors (500)
	ErrOperationFailed = "OPERATION_FAILED"
	ErrInternalError   = "INTERNAL_ERROR"
)

// Map error codes to HTTP status codes
var errorCodeToHTTPStatus = map[string]int{
	// Validation errors (400 Bad Request)
	ErrValidationFailed:     http.StatusBadRequest,
	ErrRequiredFieldMissing: http.StatusBadRequest,
	ErrInvalidInput:         http.StatusBadRequest,
	ErrInvalidPagination:    http.StatusBadRequest,

	// Authentication errors (401 Unauthorized)
	ErrAuthenticationFailed: http.StatusUnauthorized,
	ErrInvalidCredentials:   http.StatusUnauthorized,

	// Authorization errors (403 Forbidden)
	ErrUnauthorized: http.StatusForbidden,
	ErrForbidden:    http.StatusForbidden,

	// Not found errors (404)
	ErrResourceNotFound: http.StatusNotFound,

	// Server errors (500)
	ErrOperationFailed: http.StatusInternalServerError,
	ErrInternalError:   http.StatusInternalServerError,
}

type HTTPError struct {
	code      int
	msg       string
	err       error
	file      string
	line      int
	errorCode string
	details   interface{}
}

func (h *HTTPError) Error() string {
	return h.err.Error()
}

func wrapError(errorCode string, msg string, err error, details interface{}) error {
	// Look up HTTP status code from error code
	code, exists := errorCodeToHTTPStatus[errorCode]
	if !exists {
		// Default to internal server error for unknown error codes
		code = http.StatusInternalServerError
	}

	he := &HTTPError{
		code:      code,
		msg:       msg,
		err:       err,
		file:      "unknown",
		line:      -1,
		errorCode: errorCode,
		details:   details,
	}
	_, f, l, ok := runtime.Caller(1)
	if ok {
		he.file = f
		he.line = l
	}

	return he
}

func (h *Handler) ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	file := "unknown"
	line := -1
	msg := "error processing the request"
	errorCode := ErrInternalError
	var details interface{}

	if he, ok := err.(*HTTPError); ok {
		code = he.code
		msg = he.msg
		err = he.err
		file = he.file
		line = he.line
		errorCode = he.errorCode
		details = he.details
	}

	h.logger.Error("error processing request",
		"status", code,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"error", err,
		"msg", msg,
		"error_code", errorCode,
		"file", file,
		"line", line,
		"remote_ip", c.RealIP())

	if strings.Contains(c.Request().URL.Path, "/view") {
		c.Render(code, "error_page", struct {
			ErrorCode int
			Message   string
		}{
			ErrorCode: code,
			Message:   msg,
		})
	} else {
		response := map[string]interface{}{
			"error": msg,
			"code":  errorCode,
		}
		if details != nil {
			response["details"] = details
		}
		c.JSON(code, response)
	}
}
