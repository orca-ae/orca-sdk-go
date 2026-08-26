package orca

import (
	"context"
	"net/http"
	"testing"
)

func TestHealthClientEndpoints(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		path string
		call func(*HealthClient) (bool, error)
	}{
		{name: "health", path: "/apis/cloud.sn.io/v1/health", call: func(client *HealthClient) (bool, error) { return client.Health(context.Background()) }},
		{name: "ready", path: "/apis/cloud.sn.io/v1/health/ready", call: func(client *HealthClient) (bool, error) { return client.Ready(context.Background()) }},
		{name: "live", path: "/apis/cloud.sn.io/v1/health/live", call: func(client *HealthClient) (bool, error) { return client.Live(context.Background()) }},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := newCatalogTestClient(t, http.MethodGet, tc.path, `true`)
			got, err := tc.call(NewHealthClient(client))
			if err != nil || !got {
				t.Fatalf("got = %t, err = %v", got, err)
			}
		})
	}
}
