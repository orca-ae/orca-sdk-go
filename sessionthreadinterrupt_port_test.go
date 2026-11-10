// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/orca-ae/orca-sdk-go/option"
)

func TestInterruptDoesNotRetryFailures(t *testing.T) {
	for _, retries := range []int{2, 4} {
		for _, failure := range []string{"status", "transport", "timeout"} {
			t.Run(fmt.Sprintf("%s/global-%d", failure, retries), func(t *testing.T) {
				timeout := 5 * time.Second
				if failure == "timeout" {
					timeout = 10 * time.Millisecond
				}
				client, transport, delays := newRetryingClient(t, retries, func(r *http.Request) (*http.Response, error) {
					if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) > timeout {
						t.Error("lost per-call timeout")
					}
					switch failure {
					case "transport":
						return nil, io.ErrUnexpectedEOF
					case "timeout":
						<-r.Context().Done()
						return nil, r.Context().Err()
					default:
						return jsonResponse(502, `{}`), nil
					}
				})
				_, err := client.Sessions.Threads.Interrupt(context.Background(), "ses/a", "sth/b", option.WithHeader("X-Test", "interrupt"), option.WithRequestTimeout(timeout))
				if err == nil || len(transport.Calls()) != 1 || len(*delays) != 0 {
					t.Fatalf("err=%v attempts=%d delays=%v", err, len(transport.Calls()), *delays)
				}
				var timeoutErr *TimeoutError
				if failure == "timeout" && !errors.As(err, &timeoutErr) {
					t.Fatalf("err=%v, want request TimeoutError", err)
				}
				call := transport.Only(t)
				if call.Method != "POST" || call.Path() != "/v1/sessions/ses%2Fa/threads/sth%2Fb/interrupt" || call.Header.Get("X-Test") != "interrupt" || call.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatalf("request=%+v", call)
				}
				assertJSONBody(t, call, `{}`)
			})
		}
	}
}

func TestInterruptExplicitRetryOverrideAndReadRetry(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			attempts := 0
			client, transport, _ := newRetryingClient(t, 2, func(r *http.Request) (*http.Response, error) {
				attempts++
				if attempts <= 2 {
					return jsonResponse(502, `{}`), nil
				}
				return jsonResponse(200, `{"id":"sth_a"}`), nil
			})
			if explicit {
				_, err := client.Sessions.Threads.Interrupt(context.Background(), "ses_a", "sth_a", option.WithMaxRetries(1), option.WithHeader("X-Test", "explicit"))
				if err == nil || len(transport.Calls()) != 2 {
					t.Fatalf("err=%v attempts=%d", err, len(transport.Calls()))
				}
				for _, call := range transport.Calls() {
					if call.Header.Get("X-Test") != "explicit" {
						t.Fatal("lost explicit header")
					}
				}
			} else {
				if _, err := client.Sessions.Threads.Interrupt(context.Background(), "ses_a", "sth_a"); err == nil {
					t.Fatal("expected interrupt failure")
				}
			}
			result, err := client.Sessions.Threads.Get(context.Background(), "ses_a", "sth_a")
			if err != nil || result.ID != "sth_a" || len(transport.Calls()) != 3 {
				t.Fatalf("result=%+v err=%v attempts=%d", result, err, len(transport.Calls()))
			}
		})
	}
}
