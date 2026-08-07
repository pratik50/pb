package cmd

import (
	"testing"

	"github.com/parseablehq/pb/pkg/config"
)

func TestPreRunRejectsMissingDefaultProfileEntry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.WriteConfigToFile(&config.Config{
		Profiles: map[string]config.Profile{
			"available": {URL: "https://example.com"},
		},
		DefaultProfile: "missing",
	}); err != nil {
		t.Fatal(err)
	}

	err := PreRun()
	if err == nil {
		t.Fatal("expected missing default profile error")
	}
	if detail := errorDetails(err); detail.Code != ErrorNotFound || detail.Retryable {
		t.Fatalf("unexpected missing-profile classification: %+v", detail)
	}
}
