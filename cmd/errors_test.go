package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestHTTPErrorCode(t *testing.T) {
	tests := []struct {
		status    int
		code      ErrorCode
		retryable bool
	}{
		{400, ErrorInvalidInput, false},
		{401, ErrorAuthFailed, false},
		{403, ErrorPermissionDenied, false},
		{404, ErrorNotFound, false},
		{408, ErrorTimeout, true},
		{409, ErrorConflict, false},
		{422, ErrorInvalidInput, false},
		{429, ErrorRateLimited, true},
		{500, ErrorServer, true},
	}
	for _, test := range tests {
		code, retryable := httpErrorCode(test.status)
		if code != test.code || retryable != test.retryable {
			t.Fatalf("status %d: got (%s, %t), want (%s, %t)", test.status, code, retryable, test.code, test.retryable)
		}
	}
}

func TestWriteErrorJSONRedactsHTTPResponseBody(t *testing.T) {
	err := responseStatusError("list datasets", 403, "403 Forbidden", []byte("secret backend detail"))
	var output bytes.Buffer
	if writeErr := WriteErrorJSON(&output, err); writeErr != nil {
		t.Fatal(writeErr)
	}

	var result errorEnvelope
	if jsonErr := json.Unmarshal(output.Bytes(), &result); jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if result.Error.Code != ErrorPermissionDenied || result.Error.HTTPStatus != 403 || result.Error.Retryable {
		t.Fatalf("unexpected error envelope: %+v", result.Error)
	}
	if strings.Contains(result.Error.Message, "secret backend detail") {
		t.Fatalf("structured error leaked response body: %q", result.Error.Message)
	}
	if !strings.Contains(err.Error(), "secret backend detail") {
		t.Fatalf("text-compatible error no longer contains legacy detail: %q", err)
	}
}

func TestWriteErrorJSONFallback(t *testing.T) {
	var output bytes.Buffer
	if err := WriteErrorJSON(&output, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	var result errorEnvelope
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Error.Code != ErrorCommandFailed || result.Error.Message != "boom" || result.Error.Retryable {
		t.Fatalf("unexpected fallback envelope: %+v", result.Error)
	}
}

func TestWriteErrorJSONClassifiesMissingFileAsNotFound(t *testing.T) {
	missing := &os.PathError{Op: "open", Path: "/missing/config.toml", Err: os.ErrNotExist}
	var output bytes.Buffer
	if err := WriteErrorJSON(&output, fmt.Errorf("read config: %w", missing)); err != nil {
		t.Fatal(err)
	}
	var result errorEnvelope
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Error.Code != ErrorNotFound || result.Error.Retryable {
		t.Fatalf("unexpected missing-file classification: %+v", result.Error)
	}
}

func TestWriteErrorJSONClassifiesCommandInputErrors(t *testing.T) {
	for _, message := range []string{
		`unknown command "wat" for "pb"`,
		"unknown flag: --wat",
		"flag needs an argument: --output",
		`invalid argument "wat" for "--output" flag`,
		"requires at least 1 arg(s), only received 0",
		"accepts at most 1 arg(s), received 2",
		"accepts between 1 and 2 arg(s), received 3",
		"accepts 1 arg(s), received 0",
		`unsupported output format "yaml" (expected text or json)`,
	} {
		t.Run(message, func(t *testing.T) {
			var output bytes.Buffer
			if err := WriteErrorJSON(&output, errors.New(message)); err != nil {
				t.Fatal(err)
			}
			var result errorEnvelope
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Error.Code != ErrorInvalidInput || result.Error.Retryable {
				t.Fatalf("unexpected command-input classification: %+v", result.Error)
			}
		})
	}
}

func TestRenderedErrorMarker(t *testing.T) {
	err := MarkErrorRendered(errors.New("already printed"))
	if !ErrorWasRendered(err) {
		t.Fatal("expected error to be marked as rendered")
	}
	if err.Error() != "already printed" {
		t.Fatalf("unexpected error text: %q", err)
	}
}
