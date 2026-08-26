package orca

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"testing"
)

func decodeMultipartRequest(t *testing.T, r *http.Request) (map[string]string, map[string][]byte) {
	t.Helper()

	contentType := r.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType() error = %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("mediaType = %q, want multipart/form-data", mediaType)
	}

	reader := multipart.NewReader(r.Body, params["boundary"])
	fields := map[string]string{}
	files := map[string][]byte{}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart() error = %v", err)
		}
		data, err := io.ReadAll(part)
		if closeErr := part.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if part.FileName() != "" {
			files[part.FormName()] = data
			continue
		}
		fields[part.FormName()] = string(data)
	}

	return fields, files
}
