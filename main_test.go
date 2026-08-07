package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	pb "github.com/parseablehq/pb/cmd"
	"github.com/spf13/cobra"
)

func TestRenderExecutionErrorJSON(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	command.Flags().StringP("output", "o", "text", "output format")
	if err := command.Flags().Set("output", "json"); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.SetErr(&stderr)

	if err := renderExecutionError(command, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &result); err != nil {
		t.Fatalf("stderr is not JSON: %v\n%s", err, stderr.String())
	}
	if result.Error.Code != "COMMAND_FAILED" || result.Error.Message != "boom" {
		t.Fatalf("unexpected error JSON: %+v", result.Error)
	}
}

func TestRenderExecutionErrorText(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	command.Flags().StringP("output", "o", "text", "output format")
	var stderr bytes.Buffer
	command.SetErr(&stderr)

	if err := renderExecutionError(command, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	if stderr.String() != "Error: boom\n" {
		t.Fatalf("unexpected text error: %q", stderr.String())
	}
}

func TestRenderExecutionErrorSkipsAlreadyRendered(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	command.Flags().StringP("output", "o", "json", "output format")
	var stderr bytes.Buffer
	command.SetErr(&stderr)

	if err := renderExecutionError(command, pb.MarkErrorRendered(errors.New("boom"))); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("already-rendered error was duplicated: %q", stderr.String())
	}
}

func TestRequestsJSONOutput(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"dataset", "list", "-o", "json"}, true},
		{[]string{"dataset", "list", "--output=json"}, true},
		{[]string{"dataset", "list", "-ojson"}, true},
		{[]string{"dataset", "list", "-o", "text"}, false},
		{[]string{"sql", "run", "--", "-ojson"}, false},
		{[]string{"dataset", "list"}, false},
	}
	for _, test := range tests {
		if got := requestsJSONOutput(test.args); got != test.want {
			t.Fatalf("requestsJSONOutput(%q) = %t, want %t", test.args, got, test.want)
		}
	}
}
