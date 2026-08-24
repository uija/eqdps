package github

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckNewVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"tag_name":"v1.2.0","name":"eqdps v1.2.0","html_url":"https://example.com/v1.2.0","body":"Release notes"}`))
	}))
	defer server.Close()

	release, differs, err := checkNewVersion(server.Client(), server.URL, "v1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !differs || release.TagName != "v1.2.0" || release.HTMLURL == "" || release.Body != "Release notes" {
		t.Fatalf("unexpected result: release=%#v differs=%v", release, differs)
	}
}

func TestCheckNewVersionIsCurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write([]byte(`{"tag_name":"v1.2.0"}`))
	}))
	defer server.Close()

	release, differs, err := checkNewVersion(server.Client(), server.URL, "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if differs || release != (Release{}) {
		t.Fatalf("unexpected result: release=%#v differs=%v", release, differs)
	}
}
