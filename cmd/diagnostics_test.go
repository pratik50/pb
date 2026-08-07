package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/parseablehq/pb/pkg/config"
)

func TestSQLMissingQueryReturnsErrorWithoutStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	query.SetOut(&stdout)
	query.SetErr(&stderr)
	t.Cleanup(func() {
		query.SetOut(nil)
		query.SetErr(nil)
	})
	interactiveFlag := query.Flags().Lookup("interactive")
	previousInteractive := interactiveFlag.Value.String()
	previousInteractiveChanged := interactiveFlag.Changed
	t.Cleanup(func() {
		_ = interactiveFlag.Value.Set(previousInteractive)
		interactiveFlag.Changed = previousInteractiveChanged
	})
	if err := query.Flags().Set("interactive", "false"); err != nil {
		t.Fatal(err)
	}

	if err := query.RunE(query, nil); err == nil || !strings.Contains(err.Error(), "SQL query is required") {
		t.Fatalf("expected missing query error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("RunE should return diagnostics, not print them directly: %q", stderr.String())
	}
}

func TestPromqlMissingExpressionReturnsErrorWithoutStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	promqlRunCmd.SetOut(&stdout)
	promqlRunCmd.SetErr(&stderr)
	t.Cleanup(func() {
		promqlRunCmd.SetOut(nil)
		promqlRunCmd.SetErr(nil)
	})
	interactiveFlag := promqlRunCmd.Flags().Lookup("interactive")
	previousInteractive := interactiveFlag.Value.String()
	previousInteractiveChanged := interactiveFlag.Changed
	t.Cleanup(func() {
		_ = interactiveFlag.Value.Set(previousInteractive)
		interactiveFlag.Changed = previousInteractiveChanged
	})
	if err := promqlRunCmd.Flags().Set("interactive", "false"); err != nil {
		t.Fatal(err)
	}

	if err := promqlRunCmd.RunE(promqlRunCmd, nil); err == nil || !strings.Contains(err.Error(), "PromQL expression is required") {
		t.Fatalf("expected missing expression error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("RunE should return diagnostics, not print them directly: %q", stderr.String())
	}
}

func TestPromqlDecodeFailureDoesNotWriteRawBodyToStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not-json")
	}))
	defer server.Close()

	originalProfile := DefaultProfile
	DefaultProfile = config.Profile{URL: server.URL, APIKey: "test-key"}
	t.Cleanup(func() { DefaultProfile = originalProfile })
	outputFlag := promqlLabelsCmd.Flags().Lookup("output")
	previousOutput := outputFlag.Value.String()
	previousOutputChanged := outputFlag.Changed
	t.Cleanup(func() {
		_ = outputFlag.Value.Set(previousOutput)
		outputFlag.Changed = previousOutputChanged
	})
	if err := promqlLabelsCmd.Flags().Set("output", "text"); err != nil {
		t.Fatal(err)
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStdout := os.Stdout
	os.Stdout = writer
	commandErr := promqlLabelsCmd.RunE(promqlLabelsCmd, nil)
	_ = writer.Close()
	os.Stdout = originalStdout
	output, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if commandErr == nil || !strings.Contains(commandErr.Error(), "failed to decode") {
		t.Fatalf("expected decode error, got %v", commandErr)
	}
	if detail := errorDetails(commandErr); detail.Code != ErrorInvalidResponse || detail.Retryable {
		t.Fatalf("unexpected decode-error classification: %+v", detail)
	}
	if len(output) != 0 {
		t.Fatalf("raw response leaked to stdout: %q", output)
	}
}
