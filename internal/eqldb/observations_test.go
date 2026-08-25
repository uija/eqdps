package eqldb

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestUploadObservations(t *testing.T) {
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	received := make(map[string]map[string]any)
	httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", request.Method)
		}
		if request.Header.Get("Authorization") != "Bearer private-token" {
			t.Fatalf("unexpected authorization header %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected content type %q", request.Header.Get("Content-Type"))
		}

		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		received[request.URL.Path] = body
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"success":true}`)),
			Request:    request,
		}, nil
	})}

	if err := UploadKillObservations("private-token", KillObservation{
		Timestamp: "Thu Jul 09 09:12:53 2026",
		Zone:      "The Plane of Sky",
		Mob:       "a thunder spirit",
	}); err != nil {
		t.Fatal(err)
	}
	if err := UploadDropObservations("private-token", DropObservation{
		Timestamp: "Thu Jul 09 09:12:56 2026",
		Zone:      "The Plane of Sky",
		Mob:       "a thunder spirit",
		Item:      "Wind Rune Caza",
		Amount:    1,
	}); err != nil {
		t.Fatal(err)
	}

	if events, ok := received["/api/v1/observations/kills/"]["events"].([]any); !ok || len(events) != 1 {
		t.Fatalf("unexpected kill request: %#v", received["/api/v1/observations/kills/"])
	}
	if events, ok := received["/api/v1/observations/drops/"]["events"].([]any); !ok || len(events) != 1 {
		t.Fatalf("unexpected drop request: %#v", received["/api/v1/observations/drops/"])
	}
}

func TestUploadObservationsValidatesInput(t *testing.T) {
	if err := UploadKillObservations("", KillObservation{}); err == nil {
		t.Fatal("empty access token was accepted")
	}
	if err := UploadKillObservations("token"); err == nil {
		t.Fatal("empty kill batch was accepted")
	}
	if err := UploadDropObservations("token", make([]DropObservation, 2001)...); err == nil {
		t.Fatal("oversized drop batch was accepted")
	}
}

func TestUploadObservationsReturnsAPIError(t *testing.T) {
	originalClient := httpClient
	defer func() { httpClient = originalClient }()

	httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"error":"invalid_token","error_description":"The token was revoked."}`,
			)),
			Request: request,
		}, nil
	})}

	err := UploadKillObservations("revoked-token", KillObservation{})
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Status != http.StatusUnauthorized || !IsUnauthorized(err) {
		t.Fatalf("error = %#v, want unauthorized APIError", err)
	}
}
