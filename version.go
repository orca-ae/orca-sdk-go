// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import "github.com/orca-ae/orca-sdk-go/internal"

// Version is this SDK's version.
//
// It is exported because a caller needs it in two places the SDK cannot reach:
// their own User-Agent when this client sits behind another service, and the
// version line of a bug report. The SDK sends it on every request as
// X-Orca-Client.
const Version = internal.Version
