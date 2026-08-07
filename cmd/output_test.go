package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: outputText},
		{input: "text", want: outputText},
		{input: " JSON ", want: outputJSON},
	}
	for _, test := range tests {
		got, err := validateOutputFormat(test.input)
		if err != nil {
			t.Fatalf("validateOutputFormat(%q): %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf("validateOutputFormat(%q)=%q want=%q", test.input, got, test.want)
		}
	}
	if _, err := validateOutputFormat("yaml"); err == nil {
		t.Fatal("expected unsupported output format error")
	}
}

func TestWriteRawJSONProducesOneValidJSONDocument(t *testing.T) {
	var output bytes.Buffer
	if err := writeRawJSON(&output, []byte(`{"message":"ok","count":2}`)); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output %q: %v", output.String(), err)
	}
	if strings.Count(output.String(), "\n") < 2 {
		t.Fatalf("expected formatted JSON, got %q", output.String())
	}
}

func TestWriteRawJSONRejectsMalformedOrMultipleDocuments(t *testing.T) {
	for _, body := range []string{`not-json`, `{} {}`} {
		var output bytes.Buffer
		if err := writeRawJSON(&output, []byte(body)); err == nil {
			t.Fatalf("expected error for %q", body)
		}
		if output.Len() != 0 {
			t.Fatalf("unexpected partial output for %q: %q", body, output.String())
		}
	}
}

func TestNonNilSliceEncodesEmptyArray(t *testing.T) {
	var values []string
	var output bytes.Buffer
	if err := writeJSON(&output, nonNilSlice(values)); err != nil {
		t.Fatal(err)
	}
	if output.String() != "[]\n" {
		t.Fatalf("nil slice encoded as %q, want empty JSON array", output.String())
	}
}
