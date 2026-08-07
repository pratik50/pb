package cmd

import (
	"bytes"
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

func TestRemoveRoleMissingReturnsNotFound(t *testing.T) {
	useCommandTransport(t, func(*http.Request) (*http.Response, error) {
		return commandHTTPResponse(http.StatusOK, "200 OK", `{"reader":[]}`), nil
	})

	err := RemoveRoleCmd.RunE(RemoveRoleCmd, []string{"missing"})
	if err == nil {
		t.Fatal("expected missing role error")
	}
	if detail := errorDetails(err); detail.Code != ErrorNotFound {
		t.Fatalf("unexpected missing-role classification: %+v", detail)
	}
}

func TestAddUserMissingRoleReturnsNotFound(t *testing.T) {
	useCommandTransport(t, func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/user") {
			return commandHTTPResponse(http.StatusOK, "200 OK", `[]`), nil
		}
		return commandHTTPResponse(http.StatusOK, "200 OK", `{"reader":[]}`), nil
	})
	role := AddUserCmd.Flags().Lookup(roleFlag)
	previousRole := role.Value.String()
	previousRoleChanged := role.Changed
	t.Cleanup(func() {
		_ = role.Value.Set(previousRole)
		role.Changed = previousRoleChanged
	})
	if err := AddUserCmd.Flags().Set(roleFlag, "missing"); err != nil {
		t.Fatal(err)
	}

	err := AddUserCmd.RunE(AddUserCmd, []string{"alice"})
	if err == nil {
		t.Fatal("expected missing role error")
	}
	if detail := errorDetails(err); detail.Code != ErrorNotFound {
		t.Fatalf("unexpected missing-role classification: %+v", detail)
	}
}

func TestSetUserRoleMissingUserReturnsNotFound(t *testing.T) {
	useCommandTransport(t, func(*http.Request) (*http.Response, error) {
		return commandHTTPResponse(http.StatusOK, "200 OK", `[{"id":"bob","method":"native"}]`), nil
	})

	err := SetUserRoleCmd.RunE(SetUserRoleCmd, []string{"alice", "reader"})
	if err == nil {
		t.Fatal("expected missing user error")
	}
	if detail := errorDetails(err); detail.Code != ErrorNotFound {
		t.Fatalf("unexpected missing-user classification: %+v", detail)
	}
}

func TestSetUserRoleMissingRoleReturnsNotFound(t *testing.T) {
	useCommandTransport(t, func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/user") {
			return commandHTTPResponse(http.StatusOK, "200 OK", `[{"id":"alice","method":"native"}]`), nil
		}
		return commandHTTPResponse(http.StatusOK, "200 OK", `{"reader":[]}`), nil
	})

	err := SetUserRoleCmd.RunE(SetUserRoleCmd, []string{"alice", "missing"})
	if err == nil {
		t.Fatal("expected missing role error")
	}
	if detail := errorDetails(err); detail.Code != ErrorNotFound {
		t.Fatalf("unexpected missing-role classification: %+v", detail)
	}
}

func TestListUserJSONDoesNotWritePartialResult(t *testing.T) {
	useCommandTransport(t, func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/user/alice/role") {
			return commandHTTPResponse(http.StatusInternalServerError, "500 Internal Server Error", "unavailable"), nil
		}
		return commandHTTPResponse(http.StatusOK, "200 OK", `[{"id":"alice","method":"native"}]`), nil
	})
	output := ListUserCmd.Flags().Lookup("output")
	previousOutput := output.Value.String()
	previousOutputChanged := output.Changed
	var stdout bytes.Buffer
	ListUserCmd.SetOut(&stdout)
	t.Cleanup(func() {
		_ = output.Value.Set(previousOutput)
		output.Changed = previousOutputChanged
		ListUserCmd.SetOut(nil)
	})
	if err := ListUserCmd.Flags().Set("output", "json"); err != nil {
		t.Fatal(err)
	}

	err := ListUserCmd.RunE(ListUserCmd, nil)
	if err == nil {
		t.Fatal("expected role enrichment error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("partial JSON written before error: %q", stdout.String())
	}
}

func TestListRoleJSONDoesNotWritePartialResult(t *testing.T) {
	useCommandTransport(t, func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/role/reader") {
			return commandHTTPResponse(http.StatusInternalServerError, "500 Internal Server Error", "unavailable"), nil
		}
		return commandHTTPResponse(http.StatusOK, "200 OK", `{"reader":[]}`), nil
	})
	output := ListRoleCmd.Flags().Lookup("output")
	previousOutput := output.Value.String()
	previousOutputChanged := output.Changed
	var stdout bytes.Buffer
	ListRoleCmd.SetOut(&stdout)
	t.Cleanup(func() {
		_ = output.Value.Set(previousOutput)
		output.Changed = previousOutputChanged
		ListRoleCmd.SetOut(nil)
	})
	if err := ListRoleCmd.Flags().Set("output", "json"); err != nil {
		t.Fatal(err)
	}

	err := ListRoleCmd.RunE(ListRoleCmd, nil)
	if err == nil {
		t.Fatal("expected role detail error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("partial JSON written before error: %q", stdout.String())
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
