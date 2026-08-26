// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderManagedAgentSSE(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(": comment\r\n" +
		"event: update\r\n" +
		"id: 1\r\n" +
		"data: {\"type\":\"agent.message\"}\r\n\r\n" +
		"data: plain\r\n\r\n")
	var output bytes.Buffer
	if err := renderManagedAgentSSE(&output, input); err != nil {
		t.Fatalf("renderManagedAgentSSE() error = %v", err)
	}
	got := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(got) != 2 {
		t.Fatalf("lines = %v", got)
	}
	if !strings.Contains(got[0], `"event":"update"`) || !strings.Contains(got[0], `"type":"agent.message"`) {
		t.Fatalf("first event = %q", got[0])
	}
	if got[1] != `"plain"` {
		t.Fatalf("second event = %q", got[1])
	}
}
