package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Fatalf("Accept header = %q", got)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"tag_name":"v0.3.0","name":"eqdps 0.3.0","html_url":"https://github.com/uija/eqdps/releases/tag/v0.3.0","body":"Release notes"}`))
	}))
	defer server.Close()

	release, err := Latest(context.Background(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v0.3.0" || release.Name != "eqdps 0.3.0" {
		t.Fatalf("unexpected release: %#v", release)
	}
	if release.Body != "Release notes" {
		t.Fatalf("release body = %q", release.Body)
	}
}

func TestLatestRejectsErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Error(response, "rate limited", http.StatusForbidden)
	}))
	defer server.Close()

	if _, err := Latest(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("expected error response to fail")
	}
}
