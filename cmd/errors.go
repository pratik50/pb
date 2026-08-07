package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
)

type ErrorCode string

const (
	// ErrorInvalidInput indicates invalid command arguments or request input.
	ErrorInvalidInput ErrorCode = "INVALID_INPUT"
	// ErrorAuthFailed indicates failed authentication.
	ErrorAuthFailed ErrorCode = "AUTH_FAILED"
	// ErrorPermissionDenied indicates insufficient authorization.
	ErrorPermissionDenied ErrorCode = "PERMISSION_DENIED"
	// ErrorNotFound indicates a missing resource.
	ErrorNotFound ErrorCode = "NOT_FOUND"
	// ErrorConflict indicates a resource state conflict.
	ErrorConflict ErrorCode = "CONFLICT"
	// ErrorRateLimited indicates server-side rate limiting.
	ErrorRateLimited ErrorCode = "RATE_LIMITED"
	// ErrorConnectionFailed indicates a network connection failure.
	ErrorConnectionFailed ErrorCode = "CONNECTION_FAILED"
	// ErrorTimeout indicates that an operation timed out.
	ErrorTimeout ErrorCode = "TIMEOUT"
	// ErrorInvalidResponse indicates malformed server output.
	ErrorInvalidResponse ErrorCode = "INVALID_RESPONSE"
	// ErrorServer indicates a server-side failure.
	ErrorServer ErrorCode = "SERVER_ERROR"
	// ErrorCommandFailed indicates an otherwise unclassified command failure.
	ErrorCommandFailed ErrorCode = "COMMAND_FAILED"
)

type CLIError struct {
	Code          ErrorCode
	Message       string
	PublicMessage string
	HTTPStatus    int
	Retryable     bool
	Cause         error
}

func (err *CLIError) Error() string {
	return err.Message
}

func (err *CLIError) Unwrap() error {
	return err.Cause
}

func newCLIError(code ErrorCode, message string, cause error) *CLIError {
	return &CLIError{Code: code, Message: message, PublicMessage: message, Cause: cause}
}

type errorDetail struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	HTTPStatus int       `json:"http_status,omitempty"`
	Retryable  bool      `json:"retryable"`
}

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

func WriteErrorJSON(out io.Writer, err error) error {
	return writeJSON(out, errorEnvelope{Error: errorDetails(err)})
}

func errorDetails(err error) errorDetail {
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		message := cliErr.PublicMessage
		if message == "" {
			message = cliErr.Message
		}
		return errorDetail{
			Code:       cliErr.Code,
			Message:    message,
			HTTPStatus: cliErr.HTTPStatus,
			Retryable:  cliErr.Retryable,
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errorDetail{Code: ErrorTimeout, Message: err.Error(), Retryable: true}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return errorDetail{Code: ErrorNotFound, Message: err.Error(), Retryable: false}
	}
	if errors.Is(err, fs.ErrPermission) {
		return errorDetail{Code: ErrorPermissionDenied, Message: err.Error(), Retryable: false}
	}
	var statusErr interface{ HTTPStatusCode() int }
	if errors.As(err, &statusErr) {
		statusCode := statusErr.HTTPStatusCode()
		code, retryable := httpErrorCode(statusCode)
		return errorDetail{
			Code:       code,
			Message:    fmt.Sprintf("request failed: HTTP %d", statusCode),
			HTTPStatus: statusCode,
			Retryable:  retryable,
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return errorDetail{Code: ErrorTimeout, Message: err.Error(), Retryable: true}
		}
		return errorDetail{Code: ErrorConnectionFailed, Message: err.Error(), Retryable: true}
	}
	return errorDetail{Code: ErrorCommandFailed, Message: err.Error(), Retryable: false}
}

func responseStatusError(action string, statusCode int, status string, body []byte) error {
	detail := strings.TrimSpace(string(body))
	legacyMessage := fmt.Sprintf("%s failed: %s", action, status)
	if detail != "" {
		legacyMessage += ": " + detail
	}

	return httpStatusCLIError(
		statusCode,
		legacyMessage,
		fmt.Sprintf("%s failed: %s", action, status),
	)
}

func httpStatusCLIError(statusCode int, legacyMessage, publicMessage string) error {
	code, retryable := httpErrorCode(statusCode)
	return &CLIError{
		Code:          code,
		Message:       legacyMessage,
		PublicMessage: publicMessage,
		HTTPStatus:    statusCode,
		Retryable:     retryable,
	}
}

func httpErrorCode(statusCode int) (ErrorCode, bool) {
	switch statusCode {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ErrorInvalidInput, false
	case http.StatusUnauthorized:
		return ErrorAuthFailed, false
	case http.StatusForbidden:
		return ErrorPermissionDenied, false
	case http.StatusNotFound:
		return ErrorNotFound, false
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return ErrorTimeout, true
	case http.StatusConflict:
		return ErrorConflict, false
	case http.StatusTooManyRequests:
		return ErrorRateLimited, true
	default:
		if statusCode >= http.StatusInternalServerError {
			return ErrorServer, true
		}
		return ErrorCommandFailed, false
	}
}

type renderedError struct {
	err error
}

func (err renderedError) Error() string { return err.err.Error() }
func (err renderedError) Unwrap() error { return err.err }

func MarkErrorRendered(err error) error {
	if err == nil {
		return nil
	}
	return renderedError{err: err}
}

func ErrorWasRendered(err error) bool {
	var rendered renderedError
	return errors.As(err, &rendered)
}
