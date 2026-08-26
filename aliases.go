// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import "github.com/orca-ae/orca-sdk-go/internal/apierror"

// Error is satisfied by every error this SDK returns - server responses,
// network failures, and arguments rejected before a request was sent. Use it to
// tell an SDK failure from one raised by surrounding code:
//
//	var orcaErr orca.Error
//	if errors.As(err, &orcaErr) { ... }
type Error = apierror.Error

// APIError is a request that reached the server and came back with a
// non-success status.
//
// Every status-specific error below wraps one, so a caller that only needs the
// status code can match this type and ignore the rest.
type APIError = apierror.APIError

// HTTPError is the former name of [APIError], kept so existing callers keep
// compiling. New code should use [APIError], or one of the status-specific
// types when it cares about a particular failure.
type HTTPError = apierror.APIError

// Status-specific API errors. Match these when the reaction differs by status -
// re-authenticating on [AuthenticationError], backing off on [RateLimitError],
// treating [NotFoundError] as an empty result.
type (
	BadRequestError          = apierror.BadRequestError
	AuthenticationError      = apierror.AuthenticationError
	PermissionDeniedError    = apierror.PermissionDeniedError
	NotFoundError            = apierror.NotFoundError
	ConflictError            = apierror.ConflictError
	UnprocessableEntityError = apierror.UnprocessableEntityError
	RateLimitError           = apierror.RateLimitError
	InternalServerError      = apierror.InternalServerError
)

// Failures that are not a server response.
type (
	// ValidationError is a request this SDK refused to send because an argument
	// was missing or malformed.
	ValidationError = apierror.ValidationError

	// ConnectionError is a request that never got a response.
	ConnectionError = apierror.ConnectionError

	// TimeoutError is a request that exceeded its deadline.
	TimeoutError = apierror.TimeoutError

	// UserAbortError is a request cancelled through its context by the caller.
	UserAbortError = apierror.UserAbortError

	// DecodeError is a response that could not be turned into the expected type.
	DecodeError = apierror.DecodeError
)
