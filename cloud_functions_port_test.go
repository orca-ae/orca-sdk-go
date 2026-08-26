// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"net/http"
	"testing"

	util "github.com/apache/pulsar-client-go/pulsaradmin/pkg/utils"
)

// Ported from orca-sdk-typescript tests/api-resources/cloud/functions.test.ts.
//
// Function names and instance IDs both reach the wire percent-encoded, so
// "trans/form" and "instance/0" each stay one path segment, and the ":start",
// ":stop", ":restart", ":trigger" action suffixes hang off the encoded name.

// cloudFunctionOperations is the functions half of the cloud contract table.
func cloudFunctionOperations() []cloudOperation {
	const (
		collection   = "/apis/cloud.sn.io/v1/functions"
		functionPath = collection + "/trans%2Fform"
		name         = "trans/form"
		instanceID   = "instance/0"
		stateKey     = "offset/key"
	)
	config := RegistryFunctionConfig{FunctionConfig: util.FunctionConfig{Name: name}}
	numberValue := int64(10)

	return []cloudOperation{
		{
			operationID: "registerFunctionWithDefaults",
			name:        "create",
			method:      "POST",
			path:        functionPath,
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).Create(ctx, config, "", "function://transform@latest")
			},
		},
		{
			operationID: "getFunctionInfoWithDefaults",
			name:        "retrieve",
			method:      "GET",
			path:        functionPath,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewFunctionsClient(client).Get(ctx, name)
				return err
			},
		},
		{
			operationID: "updateFunctionWithDefaults",
			name:        "update",
			method:      "PUT",
			path:        functionPath,
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).Update(ctx, name, config, "", "", nil)
			},
		},
		{
			operationID: "deregisterFunctionWithDefaults",
			name:        "delete",
			method:      "DELETE",
			path:        functionPath,
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).Delete(ctx, name)
			},
		},
		{
			operationID: "getFunctionInstanceStatsWithDefaults",
			name:        "retrieveInstanceStats",
			method:      "GET",
			path:        functionPath + "/instance%2F0/stats",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewFunctionsClient(client).InstanceStats(ctx, name, instanceID)
				return err
			},
		},
		{
			operationID: "getFunctionInstanceStatusWithDefaults",
			name:        "retrieveInstanceStatus",
			method:      "GET",
			path:        functionPath + "/instance%2F0/status",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewFunctionsClient(client).InstanceStatus(ctx, name, instanceID)
				return err
			},
		},
		{
			operationID: "getFunctionStateWithDefaults",
			name:        "retrieveState",
			method:      "GET",
			path:        functionPath + "/state/offset%2Fkey",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewFunctionsClient(client).GetState(ctx, name, stateKey)
				return err
			},
		},
		{
			operationID: "putFunctionStateWithDefaults",
			name:        "updateState",
			method:      "POST",
			path:        functionPath + "/state/offset%2Fkey",
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).PutState(ctx, name, stateKey, FunctionState{
					Key:         stateKey,
					NumberValue: &numberValue,
				})
			},
		},
		{
			operationID: "getFunctionStatsWithDefaults",
			name:        "retrieveStats",
			method:      "GET",
			path:        functionPath + "/stats",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewFunctionsClient(client).Stats(ctx, name)
				return err
			},
		},
		{
			operationID: "getFunctionStatusWithDefaults",
			name:        "retrieveStatus",
			method:      "GET",
			path:        functionPath + "/status",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewFunctionsClient(client).Status(ctx, name)
				return err
			},
		},
		{
			operationID: "listFunctionsWithDefaults",
			name:        "list",
			method:      "GET",
			path:        collection,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewFunctionsClient(client).List(ctx)
				return err
			},
		},
		{
			operationID: "restartFunctionAllWithDefaults",
			name:        "restart",
			method:      "POST",
			path:        functionPath + ":restart",
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).Restart(ctx, name)
			},
		},
		{
			operationID: "restartFunctionWithDefaults",
			name:        "restartInstance",
			method:      "POST",
			path:        functionPath + "/instance%2F0:restart",
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).RestartInstance(ctx, name, instanceID)
			},
		},
		{
			operationID: "startFunctionAllWithDefaults",
			name:        "start",
			method:      "POST",
			path:        functionPath + ":start",
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).Start(ctx, name)
			},
		},
		{
			operationID: "startFunctionWithDefaults",
			name:        "startInstance",
			method:      "POST",
			path:        functionPath + "/instance%2F0:start",
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).StartInstance(ctx, name, instanceID)
			},
		},
		{
			operationID: "stopFunctionAllWithDefaults",
			name:        "stop",
			method:      "POST",
			path:        functionPath + ":stop",
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).Stop(ctx, name)
			},
		},
		{
			operationID: "stopFunctionWithDefaults",
			name:        "stopInstance",
			method:      "POST",
			path:        functionPath + "/instance%2F0:stop",
			invoke: func(ctx context.Context, client *Client) error {
				return NewFunctionsClient(client).StopInstance(ctx, name, instanceID)
			},
		},
		{
			operationID: "triggerFunctionWithDefaults",
			name:        "trigger",
			method:      "POST",
			path:        functionPath + ":trigger",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewFunctionsClient(client).Trigger(ctx, name, "hello", "", "")
				return err
			},
		},
	}
}

func TestCloudFunctionsOperations(t *testing.T) {
	t.Parallel()
	cloudRunOperations(t, cloudFunctionOperations())
}

// TestCloudFunctionsMultipartBodies ports "encodes function create, update,
// state, and trigger bodies as multipart forms".
//
// One divergence is asserted rather than hidden: the TypeScript update sends
// only the parts the caller supplied, so an update carrying just
// updateOptions produces a one-part form. This SDK's Update always writes the
// functionConfig part too, because the multipart helper requires a config
// field, so the same call produces two parts.
func TestCloudFunctionsMultipartBodies(t *testing.T) {
	t.Parallel()

	t.Run("create sends functionConfig", func(t *testing.T) {
		t.Parallel()

		client, transport := cloudTestClient(t)
		err := NewFunctionsClient(client).Create(context.Background(), RegistryFunctionConfig{
			FunctionConfig: util.FunctionConfig{Name: "transform"},
			Connection:     "events",
		}, "", "")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		fields, files := cloudDecodeMultipart(t, transport.Only(t))
		if len(files) != 0 {
			t.Errorf("file parts = %v, want none when no package file is supplied", cloudFileNames(files))
		}
		if len(fields) != 1 {
			t.Errorf("parts = %v, want only functionConfig", cloudFieldNames(fields))
		}
		cloudAssertJSONFieldContains(t, fields, "functionConfig", map[string]any{
			"name": "transform", "connection": "events",
		})

		// Divergence, asserted so it cannot change unnoticed: the TypeScript
		// SDK sends exactly the JSON the caller passed, while the embedded
		// pulsaradmin FunctionConfig leaves its booleans untagged, so this SDK
		// also sends every one of them at its zero value.
		config := cloudDecodeJSONField(t, fields, "functionConfig")
		for _, defaulted := range []string{"autoAck", "retainOrdering", "skipToLatest"} {
			if _, ok := config[defaulted]; !ok {
				t.Errorf("functionConfig has no %q; this SDK sends the defaulted booleans "+
					"the caller never set - update this test if that changes", defaulted)
			}
		}
	})

	t.Run("update sends functionConfig and updateOptions", func(t *testing.T) {
		t.Parallel()

		client, transport := cloudTestClient(t)
		err := NewFunctionsClient(client).Update(context.Background(), "transform",
			RegistryFunctionConfig{FunctionConfig: util.FunctionConfig{Name: "transform"}},
			"", "", &UpdateOptionsImpl{UpdateAuthData: true})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		fields, _ := cloudDecodeMultipart(t, transport.Only(t))
		cloudAssertJSONField(t, fields, "updateOptions", map[string]any{"updateAuthData": true})
		// The TypeScript SDK omits functionConfig here; this one always sends it.
		cloudAssertJSONFieldContains(t, fields, "functionConfig", map[string]any{"name": "transform"})
	})

	t.Run("updateState sends state", func(t *testing.T) {
		t.Parallel()

		numberValue := int64(1)
		client, transport := cloudTestClient(t)
		err := NewFunctionsClient(client).PutState(context.Background(), "transform", "offset",
			FunctionState{Key: "offset", NumberValue: &numberValue})
		if err != nil {
			t.Fatalf("PutState() error = %v", err)
		}

		fields, _ := cloudDecodeMultipart(t, transport.Only(t))
		if len(fields) != 1 {
			t.Errorf("parts = %v, want only state", cloudFieldNames(fields))
		}
		cloudAssertJSONField(t, fields, "state", map[string]any{"key": "offset", "numberValue": 1})
	})

	t.Run("trigger sends the data field", func(t *testing.T) {
		t.Parallel()

		client, transport := cloudTestClient(t)
		if _, err := NewFunctionsClient(client).Trigger(
			context.Background(), "transform", "hello", "", "",
		); err != nil {
			t.Fatalf("Trigger() error = %v", err)
		}

		fields, _ := cloudDecodeMultipart(t, transport.Only(t))
		if got := fields["data"]; got != "hello" {
			t.Errorf("data = %q, want %q (parts: %v)", got, "hello", cloudFieldNames(fields))
		}
	})
}

// TestCloudFunctionsTriggerAcceptsAnEmptyResponse mirrors the TypeScript
// expectation that a trigger against a function with no output topic still
// succeeds - the server answers with an empty body once the input message has
// been published.
func TestCloudFunctionsTriggerAcceptsAnEmptyResponse(t *testing.T) {
	t.Parallel()

	client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, ""), nil
	})

	output, err := NewFunctionsClient(client).Trigger(context.Background(), "transform", "hello", "", "")
	if err != nil {
		t.Fatalf("Trigger() error = %v", err)
	}
	if output != "" {
		t.Errorf("output = %q, want the empty string", output)
	}
}

// TestCloudFunctionsPreservesRequestHeaders ports "preserves Headers instances
// for discovery and void operations".
func TestCloudFunctionsPreservesRequestHeaders(t *testing.T) {
	t.Parallel()
	t.Skip("not implemented: per-request options (caller-supplied headers, per-call defaultBaseURL); " +
		"this SDK has no options argument, only Client.WithDefaultHeader on the whole client")
}

// TestCloudFunctionsGating ports "gates %s before the function API request".
func TestCloudFunctionsGating(t *testing.T) {
	t.Parallel()
	t.Skip(cloudGatingUnimplemented)
}
