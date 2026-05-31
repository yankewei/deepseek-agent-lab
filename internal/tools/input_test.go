package tools

import "testing"

func TestDecodeInputRejectsUnknownFields(t *testing.T) {
	var args struct {
		Path string `json:"path"`
	}
	if err := decodeInput([]byte(`{"path":"a.txt","extra":true}`), &args); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestObjectSchemaRejectsAdditionalProperties(t *testing.T) {
	schema := objectSchema(map[string]any{"path": map[string]any{"type": "string"}}, "path")
	if schema["additionalProperties"] != false {
		t.Fatalf("additionalProperties = %v, want false", schema["additionalProperties"])
	}
}
