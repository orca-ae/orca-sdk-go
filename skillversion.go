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

// SkillVersionService manages a skill's published versions.
//
// A version is addressed by its version string, not by an opaque ID, so the
// string is a path segment and is escaped like any other.
type SkillVersionService struct {
	client *Client
}

// SkillVersion is one published version of a skill.
type SkillVersion struct {
	ID           string `json:"id"`
	Type         string `json:"type,omitzero"`
	SkillID      string `json:"skill_id,omitzero"`
	Version      string `json:"version,omitzero"`
	DisplayTitle string `json:"display_title,omitzero"`
	CreatedAt    string `json:"created_at,omitzero"`
}

// SkillVersionDeleted is the tombstone a delete returns.
type SkillVersionDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// SkillVersionNewParams publishes a new version of a skill.
type SkillVersionNewParams struct {
	Files        []SkillFile
	DisplayTitle param.Opt[string]
}

// SkillVersionListParams pages a skill's versions.
type SkillVersionListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
}

// Create publishes a new version of a skill.
func (s SkillVersionService) Create(ctx context.Context, skillID string, params SkillVersionNewParams, opts ...option.RequestOption) (*SkillVersion, error) {
	files, err := multipartSkillFiles(params.Files)
	if err != nil {
		return nil, err
	}

	body := MultipartRequest{Files: files}
	if title, ok := params.DisplayTitle.Value(); ok {
		body.Fields = map[string]string{"display_title": title}
	}

	path := "v1/skills/" + url.PathEscape(skillID) + "/versions"
	payload, err := s.client.PostMultipartWithResponse(ctx, path, body, opts...)
	if err != nil {
		return nil, err
	}
	var version SkillVersion
	if err := decodeJSON(payload, &version); err != nil {
		return nil, &apierror.DecodeError{Method: "POST", URL: path, Err: err}
	}
	return &version, nil
}

// List returns a page of a skill's versions.
func (s SkillVersionService) List(ctx context.Context, skillID string, params SkillVersionListParams, opts ...option.RequestOption) (*pagination.PageCursor[SkillVersion], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	return ListPage[SkillVersion](ctx, s.client, "v1/skills/"+url.PathEscape(skillID)+"/versions", opts...)
}

// Get reads one version of a skill.
func (s SkillVersionService) Get(ctx context.Context, skillID, version string, opts ...option.RequestOption) (*SkillVersion, error) {
	var skillVersion SkillVersion
	path := "v1/skills/" + url.PathEscape(skillID) + "/versions/" + url.PathEscape(version)
	if err := s.client.GetJSON(ctx, path, &skillVersion, opts...); err != nil {
		return nil, err
	}
	return &skillVersion, nil
}

// Download streams a version's bundle to writer as a zip archive.
//
// A published version is immutable, so the bytes are the same every time and
// safe to cache by (skill, version).
func (s SkillVersionService) Download(ctx context.Context, skillID, version string, writer io.Writer, opts ...option.RequestOption) error {
	path := "v1/skills/" + url.PathEscape(skillID) + "/versions/" + url.PathEscape(version) + "/content"
	return s.client.GetStream(ctx, path, "application/zip", func(body io.Reader) error {
		_, err := io.Copy(writer, body)
		return err
	}, opts...)
}

// Delete permanently deletes a version and returns its tombstone.
func (s SkillVersionService) Delete(ctx context.Context, skillID, version string, opts ...option.RequestOption) (*SkillVersionDeleted, error) {
	var deleted SkillVersionDeleted
	path := "v1/skills/" + url.PathEscape(skillID) + "/versions/" + url.PathEscape(version)
	if err := s.client.doJSON(ctx, "DELETE", path, nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}
