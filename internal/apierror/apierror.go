// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

// Package apierror defines every error this SDK returns.
//
// Two properties matter, and they pull in different directions:
//
//   - A caller should be able to catch any SDK failure without enumerating
//     types, so everything here satisfies [Error].
//   - A caller should be able to react to one specific failure - "the agent is
//     gone", "we are being rate limited" - without reading a status code out of
//     a generic error, so each meaningful status has its own type.
//
// Both work through the standard library. Every status-specific type wraps an
// [APIError], so errors.As finds either the specific type or the general one:
//
//	var notFound *orca.NotFoundError
//	if errors.As(err, &notFound) { ... }
//
//	var apiErr *orca.APIError
//	if errors.As(err, &apiErr) { log(apiErr.StatusCode) }
//
//	var anyErr orca.Error
//	if errors.As(err, &anyErr) { ... }   // including validation and decode failures
package apierror

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Error is satisfied by every error this SDK returns, whether it came from the
// server, from the network, or from argument validation before a request was
// ever sent. The unexported method keeps the interface closed: an error
// satisfying it is one of ours.
type Error interface {
	error
	isOrcaError()
}

// APIError is a request that reached the server and came back with a
// non-success status.
//
// It is also exported from the root package as HTTPError, the name this type
// had before the hierarchy existed.
type APIError struct {
	// Method and URL identify the request that failed. They are part of the
	// message because an error surfacing several layers up is otherwise
	// impossible to place.
	Method string
	URL    string

	// StatusCode is the HTTP status the server returned.
	StatusCode int

	// Body is the response body, trimmed. Kept as a string rather than parsed
	// because the error envelope is not uniform across the core and cloud
	// surfaces, and a caller debugging a failure wants what the server actually
	// said.
	Body string

	// RequestID is the server's correlation ID, when it sent one. Quote it in a
	// bug report: it is what ties this failure to the server-side logs.
	RequestID string

	// Header is the full response header set, so a caller can read Retry-After
	// or any deployment-specific diagnostic the SDK does not model.
	Header http.Header
}

func (e *APIError) isOrcaError() {}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s %s returned status %d", e.Method, e.URL, e.StatusCode)
	}
	return fmt.Sprintf("%s %s returned status %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

// BadRequestError is a 400: the request was malformed or failed validation.
type BadRequestError struct{ *APIError }

// AuthenticationError is a 401: the credential was missing, malformed, or
// rejected.
type AuthenticationError struct{ *APIError }

// PermissionDeniedError is a 403: the credential is valid but not authorized
// for this resource.
type PermissionDeniedError struct{ *APIError }

// NotFoundError is a 404. On a cloud extension path it may mean the resource is
// missing or that the deployment does not serve the extension at all - prefer
// discovery to tell those apart rather than inferring from the status.
type NotFoundError struct{ *APIError }

// ConflictError is a 409: the request lost an optimistic-concurrency check or
// collided with an existing resource.
type ConflictError struct{ *APIError }

// UnprocessableEntityError is a 422: the request parsed but the server refused
// its contents.
type UnprocessableEntityError struct{ *APIError }

// RateLimitError is a 429. Read Retry-After from Header before retrying.
type RateLimitError struct{ *APIError }

// InternalServerError is any 5xx.
type InternalServerError struct{ *APIError }

// Unwrap lets errors.As find the underlying [APIError] through any of the
// status-specific types, so a caller that only cares about the status code does
// not have to enumerate them.
func (e *BadRequestError) Unwrap() error          { return e.APIError }
func (e *AuthenticationError) Unwrap() error      { return e.APIError }
func (e *PermissionDeniedError) Unwrap() error    { return e.APIError }
func (e *NotFoundError) Unwrap() error            { return e.APIError }
func (e *ConflictError) Unwrap() error            { return e.APIError }
func (e *UnprocessableEntityError) Unwrap() error { return e.APIError }
func (e *RateLimitError) Unwrap() error           { return e.APIError }
func (e *InternalServerError) Unwrap() error      { return e.APIError }

// FromResponse builds the error for a failed request, choosing the
// status-specific type where one exists and returning a plain [APIError]
// otherwise. An unmapped status is not an error in itself - the server is
// allowed to use one this SDK has no opinion about.
func FromResponse(method, url string, status int, body string, header http.Header) error {
	base := &APIError{
		Method:     method,
		URL:        url,
		StatusCode: status,
		Body:       strings.TrimSpace(body),
		RequestID:  requestIDFrom(header),
		Header:     header,
	}

	switch {
	case status == http.StatusBadRequest:
		return &BadRequestError{base}
	case status == http.StatusUnauthorized:
		return &AuthenticationError{base}
	case status == http.StatusForbidden:
		return &PermissionDeniedError{base}
	case status == http.StatusNotFound:
		return &NotFoundError{base}
	case status == http.StatusConflict:
		return &ConflictError{base}
	case status == http.StatusUnprocessableEntity:
		return &UnprocessableEntityError{base}
	case status == http.StatusTooManyRequests:
		return &RateLimitError{base}
	case status >= 500:
		return &InternalServerError{base}
	default:
		return base
	}
}

// requestIDHeaders are the correlation-ID header names seen across the
// deployments this SDK talks to. They are checked in order.
var requestIDHeaders = []string{"Request-Id", "X-Request-Id", "X-Correlation-Id"}

func requestIDFrom(header http.Header) string {
	for _, name := range requestIDHeaders {
		if value := header.Get(name); value != "" {
			return value
		}
	}
	return ""
}

// ValidationError is a request this SDK refused to send, because an argument
// was missing or malformed. It never reached the network, so there is no status
// code and nothing to retry - the call itself has to change.
type ValidationError struct {
	Message string
	Err     error
}

func (e *ValidationError) isOrcaError()  {}
func (e *ValidationError) Error() string { return e.Message }
func (e *ValidationError) Unwrap() error { return e.Err }

// Validationf returns a [ValidationError]. It formats like [fmt.Errorf],
// including %w.
func Validationf(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	return &ValidationError{Message: err.Error(), Err: errors.Unwrap(err)}
}

// ConnectionError is a request that never got a response: DNS failure, refused
// connection, TLS failure, or a dropped socket.
type ConnectionError struct {
	Method string
	URL    string
	Err    error
}

func (e *ConnectionError) isOrcaError()  {}
func (e *ConnectionError) Unwrap() error { return e.Err }
func (e *ConnectionError) Error() string {
	return fmt.Sprintf("failed to execute %s %s: %v", e.Method, e.URL, e.Err)
}

// TimeoutError is a request that exceeded its deadline. It is reported
// separately from [ConnectionError] because it is the one network failure where
// retrying unchanged is usually wrong - the deadline, not the network, is what
// needs adjusting.
type TimeoutError struct {
	Method string
	URL    string
	Err    error
}

func (e *TimeoutError) isOrcaError()  {}
func (e *TimeoutError) Unwrap() error { return e.Err }
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("failed to execute %s %s: %v", e.Method, e.URL, e.Err)
}

// UserAbortError is a request cancelled through its context by the caller, as
// opposed to one that ran out of time on its own.
type UserAbortError struct {
	Method string
	URL    string
	Err    error
}

func (e *UserAbortError) isOrcaError()  {}
func (e *UserAbortError) Unwrap() error { return e.Err }
func (e *UserAbortError) Error() string {
	return fmt.Sprintf("failed to execute %s %s: %v", e.Method, e.URL, e.Err)
}

// FromTransport classifies a failure returned by [http.Client.Do], which
// reports a cancelled context, an exceeded deadline, and a refused connection
// through the same error value. ctx is the request's context, needed to tell a
// caller-driven cancellation from a timeout.
func FromTransport(ctx context.Context, method, url string, err error) error {
	switch {
	case ctx != nil && errors.Is(ctx.Err(), context.Canceled):
		return &UserAbortError{Method: method, URL: url, Err: err}
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		return &TimeoutError{Method: method, URL: url, Err: err}
	default:
		return &ConnectionError{Method: method, URL: url, Err: err}
	}
}

func isTimeout(err error) bool {
	var timeout interface{ Timeout() bool }
	return errors.As(err, &timeout) && timeout.Timeout()
}

// DecodeError is a response the SDK could not turn into the expected type -
// either the body was not valid JSON, or it did not match the shape the
// operation declares.
type DecodeError struct {
	Method string
	URL    string
	Err    error
}

func (e *DecodeError) isOrcaError()  {}
func (e *DecodeError) Unwrap() error { return e.Err }
func (e *DecodeError) Error() string {
	return fmt.Sprintf("failed to decode %s %s response: %v", e.Method, e.URL, e.Err)
}

// RequestError is a failure in the request/response plumbing that is neither a
// server status nor a rejected argument: marshalling a payload, building the
// HTTP request, writing a multipart part, reading a body.
//
// There is no useful way for a caller to branch on these individually, so they
// share one type. What matters is that they satisfy [Error] like everything
// else, so a caller catching SDK failures catches these too.
type RequestError struct {
	Message string
	Err     error
}

func (e *RequestError) isOrcaError()  {}
func (e *RequestError) Error() string { return e.Message }
func (e *RequestError) Unwrap() error { return e.Err }

// Errorf returns a [RequestError]. It formats like [fmt.Errorf], including %w,
// so it is a drop-in replacement at call sites whose message is already right.
func Errorf(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	return &RequestError{Message: err.Error(), Err: errors.Unwrap(err)}
}

// ExtensionNotAvailableError is a call to an API extension the deployment does
// not serve.
//
// It exists so this case is distinguishable from a resource that happens to be
// missing. Both would otherwise arrive as a 404 from a path the caller has no
// reason to doubt, and the difference matters: one means "create it", the other
// means "this deployment cannot do that at all".
type ExtensionNotAvailableError struct {
	// Group is the API group that was required, e.g. "cloud.sn.io".
	Group string

	// BaseURL is the deployment that was asked.
	BaseURL string

	// Reason says how the deployment answered - no extensions installed, a
	// different set of extensions, or no discovery endpoint at all.
	Reason string
}

func (e *ExtensionNotAvailableError) isOrcaError() {}

func (e *ExtensionNotAvailableError) Error() string {
	return fmt.Sprintf("the %q extension group is not available on %s: %s", e.Group, e.BaseURL, e.Reason)
}
