// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
	"github.com/orca-ae/orca-sdk-go/option"
	"github.com/orca-ae/orca-sdk-go/packages/pagination"
	"github.com/orca-ae/orca-sdk-go/packages/param"
)

// SkillService manages skills.
type SkillService struct {
	client *Client

	// Versions manages a skill's published versions.
	Versions SkillVersionService
}

func newSkillService(client *Client) SkillService {
	return SkillService{client: client, Versions: SkillVersionService{client: client}}
}

// skillFilesField is the multipart field every skill file is uploaded under.
// The brackets are literal: the server reads a repeated "files[]" field, not
// "files".
const skillFilesField = "files[]"

// Skill is a bundle of files an agent can use.
type Skill struct {
	ID           string `json:"id"`
	Type         string `json:"type,omitzero"`
	DisplayTitle string `json:"display_title,omitzero"`
	Source       string `json:"source,omitzero"`
	Version      string `json:"version,omitzero"`
	CreatedAt    string `json:"created_at,omitzero"`
	UpdatedAt    string `json:"updated_at,omitzero"`
}

// SkillDeleted is the tombstone a delete returns.
type SkillDeleted struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// SkillFile is one file in a skill bundle.
type SkillFile struct {
	// Filename is the path the file takes inside the bundle.
	Filename string

	// ContentType is the file's MIME type. When empty the server infers one.
	ContentType string

	// Content is the file's bytes.
	Content []byte
}

// SkillNewParams creates a skill from a bundle of files.
type SkillNewParams struct {
	Files []SkillFile

	// DisplayTitle is an optional human-readable name, sent as a plain form
	// field.
	DisplayTitle param.Opt[string]
}

// SkillListParams pages skills.
//
// Limit is the only portable filter: the overlay removes source,
// include_archived and provider.
type SkillListParams struct {
	Limit param.Opt[int64]
	Page  param.Opt[string]
}

// multipartSkillFiles turns a bundle into repeated "files[]" parts.
func multipartSkillFiles(files []SkillFile) ([]*MultipartFile, error) {
	if len(files) == 0 {
		return nil, apierror.Validationf("at least one skill file is required")
	}
	parts := make([]*MultipartFile, 0, len(files))
	for _, file := range files {
		if file.Filename == "" {
			return nil, apierror.Validationf("every skill file needs a name")
		}
		parts = append(parts, &MultipartFile{
			FieldName:   skillFilesField,
			FileName:    file.Filename,
			ContentType: file.ContentType,
			Content:     file.Content,
		})
	}
	return parts, nil
}

// decodeJSON decodes a raw response payload.
func decodeJSON(payload []byte, out any) error {
	return json.Unmarshal(payload, out)
}

// Create uploads a skill bundle.
func (s SkillService) Create(ctx context.Context, params SkillNewParams, opts ...option.RequestOption) (*Skill, error) {
	files, err := multipartSkillFiles(params.Files)
	if err != nil {
		return nil, err
	}

	body := MultipartRequest{Files: files}
	if title, ok := params.DisplayTitle.Value(); ok {
		body.Fields = map[string]string{"display_title": title}
	}

	payload, err := s.client.PostMultipartWithResponse(ctx, "v1/skills", body, opts...)
	if err != nil {
		return nil, err
	}
	var skill Skill
	if err := decodeJSON(payload, &skill); err != nil {
		return nil, &apierror.DecodeError{Method: "POST", URL: "v1/skills", Err: err}
	}
	return &skill, nil
}

// Get reads a skill.
func (s SkillService) Get(ctx context.Context, skillID string, opts ...option.RequestOption) (*Skill, error) {
	var skill Skill
	if err := s.client.GetJSON(ctx, "v1/skills/"+url.PathEscape(skillID), &skill, opts...); err != nil {
		return nil, err
	}
	return &skill, nil
}

// List returns a page of skills.
func (s SkillService) List(ctx context.Context, params SkillListParams, opts ...option.RequestOption) (*pagination.PageCursor[Skill], error) {
	opts = appendListQuery(opts, params.Limit, params.Page)
	return ListPage[Skill](ctx, s.client, "v1/skills", opts...)
}

// Delete permanently deletes a skill and returns its tombstone.
func (s SkillService) Delete(ctx context.Context, skillID string, opts ...option.RequestOption) (*SkillDeleted, error) {
	var deleted SkillDeleted
	if err := s.client.doJSON(ctx, "DELETE", "v1/skills/"+url.PathEscape(skillID), nil, &deleted, opts...); err != nil {
		return nil, err
	}
	return &deleted, nil
}
