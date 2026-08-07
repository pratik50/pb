package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/parseablehq/pb/pkg/config"
	"github.com/spf13/cobra"
)

func TestPrintVersionUsesCallingCommandOutputFormat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.WriteConfigToFile(&config.Config{
		Profiles: map[string]config.Profile{
			"test": {URL: "https://example.com", APIKey: "test-key"},
		},
		DefaultProfile: "test",
	}); err != nil {
		t.Fatal(err)
	}
	useCommandTransport(t, func(*http.Request) (*http.Response, error) {
		return commandHTTPResponse(http.StatusOK, "200 OK", `{"version":"v2","commit":"server-commit"}`), nil
	})

	command := &cobra.Command{Use: "pb"}
	command.Flags().StringP("output", "o", "text", "output format")
	if err := command.Flags().Set("output", "json"); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	command.SetOut(&output)

	if err := PrintVersion(command, "v1", "client-commit"); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Client map[string]string `json:"client"`
		Server map[string]string `json:"server"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatalf("invalid version JSON %q: %v", output.String(), err)
	}
	if result.Client["version"] != "v1" || result.Server["version"] != "v2" {
		t.Fatalf("unexpected version output: %+v", result)
	}
}
