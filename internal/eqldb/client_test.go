package eqldb

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestDeviceConnectionAndTokenPolling(t *testing.T) {
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/device/connect/":
			writeJSON(t, response, map[string]any{
				"device_code":               "device-code",
				"user_code":                 "ABCD-EFGH",
				"verification_uri":          serverURL(request) + "/connect/",
				"verification_uri_complete": serverURL(request) + "/connect/?code=ABCD-EFGH",
				"expires_in":                600,
				"interval":                  1,
			})
		case "/api/v1/device/token/":
			if polls.Add(1) == 1 {
				response.WriteHeader(http.StatusBadRequest)
				writeJSON(t, response, map[string]any{
					"error":             "authorization_pending",
					"error_description": "waiting",
				})
				return
			}
			writeJSON(t, response, map[string]any{
				"access_token":  "private-token",
				"token_type":    "Bearer",
				"scope":         "inventory:upload",
				"connection_id": "connection-id",
			})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTP: server.Client()}
	authorization, err := client.StartConnection(context.Background(), "test device")
	if err != nil {
		t.Fatal(err)
	}
	authorization.Interval = time.Millisecond
	token, err := client.WaitForToken(context.Background(), authorization)
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "private-token" || token.ConnectionID != "connection-id" || polls.Load() != 2 {
		t.Fatalf("unexpected token: %#v after %d polls", token, polls.Load())
	}
}

func TestDefaultClientUsesLocalDevelopmentServer(t *testing.T) {
	client := NewClient()
	if client.BaseURL != "https://eqldb.org" {
		t.Fatalf("unexpected production base URL: %q", client.BaseURL)
	}
}

func TestUploadInventory(t *testing.T) {
	var received bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/inventory/upload/" {
			http.NotFound(response, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer private-token" {
			t.Fatalf("unexpected authorization header")
		}
		if err := request.ParseMultipartForm(1024 * 1024); err != nil {
			t.Fatal(err)
		}
		file, header, err := request.FormFile("inventory_file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if header.Filename != "Wyrmberg_rivervale-Inventory.txt" || string(data) != "inventory data" {
			t.Fatalf("unexpected file %q: %q", header.Filename, data)
		}
		if got := request.MultipartForm.Value["classes[]"]; len(got) != 3 || got[0] != "PAL" || got[2] != "MNK" {
			t.Fatalf("unexpected classes: %v", got)
		}
		if request.FormValue("race") != "Ancient Wolf" || request.FormValue("level") != "50" {
			t.Fatalf("unexpected metadata: %#v", request.MultipartForm.Value)
		}
		received = true
		writeJSON(t, response, map[string]any{
			"status":      "completed",
			"character":   "Wyrmberg",
			"server":      "rivervale",
			"profile_url": "/profiles/account/rivervale/wyrmberg/loadout/",
		})
	}))
	defer server.Close()

	inventoryPath := filepath.Join(t.TempDir(), "Wyrmberg_rivervale-Inventory.txt")
	if err := os.WriteFile(inventoryPath, []byte("inventory data"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &Client{BaseURL: server.URL, HTTP: server.Client()}
	result, err := client.UploadInventory(context.Background(), "private-token", inventoryPath, UploadMetadata{
		Level:   50,
		Classes: []string{"PAL", "DRU", "MNK"},
		Race:    "Ancient Wolf",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !received || result.Status != "completed" || result.Character != "Wyrmberg" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestWaitForTokenStopsOnDenial(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusBadRequest)
		writeJSON(t, response, map[string]any{
			"error":             "access_denied",
			"error_description": "The connection was denied.",
		})
	}))
	defer server.Close()
	client := &Client{BaseURL: server.URL, HTTP: server.Client()}
	_, err := client.WaitForToken(context.Background(), DeviceAuthorization{
		DeviceCode: "code",
		ExpiresAt:  time.Now().Add(time.Minute),
		Interval:   time.Millisecond,
	})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "access_denied" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeJSON(t *testing.T, response http.ResponseWriter, value any) {
	t.Helper()
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host
}
