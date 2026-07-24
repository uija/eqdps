package eqldb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	ClientID       = "eql-log-parser"
	DefaultBaseURL = "https://eqldb.org"
	maxResponse    = 1024 * 1024
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

type DeviceAuthorization struct {
	DeviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresAt               time.Time
	Interval                time.Duration
}

type Token struct {
	AccessToken  string
	TokenType    string
	Scope        string
	ConnectionID string
}

type UploadMetadata struct {
	Level   int
	Classes []string
	Race    string
}

type UploadResult struct {
	Status     string `json:"status"`
	Character  string `json:"character"`
	Server     string `json:"server"`
	ProfileURL string `json:"profile_url"`
	Message    string `json:"message"`
}

type APIError struct {
	Status      int
	Code        string
	Description string
	RetryAfter  time.Duration
}

func (e *APIError) Error() string {
	if e.Description != "" {
		return e.Description
	}
	if e.Code != "" {
		return e.Code
	}
	return fmt.Sprintf("EQLDB returned HTTP %d", e.Status)
}

func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) StartConnection(ctx context.Context, deviceName string) (DeviceAuthorization, error) {
	requestBody := struct {
		ClientID   string `json:"client_id"`
		DeviceName string `json:"device_name"`
	}{ClientID: ClientID, DeviceName: deviceName}
	var response struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := c.postJSON(ctx, "/api/v1/device/connect/", requestBody, &response); err != nil {
		return DeviceAuthorization{}, err
	}
	if response.DeviceCode == "" || response.UserCode == "" || response.VerificationURIComplete == "" || response.ExpiresIn <= 0 {
		return DeviceAuthorization{}, errors.New("EQLDB returned an incomplete connection response")
	}
	interval := time.Duration(response.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return DeviceAuthorization{
		DeviceCode:              response.DeviceCode,
		UserCode:                response.UserCode,
		VerificationURI:         response.VerificationURI,
		VerificationURIComplete: response.VerificationURIComplete,
		ExpiresAt:               time.Now().Add(time.Duration(response.ExpiresIn) * time.Second),
		Interval:                interval,
	}, nil
}

func (c *Client) WaitForToken(ctx context.Context, authorization DeviceAuthorization) (Token, error) {
	interval := authorization.Interval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		if !authorization.ExpiresAt.IsZero() && time.Now().After(authorization.ExpiresAt) {
			return Token{}, &APIError{Code: "expired_token", Description: "The EQLDB connection code expired."}
		}
		if err := wait(ctx, interval); err != nil {
			return Token{}, err
		}
		token, err := c.requestToken(ctx, authorization.DeviceCode)
		if err == nil {
			return token, nil
		}
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			return Token{}, err
		}
		switch apiErr.Code {
		case "authorization_pending":
			continue
		case "slow_down":
			if apiErr.RetryAfter > interval {
				interval = apiErr.RetryAfter
			} else {
				interval += time.Second
			}
			continue
		default:
			return Token{}, apiErr
		}
	}
}

func (c *Client) requestToken(ctx context.Context, deviceCode string) (Token, error) {
	requestBody := struct {
		ClientID   string `json:"client_id"`
		DeviceCode string `json:"device_code"`
	}{ClientID: ClientID, DeviceCode: deviceCode}
	var response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		ConnectionID string `json:"connection_id"`
	}
	if err := c.postJSON(ctx, "/api/v1/device/token/", requestBody, &response); err != nil {
		return Token{}, err
	}
	if response.AccessToken == "" {
		return Token{}, errors.New("EQLDB returned an empty access token")
	}
	return Token{
		AccessToken:  response.AccessToken,
		TokenType:    response.TokenType,
		Scope:        response.Scope,
		ConnectionID: response.ConnectionID,
	}, nil
}

func (c *Client) UploadInventory(ctx context.Context, accessToken, inventoryPath string, metadata UploadMetadata) (UploadResult, error) {
	file, err := os.Open(inventoryPath)
	if err != nil {
		return UploadResult{}, fmt.Errorf("open inventory export: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("inventory_file", filepath.Base(inventoryPath))
	if err != nil {
		return UploadResult{}, fmt.Errorf("create inventory upload: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return UploadResult{}, fmt.Errorf("read inventory export: %w", err)
	}
	for _, class := range metadata.Classes {
		if err := writer.WriteField("classes[]", class); err != nil {
			return UploadResult{}, fmt.Errorf("add inventory classes: %w", err)
		}
	}
	if metadata.Race != "" {
		if err := writer.WriteField("race", metadata.Race); err != nil {
			return UploadResult{}, fmt.Errorf("add inventory race: %w", err)
		}
	}
	if metadata.Level > 0 {
		if err := writer.WriteField("level", strconv.Itoa(metadata.Level)); err != nil {
			return UploadResult{}, fmt.Errorf("add inventory level: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return UploadResult{}, fmt.Errorf("finish inventory upload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/api/v1/inventory/upload/"), &body)
	if err != nil {
		return UploadResult{}, fmt.Errorf("create inventory request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := c.httpClient().Do(request)
	if err != nil {
		return UploadResult{}, fmt.Errorf("upload inventory: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return UploadResult{}, decodeAPIError(response)
	}
	var result UploadResult
	if err := decodeJSON(response.Body, &result); err != nil {
		return UploadResult{}, fmt.Errorf("decode inventory response: %w", err)
	}
	return result, nil
}

func (c *Client) postJSON(ctx context.Context, path string, requestBody, responseBody any) error {
	data, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient().Do(request)
	if err != nil {
		return fmt.Errorf("contact EQLDB: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeAPIError(response)
	}
	if err := decodeJSON(response.Body, responseBody); err != nil {
		return fmt.Errorf("decode EQLDB response: %w", err)
	}
	return nil
}

func (c *Client) endpoint(path string) string {
	return strings.TrimRight(c.BaseURL, "/") + path
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func decodeAPIError(response *http.Response) error {
	var body struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	_ = decodeJSON(response.Body, &body)
	description := body.ErrorDescription
	if description == "" {
		description = body.Message
	}
	return &APIError{
		Status:      response.StatusCode,
		Code:        body.Error,
		Description: description,
		RetryAfter:  parseRetryAfter(response.Header.Get("Retry-After")),
	}
}

func decodeJSON(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, maxResponse))
	return decoder.Decode(destination)
}

func parseRetryAfter(value string) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if timestamp, err := http.ParseTime(value); err == nil {
		return max(time.Until(timestamp), 0)
	}
	return 0
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
