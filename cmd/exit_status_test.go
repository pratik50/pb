package cmd

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/parseablehq/pb/pkg/config"
)

type commandRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn commandRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func useCommandTransport(t *testing.T, fn commandRoundTripFunc) {
	t.Helper()
	originalTransport := http.DefaultTransport
	http.DefaultTransport = fn
	originalProfile := DefaultProfile
	DefaultProfile = config.Profile{URL: "https://example.com", APIKey: "test-key"}
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
		DefaultProfile = originalProfile
	})
}

func commandHTTPResponse(statusCode int, status, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestAddDatasetReturnsErrorForServerFailure(t *testing.T) {
	useCommandTransport(t, func(*http.Request) (*http.Response, error) {
		return commandHTTPResponse(http.StatusInternalServerError, "500 Internal Server Error", "storage unavailable"), nil
	})
	if err := AddDatasetCmd.Flags().Set(datasetTypeFlag, "logs"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = AddDatasetCmd.Flags().Set(datasetTypeFlag, "") })

	err := AddDatasetCmd.RunE(AddDatasetCmd, []string{"backend"})
	if err == nil || !strings.Contains(err.Error(), "create dataset failed: 500 Internal Server Error") {
		t.Fatalf("expected server failure, got %v", err)
	}
}

func TestRemoveUserReturnsErrorForServerFailure(t *testing.T) {
	useCommandTransport(t, func(*http.Request) (*http.Response, error) {
		return commandHTTPResponse(http.StatusForbidden, "403 Forbidden", "denied"), nil
	})

	err := RemoveUserCmd.RunE(RemoveUserCmd, []string{"alice"})
	if err == nil || !strings.Contains(err.Error(), "delete user failed: 403 Forbidden") {
		t.Fatalf("expected server failure, got %v", err)
	}
}

func TestRemoveRoleReturnsErrorForServerFailure(t *testing.T) {
	useCommandTransport(t, func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			return commandHTTPResponse(http.StatusOK, "200 OK", `{"reader":[]}`), nil
		}
		return commandHTTPResponse(http.StatusForbidden, "403 Forbidden", "denied"), nil
	})

	err := RemoveRoleCmd.RunE(RemoveRoleCmd, []string{"reader"})
	if err == nil || !strings.Contains(err.Error(), "delete role failed: 403 Forbidden") {
		t.Fatalf("expected server failure, got %v", err)
	}
}

func TestRemoveProfileReturnsErrorWhenProfileDoesNotExist(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.WriteConfigToFile(&config.Config{
		Profiles: map[string]config.Profile{
			"local": {URL: "https://example.com", APIKey: "test-key"},
		},
		DefaultProfile: "local",
	}); err != nil {
		t.Fatal(err)
	}

	err := RemoveProfileCmd.RunE(RemoveProfileCmd, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "no profile found") {
		t.Fatalf("expected missing profile error, got %v", err)
	}
}
