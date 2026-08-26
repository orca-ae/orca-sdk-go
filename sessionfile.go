// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"io"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// SessionFileService manages the files attached to a session.
//
// Every operation keeps the session in the path, so the server validates that
// the file belongs to it. Reaching the same files through a global file filter
// would skip that check, and one supported backend does not offer the filter at
// all.
type SessionFileService struct {
	client *Client
}

// SessionFile is a file attached to a session.
type SessionFile struct {
	ID        string `json:"id"`
	Type      string `json:"type,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Filename  string `json:"filename,omitempty"`
	MimeType  string `json:"mime_type,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// SessionFileDeleted is the tombstone a delete returns.
type SessionFileDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// SessionFileListParams pages a session's files.
//
// Session files use ID cursors rather than page tokens: pass AfterID to walk
// forwards or BeforeID to walk backwards, and iteration keeps whichever
// direction it started in.
type SessionFileListParams struct {
	Limit    param.Opt[int64]
	AfterID  param.Opt[string]
	BeforeID param.Opt[string]
}

// List returns a page of the files attached to a session.
func (s SessionFileService) List(ctx context.Context, sessionID string, params SessionFileListParams, opts ...option.RequestOption) (*pagination.PageCursor[SessionFile], error) {
	opts = appendIDCursorQuery(opts, params.Limit, params.AfterID, params.BeforeID)
	return ListPage[SessionFile](ctx, s.client, "v1/sessions/"+url.PathEscape(sessionID)+"/files", opts...)
}

// Get reads a session file's metadata.
func (s SessionFileService) Get(ctx context.Context, sessionID, fileID string, opts ...option.RequestOption) (*SessionFile, error) {
	var file SessionFile
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/files/" + url.PathEscape(fileID)
	if err := s.client.GetJSON(ctx, path, &file, opts...); err != nil {
		return nil, err
	}
	return &file, nil
}

// Download streams a session file's raw content to writer.
func (s SessionFileService) Download(ctx context.Context, sessionID, fileID string, writer io.Writer, opts ...option.RequestOption) error {
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/files/" + url.PathEscape(fileID) + "/content"
	return s.client.GetStream(ctx, path, "application/octet-stream", func(body io.Reader) error {
		_, err := io.Copy(writer, body)
		return err
	}, opts...)
}

// Delete removes a session file and returns its tombstone.
func (s SessionFileService) Delete(ctx context.Context, sessionID, fileID string, opts ...option.RequestOption) (*SessionFileDeleted, error) {
	var deleted SessionFileDeleted
	path := "v1/sessions/" + url.PathEscape(sessionID) + "/files/" + url.PathEscape(fileID)
	if err := s.client.doJSON(ctx, "DELETE", path, nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}
