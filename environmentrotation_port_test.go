// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package orca

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/orca-ae/orca-sdk-go/option"
)

func TestRotateKeyDoesNotRetryFailures(t *testing.T) {
	for _, retries := range []int{2, 4} {
		for _, failure := range []string{"status", "transport"} {
			t.Run(fmt.Sprintf("%s/global-%d", failure, retries), func(t *testing.T) {
				attempts := 0
				client, transport, delays := newRetryingClient(t, retries, func(r *http.Request) (*http.Response, error) {
					attempts++
					if attempts == 1 {
						if failure == "transport" {
							return nil, io.ErrUnexpectedEOF
						}
						return jsonResponse(504, `{"error":{"message":"timeout"}}`), nil
					}
					return jsonResponse(200, `{"env_key":"retry-key"}`), nil
				})
				_, err := client.Environments.RotateKey(context.Background(), "env/a", option.WithHeader("X-Test", "rotation"))
				if err == nil || len(transport.Calls()) != 1 || len(*delays) != 0 {
					t.Fatalf("err=%v attempts=%d delays=%v", err, len(transport.Calls()), *delays)
				}
				call := transport.Last(t)
				if call.Method != "POST" || call.Path() != "/v1/environments/env%2Fa/rotate-key" || call.Header.Get("X-Test") != "rotation" {
					t.Fatalf("request = %+v", call)
				}
				assertJSONBody(t, call, `{}`)
			})
		}
	}
}

func TestRotateKeyExplicitRetryOverrideAndReadRetry(t *testing.T) {
	for _, rotation := range []bool{false, true} {
		t.Run(fmt.Sprintf("rotation-%t", rotation), func(t *testing.T) {
			attempts := 0
			client, transport, _ := newRetryingClient(t, 2, func(r *http.Request) (*http.Response, error) {
				attempts++
				if attempts == 1 || (!rotation && attempts == 2) {
					return jsonResponse(504, `{}`), nil
				}
				return jsonResponse(200, `{"id":"env_a","env_key":"key"}`), nil
			})
			if rotation {
				result, err := client.Environments.RotateKey(context.Background(), "env_a", option.WithMaxRetries(1), option.WithHeader("Idempotency-Key", "explicit"))
				if err != nil || result.EnvKey != "key" {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				for _, call := range transport.Calls() {
					if call.Header.Get("Idempotency-Key") != "explicit" {
						t.Fatal("lost explicit header")
					}
				}
			} else {
				if _, err := client.Environments.RotateKey(context.Background(), "env_a"); err == nil {
					t.Fatal("expected rotation failure")
				}
				if _, err := client.Environments.Get(context.Background(), "env_a"); err != nil {
					t.Fatal(err)
				}
			}
			want := 2
			if !rotation {
				want = 3
			}
			if len(transport.Calls()) != want {
				t.Fatalf("attempts=%d", len(transport.Calls()))
			}
		})
	}
}
