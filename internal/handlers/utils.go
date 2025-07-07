package handlers

import (
	"net/http"
	"runtime"

	"github.com/labstack/echo/v4"
)

type HTTPError struct {
	code           int
	msg            string
	err            error
	file           string
	line           int
	customResponse interface{}
}

func (h *HTTPError) Error() string {
	return h.err.Error()
}

func wrapError(code int, msg string, err error, customResponse interface{}) error {
	he := &HTTPError{
		code:           code,
		msg:            msg,
		err:            err,
		file:           "unknown",
		line:           -1,
		customResponse: customResponse,
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
	var customResponse interface{}
	if he, ok := err.(*HTTPError); ok {
		code = he.code
		msg = he.msg
		err = he.err
		file = he.file
		line = he.line
		customResponse = he.customResponse
	}

	h.logger.Error("error processing request",
		"status", code,
		"path", c.Request().URL.Path,
		"method", c.Request().Method,
		"error", err,
		"msg", msg,
		"file", file,
		"line", line,
		"remote_ip", c.RealIP())

	if customResponse != nil {
		c.JSON(code, customResponse)
	} else {
		c.JSON(code, map[string]string{
			"error": msg,
		})
	}
}
