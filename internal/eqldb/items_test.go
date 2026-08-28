package eqldb

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetItemMetadata(t *testing.T) {
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/api/v1/items/metadata/" {
			t.Fatalf("path = %q, want item metadata endpoint", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer private-token" {
			t.Fatalf("unexpected authorization header %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content type %q", request.Header.Get("Content-Type"))
		}

		var body struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.IDs) != 2 || body.IDs[0] != "10150" || body.IDs[1] != "10151" {
			t.Fatalf("unexpected item IDs: %#v", body.IDs)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`[
				{"id":"10150","data":{"itemname":"Example Item"}},
				{"id":"10151","data":null}
			]`)),
			Request: request,
		}, nil
	})}

	result, err := GetItemMetadata("private-token", "10150", "10151")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[0].ID != "10150" || result[0].Data["itemname"] != "Example Item" {
		t.Fatalf("unexpected metadata result: %#v", result)
	}
	if result[1].ID != "10151" || result[1].Data != nil {
		t.Fatalf("unexpected missing metadata result: %#v", result[1])
	}
}

func TestGetItemMetadataValidatesInput(t *testing.T) {
	invalidRequests := []struct {
		name  string
		token string
		ids   []string
	}{
		{name: "empty token", ids: []string{"1"}},
		{name: "empty IDs", token: "token"},
		{name: "too many IDs", token: "token", ids: make([]string, 501)},
		{name: "zero", token: "token", ids: []string{"0"}},
		{name: "negative", token: "token", ids: []string{"-1"}},
		{name: "non-decimal", token: "token", ids: []string{"1a"}},
		{name: "too long", token: "token", ids: []string{"123456789012345678901"}},
	}
	for _, test := range invalidRequests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := GetItemMetadata(test.token, test.ids...); err == nil {
				t.Fatal("invalid request was accepted")
			}
		})
	}
}

func TestGetItemMetadataRejectsWrongResponseOrder(t *testing.T) {
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`[{"id":"2","data":null},{"id":"1","data":null}]`)),
			Request:    request,
		}, nil
	})}

	if _, err := GetItemMetadata("token", "1", "2"); err == nil {
		t.Fatal("out-of-order response was accepted")
	}
}
