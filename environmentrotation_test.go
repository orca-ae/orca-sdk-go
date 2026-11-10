// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/orca-ae/orca-sdk-go/option"
)

func TestRotateKeyDefaultClientDoesNotRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != "POST" || r.URL.Path != "/v1/environments/env_a/rotate-key" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "0.001")
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()
	client, err := New(option.WithBaseURL(server.URL), option.WithAuthToken("fixture"), option.WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Environments.RotateKey(context.Background(), "env_a"); err == nil {
		t.Fatal("expected 504")
	}
	if attempts != 1 {
		t.Fatalf("attempts=%d, want 1", attempts)
	}
}
