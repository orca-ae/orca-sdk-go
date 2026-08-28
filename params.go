// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"strconv"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// Query-parameter helpers shared by the resource services.
//
// Every optional filter follows the same rule: a parameter the caller did not
// set must not appear in the query string at all. Sending `version=` or
// `limit=0` is not the same request as sending nothing, and the server is
// entitled to treat them differently.

func formatInt(value int64) string { return strconv.FormatInt(value, 10) }

func formatBool(value bool) string { return strconv.FormatBool(value) }

// appendListQuery adds the pagination parameters every page-token list shares.
func appendListQuery(opts []option.RequestOption, limit param.Opt[int64], page param.Opt[string]) []option.RequestOption {
	opts = appendIntQuery(opts, "limit", limit)
	return appendStringQuery(opts, "page", page)
}

// appendIDCursorQuery adds the pagination parameters an ID-cursor list uses.
// Only files and session files speak this dialect.
func appendIDCursorQuery(opts []option.RequestOption, limit param.Opt[int64], afterID, beforeID param.Opt[string]) []option.RequestOption {
	opts = appendIntQuery(opts, "limit", limit)
	opts = appendStringQuery(opts, "after_id", afterID)
	return appendStringQuery(opts, "before_id", beforeID)
}

func appendStringQuery(opts []option.RequestOption, key string, value param.Opt[string]) []option.RequestOption {
	if v, ok := value.Value(); ok {
		return append(opts, option.WithQuery(key, v))
	}
	return opts
}

func appendIntQuery(opts []option.RequestOption, key string, value param.Opt[int64]) []option.RequestOption {
	if v, ok := value.Value(); ok {
		return append(opts, option.WithQuery(key, formatInt(v)))
	}
	return opts
}

func appendBoolQuery(opts []option.RequestOption, key string, value param.Opt[bool]) []option.RequestOption {
	if v, ok := value.Value(); ok {
		return append(opts, option.WithQuery(key, formatBool(v)))
	}
	return opts
}

// appendEnumQuery adds a query parameter whose value is a string-kinded named
// type, so callers keep their enum types instead of converting at each site.
func appendEnumQuery[T ~string](opts []option.RequestOption, key string, value param.Opt[T]) []option.RequestOption {
	if v, ok := value.Value(); ok {
		return append(opts, option.WithQuery(key, string(v)))
	}
	return opts
}
