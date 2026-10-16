package handler

import (
	"bytes"
	"testing"
)

// Routing extraction must not require multipart or rewrite native fields.
// This is mechanics-only: it cannot establish file ownership or server support.
func TestImageEditNativeJSONRouting(t *testing.T) {
	body := []byte(`{ "model":"gpt-image-2", "images":[{"file_id":"owned-fixture"}], "native_future":{"keep":true} }`)
	original := bytes.Clone(body)
	got, err := imageEditRequestModel(body, "application/json; charset=utf-8")
	if err != nil {
		t.Fatalf("native JSON routing rejected: %v", err)
	}
	if got != "gpt-image-2" {
		t.Fatalf("model = %q", got)
	}
	if !bytes.Equal(body, original) {
		t.Fatal("routing extraction mutated native body")
	}
}
