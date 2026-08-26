// Copyright (c) 2026 StreamNative, Inc. All Rights Reserved.

package orca

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"reflect"
	"strings"

	"github.com/orca-ae/orca-sdk-go/internal/apierror"
)

const (
	defaultMultipartFieldName = "data"
	defaultMultipartFileName  = "upload.bin"
)

// MultipartFile represents one file part in a multipart registry request.
type MultipartFile struct {
	FieldName   string
	FileName    string
	ContentType string
	Content     []byte
}

// MultipartRequest represents a registry multipart/form-data request.
type MultipartRequest struct {
	Accept        string
	File          *MultipartFile
	Files         []*MultipartFile
	URL           string
	Fields        map[string]string
	JSONFields    map[string]interface{}
	ConfigField   string
	Config        interface{}
	UpdateOptions interface{}
}

// buildMultipartBody encodes body into a multipart/form-data payload and
// returns it along with the Content-Type carrying its boundary.
//
// The whole body is buffered rather than streamed because a retry has to send
// it again, and a streaming reader is consumed by the first attempt.
func buildMultipartBody(body MultipartRequest, allowEmptyConfig bool) (contentType string, payload []byte, err error) {
	hasConfig := strings.TrimSpace(body.ConfigField) != "" || body.Config != nil
	if !allowEmptyConfig && !hasConfig {
		return "", nil, apierror.Validationf("multipart config field is required")
	}
	if hasConfig {
		if strings.TrimSpace(body.ConfigField) == "" {
			return "", nil, apierror.Validationf("multipart config field is required")
		}
		if body.Config == nil {
			return "", nil, apierror.Validationf("multipart config payload is required")
		}
	}

	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)

	files := make([]*MultipartFile, 0, 1+len(body.Files))
	if body.File != nil {
		files = append(files, body.File)
	}
	files = append(files, body.Files...)
	for _, file := range files {
		if file == nil {
			continue
		}
		if err := writeMultipartFileField(writer, file); err != nil {
			return "", nil, err
		}
	}

	if strings.TrimSpace(body.URL) != "" {
		if err := writer.WriteField("url", body.URL); err != nil {
			return "", nil, apierror.Errorf("failed to write multipart url field: %w", err)
		}
	}

	if hasConfig {
		if err := writeMultipartJSONField(writer, body.ConfigField, body.Config); err != nil {
			return "", nil, err
		}
	}

	for fieldName, fieldValue := range body.Fields {
		if strings.TrimSpace(fieldValue) == "" {
			continue
		}
		if err := writer.WriteField(fieldName, fieldValue); err != nil {
			return "", nil, apierror.Errorf("failed to write multipart %s field: %w", fieldName, err)
		}
	}

	for fieldName, fieldValue := range body.JSONFields {
		if fieldValue == nil {
			continue
		}
		if err := writeMultipartJSONField(writer, fieldName, fieldValue); err != nil {
			return "", nil, err
		}
	}

	if !isNilMultipartValue(body.UpdateOptions) {
		if err := writeMultipartJSONField(writer, "updateOptions", body.UpdateOptions); err != nil {
			return "", nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return "", nil, apierror.Errorf("failed to finalize multipart request: %w", err)
	}

	return writer.FormDataContentType(), buf.Bytes(), nil
}

func isNilMultipartValue(value interface{}) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

func writeMultipartFileField(writer *multipart.Writer, file *MultipartFile) error {
	fieldName := file.FieldName
	if strings.TrimSpace(fieldName) == "" {
		fieldName = defaultMultipartFieldName
	}
	fileName := file.FileName
	if strings.TrimSpace(fileName) == "" {
		fileName = defaultMultipartFileName
	}

	if strings.TrimSpace(file.ContentType) == "" {
		part, err := writer.CreateFormFile(fieldName, fileName)
		if err != nil {
			return apierror.Errorf("failed to create multipart file part: %w", err)
		}
		if _, err := part.Write(file.Content); err != nil {
			return apierror.Errorf("failed to write multipart file part: %w", err)
		}
		return nil
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			sanitizeHeaderValue(fieldName), sanitizeHeaderValue(fileName)))
	header.Set("Content-Type", file.ContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return apierror.Errorf("failed to create multipart file part: %w", err)
	}
	if _, err := part.Write(file.Content); err != nil {
		return apierror.Errorf("failed to write multipart file part: %w", err)
	}
	return nil
}

// sanitizeHeaderValue removes characters that would let a resource name break
// out of the Content-Disposition header it is interpolated into: double quotes,
// backslashes, carriage returns, and newlines. Names reach here from user input,
// so this is header injection, not hypothetical.
func sanitizeHeaderValue(s string) string {
	r := strings.NewReplacer(`"`, "", `\`, "", "\r", "", "\n", "")
	return r.Replace(s)
}

func writeMultipartJSONField(writer *multipart.Writer, fieldName string, value interface{}) error {
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, fieldName))
	header.Set("Content-Type", "application/json")

	part, err := writer.CreatePart(header)
	if err != nil {
		return apierror.Errorf("failed to create multipart %s field: %w", fieldName, err)
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return apierror.Errorf("failed to marshal multipart %s payload: %w", fieldName, err)
	}

	if _, err := part.Write(payload); err != nil {
		return apierror.Errorf("failed to write multipart %s payload: %w", fieldName, err)
	}

	return nil
}
