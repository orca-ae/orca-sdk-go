// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/option"
)

func TestInterruptOverHTTPDoesNotRetry(t *testing.T) {
	for _, retries := range []int{2, 4} {
		for _, failure := range []string{"status", "transport", "timeout"} {
			t.Run(fmt.Sprintf("global-%d/%s", retries, failure), func(t *testing.T) {
				var attempts atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempts.Add(1)
					body, _ := io.ReadAll(r.Body)
					if r.Method != "POST" || r.URL.EscapedPath() != "/v1/sessions/ses%2Fa/threads/sth%2Fb/interrupt" || string(body) != `{}` || r.Header.Get("Authorization") != "Bearer fixture" || r.Header.Get("X-Test") != "interrupt" {
						t.Errorf("unexpected request %s %s body=%s headers=%v", r.Method, r.URL, body, r.Header)
					}
					if failure == "timeout" {
						<-r.Context().Done()
						return
					}
					if failure == "transport" {
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					}
					w.Header().Set("Retry-After", "0.001")
					w.WriteHeader(502)
				}))
				defer server.Close()
				delays := &[]time.Duration{}
				client, err := New(option.WithBaseURL(server.URL), option.WithAuthToken("fixture"), option.WithHTTPClient(server.Client()), option.WithMaxRetries(retries), withRecordedSleep(delays))
				if err != nil {
					t.Fatal(err)
				}
				opts := []option.RequestOption{option.WithHeader("X-Test", "interrupt")}
				if failure == "timeout" {
					opts = append(opts, option.WithRequestTimeout(50*time.Millisecond))
				}
				_, err = client.Sessions.Threads.Interrupt(context.Background(), "ses/a", "sth/b", opts...)
				if err == nil || attempts.Load() != 1 || len(*delays) != 0 {
					t.Fatalf("err=%v attempts=%d delays=%v", err, attempts.Load(), *delays)
				}
				var timeoutErr *TimeoutError
				if failure == "timeout" && !errors.As(err, &timeoutErr) {
					t.Fatalf("err=%v, want request TimeoutError", err)
				}
			})
		}
	}
}
