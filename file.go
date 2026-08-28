// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"io"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// FileService manages uploaded files.
type FileService struct {
	client *Client
}

// File is an uploaded file.
type File struct {
	ID        string `json:"id"`
	Type      string `json:"type,omitzero"`
	Filename  string `json:"filename,omitzero"`
	MimeType  string `json:"mime_type,omitzero"`
	SizeBytes int64  `json:"size_bytes,omitzero"`
	CreatedAt string `json:"created_at,omitzero"`
}

// FileDeleted is the tombstone a delete returns.
type FileDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// FileUploadParams uploads a file.
//
// The content type travels on the uploaded part itself rather than as a
// separate form field: the multipart part already carries it, and a second
// declaration is one more thing that can disagree with the bytes.
type FileUploadParams struct {
	// Filename is the name to store the file under.
	Filename string

	// ContentType is the file's MIME type. When empty the server infers one.
	ContentType string

	// Content is the file's bytes.
	Content []byte
}

// FileListParams pages uploaded files.
//
// Files use ID cursors rather than page tokens. The overlay removes the
// scope_id and provider filters.
type FileListParams struct {
	Limit    param.Opt[int64]
	AfterID  param.Opt[string]
	BeforeID param.Opt[string]
}

// Upload uploads a file.
func (s FileService) Upload(ctx context.Context, params FileUploadParams, opts ...option.RequestOption) (*File, error) {
	if params.Filename == "" {
		return nil, apierror.Validationf("file name is required")
	}

	payload, err := s.client.PostMultipartWithResponse(ctx, "v1/files", MultipartRequest{
		File: &MultipartFile{
			FieldName:   "file",
			FileName:    params.Filename,
			ContentType: params.ContentType,
			Content:     params.Content,
		},
	}, opts...)
	if err != nil {
		return nil, err
	}

	var file File
	if err := decodeJSON(payload, &file); err != nil {
		return nil, &apierror.DecodeError{Method: "POST", URL: "v1/files", Err: err}
	}
	return &file, nil
}

// Get reads a file's metadata.
func (s FileService) Get(ctx context.Context, fileID string, opts ...option.RequestOption) (*File, error) {
	var file File
	if err := s.client.GetJSON(ctx, "v1/files/"+url.PathEscape(fileID), &file, opts...); err != nil {
		return nil, err
	}
	return &file, nil
}

// Download streams a file's raw content to writer.
func (s FileService) Download(ctx context.Context, fileID string, writer io.Writer, opts ...option.RequestOption) error {
	path := "v1/files/" + url.PathEscape(fileID) + "/content"
	return s.client.GetStream(ctx, path, "application/octet-stream", func(body io.Reader) error {
		_, err := io.Copy(writer, body)
		return err
	}, opts...)
}

// List returns a page of uploaded files.
func (s FileService) List(ctx context.Context, params FileListParams, opts ...option.RequestOption) (*pagination.PageCursor[File], error) {
	opts = appendIDCursorQuery(opts, params.Limit, params.AfterID, params.BeforeID)
	return ListPage[File](ctx, s.client, "v1/files", opts...)
}

// Delete permanently deletes a file and returns its tombstone.
func (s FileService) Delete(ctx context.Context, fileID string, opts ...option.RequestOption) (*FileDeleted, error) {
	var deleted FileDeleted
	if err := s.client.doJSON(ctx, "DELETE", "v1/files/"+url.PathEscape(fileID), nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}
