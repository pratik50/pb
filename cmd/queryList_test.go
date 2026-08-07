package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/parseablehq/pb/pkg/config"
	internalHTTP "github.com/parseablehq/pb/pkg/http"
)

type savedQueryRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn savedQueryRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestSavedQueryToPbQueryPrintsEmptyQueryMessage(t *testing.T) {
	var output bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = w.Close()
		_ = r.Close()
	})
	os.Stdout = w

	err = savedQueryToPbQuery("   ", "", "")

	_ = w.Close()
	_, _ = io.Copy(&output, r)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got, want := output.String(), "Empty query selected.\n"; got != want {
		t.Fatalf("unexpected output\nwant: %q\n got: %q", want, got)
	}
}

func TestFetchFiltersReturnsEmptyArrayForNoSavedQueries(t *testing.T) {
	profile := config.Profile{URL: "https://example.com", APIKey: "test-key"}
	client := internalHTTP.DefaultClientWithTransport(&profile, savedQueryRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`[]`)),
		}, nil
	}))

	items, err := fetchFilters(&client)
	if err != nil {
		t.Fatal(err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("expected non-nil empty result, got %#v", items)
	}
}

func TestFetchFiltersReturnsServerErrors(t *testing.T) {
	profile := config.Profile{URL: "https://example.com", APIKey: "test-key"}
	client := internalHTTP.DefaultClientWithTransport(&profile, savedQueryRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
			Body:       io.NopCloser(strings.NewReader(`denied`)),
		}, nil
	}))

	if _, err := fetchFilters(&client); err == nil || !strings.Contains(err.Error(), "403 Forbidden") {
		t.Fatalf("expected server error, got %v", err)
	}
}

func TestSavedQueryListJSONBypassesInteractiveMenu(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/filters" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	previousProfile := DefaultProfile
	previousOutput := outputFlag
	DefaultProfile = config.Profile{URL: server.URL, APIKey: "test-key"}
	outputFlag = outputJSON
	t.Cleanup(func() {
		DefaultProfile = previousProfile
		outputFlag = previousOutput
		SavedQueryList.SetOut(nil)
	})

	var stdout bytes.Buffer
	SavedQueryList.SetOut(&stdout)
	if err := SavedQueryList.RunE(SavedQueryList, nil); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "[]\n" {
		t.Fatalf("unexpected JSON output: %q", stdout.String())
	}
}
