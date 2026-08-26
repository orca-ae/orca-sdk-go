// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"
)

// Ported from orca-sdk-typescript tests/api-resources/cloud/packages.test.ts.
//
// A package is addressed by the triple (type, name, version); the name is the
// only part that can contain a "/", and it must survive as one path segment.

// cloudPackageOperations is the packages half of the cloud contract table.
func cloudPackageOperations() []cloudOperation {
	const (
		collection  = "/apis/cloud.sn.io/v1/packages/function"
		versions    = collection + "/trans%2Fform"
		versioned   = versions + "/v1"
		packageType = "function"
		packageName = "trans/form"
		version     = "v1"
	)
	metadata := PackageMetadata{Description: "Transform"}

	return []cloudOperation{
		{
			operationID: "listPackages",
			name:        "list",
			method:      "GET",
			path:        collection,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewPackagesClient(client).List(ctx, packageType)
				return err
			},
		},
		{
			operationID: "listPackageVersion",
			name:        "listVersions",
			method:      "GET",
			path:        versions,
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewPackagesClient(client).ListVersions(ctx, packageType, packageName)
				return err
			},
		},
		{
			operationID: "download",
			name:        "download",
			method:      "GET",
			path:        versioned,
			invoke: func(ctx context.Context, client *Client) error {
				return NewPackagesClient(client).Download(ctx, packageType, packageName, version, io.Discard)
			},
		},
		{
			operationID: "upload",
			name:        "upload",
			method:      "POST",
			path:        versioned,
			invoke: func(ctx context.Context, client *Client) error {
				return NewPackagesClient(client).Upload(ctx, packageType, packageName, version, "", metadata)
			},
		},
		{
			operationID: "delete",
			name:        "delete",
			method:      "DELETE",
			path:        versioned,
			invoke: func(ctx context.Context, client *Client) error {
				return NewPackagesClient(client).Delete(ctx, packageType, packageName, version)
			},
		},
		{
			operationID: "getMeta",
			name:        "retrieveMetadata",
			method:      "GET",
			path:        versioned + "/metadata",
			invoke: func(ctx context.Context, client *Client) error {
				_, err := NewPackagesClient(client).GetMetadata(ctx, packageType, packageName, version)
				return err
			},
		},
		{
			operationID: "updateMeta",
			name:        "updateMetadata",
			method:      "PUT",
			path:        versioned + "/metadata",
			invoke: func(ctx context.Context, client *Client) error {
				return NewPackagesClient(client).UpdateMetadata(ctx, packageType, packageName, version, metadata)
			},
		},
	}
}

func TestCloudPackagesOperations(t *testing.T) {
	t.Parallel()
	cloudRunOperations(t, cloudPackageOperations())
}

// TestCloudPackagesUploadIsMultipart ports "encodes uploads as multipart form
// data": the metadata travels as an application/json part, and with no file
// supplied it is the only part.
func TestCloudPackagesUploadIsMultipart(t *testing.T) {
	t.Parallel()

	client, transport := cloudTestClient(t)
	err := NewPackagesClient(client).Upload(
		context.Background(), "function", "transform", "v1", "",
		PackageMetadata{Description: "Transform"},
	)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	fields, files := cloudDecodeMultipart(t, transport.Only(t))
	if len(files) != 0 {
		t.Errorf("file parts = %v, want none when no file is supplied", cloudFileNames(files))
	}
	if len(fields) != 1 {
		t.Errorf("parts = %v, want only metadata", cloudFieldNames(fields))
	}
	cloudAssertJSONField(t, fields, "metadata", map[string]any{"description": "Transform"})
}

// TestCloudPackagesUploadSendsTheFilePart asserts the other half of the upload
// contract: when a file is supplied it rides alongside the metadata part, under
// the field name "file".
func TestCloudPackagesUploadSendsTheFilePart(t *testing.T) {
	t.Parallel()

	filePath := cloudWriteTempFile(t, "package-payload")

	client, transport := cloudTestClient(t)
	err := NewPackagesClient(client).Upload(
		context.Background(), "function", "transform", "v1", filePath,
		PackageMetadata{Description: "Transform"},
	)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	fields, files := cloudDecodeMultipart(t, transport.Only(t))
	if got := string(files["file"]); got != "package-payload" {
		t.Errorf("file part = %q, want %q (file parts: %v)", got, "package-payload", cloudFileNames(files))
	}
	cloudAssertJSONField(t, fields, "metadata", map[string]any{"description": "Transform"})
}

// cloudWriteTempFile writes contents to a temp file and returns its path.
func cloudWriteTempFile(t *testing.T, contents string) string {
	t.Helper()

	path := t.TempDir() + "/payload.bin"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

// TestCloudPackagesDownloadStreamsTheRawBody asserts download does not try to
// decode the response: package payloads are opaque bytes, not JSON.
func TestCloudPackagesDownloadStreamsTheRawBody(t *testing.T) {
	t.Parallel()

	client, transport := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			Body:       io.NopCloser(bytes.NewReader([]byte{0x00, 0x01, 0x02})),
		}, nil
	})

	var payload bytes.Buffer
	if err := NewPackagesClient(client).Download(
		context.Background(), "function", "transform", "v1", &payload,
	); err != nil {
		t.Fatalf("Download() error = %v", err)
	}

	if got := payload.Bytes(); !bytes.Equal(got, []byte{0x00, 0x01, 0x02}) {
		t.Errorf("payload = %v, want the raw bytes", got)
	}
	if got := transport.Only(t).Header.Get("Accept"); got != "*/*" {
		t.Errorf("Accept = %q, want */* for an opaque download", got)
	}
}

// TestCloudPackagesMetadataResponseIsTyped records a divergence the TypeScript
// contract test asserts from the other direction: there, packages list,
// listVersions, and retrieveMetadata all resolve to `unknown` because the spec
// declares no response schema. This SDK types them anyway - []string, []string,
// and PackageMetadata - so callers get fields the contract does not promise.
func TestCloudPackagesMetadataResponseIsTyped(t *testing.T) {
	t.Parallel()

	client, _ := newRecordingClient(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"description":"Transform","properties":{"team":"data"}}`), nil
	})

	metadata, err := NewPackagesClient(client).GetMetadata(
		context.Background(), "function", "transform", "v1",
	)
	if err != nil {
		t.Fatalf("GetMetadata() error = %v", err)
	}
	if metadata.Description != "Transform" || metadata.Properties["team"] != "data" {
		t.Errorf("metadata = %#v, want the decoded description and properties", metadata)
	}
}

// TestCloudPackagesGating ports "gates %s before the package API request".
func TestCloudPackagesGating(t *testing.T) {
	t.Parallel()
	t.Skip(cloudGatingUnimplemented)
}
