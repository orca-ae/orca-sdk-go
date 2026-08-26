// Copyright (c) 2026 StreamNative, Inc.. All Rights Reserved.

package orca

import (
	"context"
	"io"
	"net/url"
)

// PackagesClient provides registry operations for workspace packages.
type PackagesClient struct {
	client *Client
}

// NewPackagesClient creates a workspace packages client. Packages are a StreamNative Cloud
// extension, served under CloudExtensionBasePath.
func NewPackagesClient(client *Client) *PackagesClient {
	return &PackagesClient{client: client.WithPathPrefix(CloudExtensionBasePath)}
}

// List returns all package names for the given package type.
func (c *PackagesClient) List(ctx context.Context, packageType string) ([]string, error) {
	var result []string
	if err := c.client.GetJSON(ctx, "/packages/"+url.PathEscape(packageType), &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ListVersions returns all versions for the given package.
func (c *PackagesClient) ListVersions(ctx context.Context, packageType, packageName string) ([]string, error) {
	var result []string
	if err := c.client.GetJSON(
		ctx,
		"/packages/"+url.PathEscape(packageType)+"/"+url.PathEscape(packageName),
		&result,
	); err != nil {
		return nil, err
	}

	return result, nil
}

// GetMetadata returns the metadata for one package version.
func (c *PackagesClient) GetMetadata(ctx context.Context, packageType, packageName, version string) (*PackageMetadata, error) {
	var result PackageMetadata
	if err := c.client.GetJSON(
		ctx,
		"/packages/"+url.PathEscape(packageType)+"/"+url.PathEscape(packageName)+"/"+url.PathEscape(version)+"/metadata",
		&result,
	); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateMetadata updates metadata for one package version.
func (c *PackagesClient) UpdateMetadata(ctx context.Context, packageType, packageName, version string, metadata PackageMetadata) error {
	return c.client.PutJSON(
		ctx,
		"/packages/"+url.PathEscape(packageType)+"/"+url.PathEscape(packageName)+"/"+url.PathEscape(version)+"/metadata",
		metadata,
		nil,
	)
}

// Upload uploads one package version via multipart/form-data.
func (c *PackagesClient) Upload(ctx context.Context, packageType, packageName, version, filePath string, metadata PackageMetadata) error {
	file, err := multipartFileFromPathWithField(filePath, "file")
	if err != nil {
		return err
	}

	return c.client.PostMultipart(
		ctx,
		"/packages/"+url.PathEscape(packageType)+"/"+url.PathEscape(packageName)+"/"+url.PathEscape(version),
		MultipartRequest{
			File:        file,
			ConfigField: "metadata",
			Config:      metadata,
		},
	)
}

// Download streams one package version into the provided writer.
func (c *PackagesClient) Download(ctx context.Context, packageType, packageName, version string, writer io.Writer) error {
	return c.client.GetToWriter(
		ctx,
		"/packages/"+url.PathEscape(packageType)+"/"+url.PathEscape(packageName)+"/"+url.PathEscape(version),
		writer,
	)
}

// Delete removes one package version.
func (c *PackagesClient) Delete(ctx context.Context, packageType, packageName, version string) error {
	return c.client.Delete(
		ctx,
		"/packages/"+url.PathEscape(packageType)+"/"+url.PathEscape(packageName)+"/"+url.PathEscape(version),
	)
}
