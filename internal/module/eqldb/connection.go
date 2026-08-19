package eqldb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/uija/eqdps/internal/native"
)

const (
	eqldbBaseURL          = "https://eqldb.org"
	eqldbClientID         = "eql-log-parser"
	eqldbMaxResponseBytes = 1024 * 1024

	connectionIdle       = -1
	connectionRequesting = 0
	connectionWaiting    = 1
	connectionConnected  = 2
	connectionError      = 3
)

var eqldbHTTPClient = &http.Client{Timeout: 30 * time.Second}

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

func (m *Module) StartConnection() {
	m.mu.Lock()
	if m.process_stage == connectionRequesting || m.process_stage == connectionWaiting {
		m.mu.Unlock()
		return
	}
	m.process_stage = connectionRequesting
	m.authorization = DeviceAuthorization{}
	m.connection_error = ""
	ctx, cancel := context.WithCancel(context.Background())
	m.connection_cancel = cancel
	m.mu.Unlock()
	m.invalidateView()

	go func() {
		defer cancel()
		m.requestConnection(ctx)
	}()
}

func (m *Module) requestConnection(ctx context.Context) {
	requestContext, cancelRequest := context.WithTimeout(ctx, 30*time.Second)
	authorization, err := requestDeviceAuthorization(requestContext, eqldbHTTPClient)
	cancelRequest()

	m.mu.Lock()
	if err != nil {
		m.process_stage = connectionError
		m.connection_error = err.Error()
		m.connection_cancel = nil
		m.mu.Unlock()
		m.invalidateView()
		return
	}
	m.process_stage = connectionWaiting
	m.authorization = authorization
	m.mu.Unlock()
	m.invalidateView()
	if err := native.OpenURL(authorization.VerificationURIComplete); err != nil {
		m.mu.Lock()
		m.connection_error = fmt.Sprintf("Could not open the browser: %v", err)
		m.mu.Unlock()
		m.invalidateView()
	}

	token, err := waitForToken(ctx, eqldbHTTPClient, authorization)
	if errors.Is(err, context.Canceled) {
		return
	}

	m.mu.Lock()
	m.connection_cancel = nil
	if err != nil {
		m.process_stage = connectionError
		m.connection_error = err.Error()
		m.mu.Unlock()
		m.invalidateView()
		return
	}
	m.ctx.Config.EQLDbConfig.AccessToken = token.AccessToken
	m.ctx.Config.EQLDbConfig.AuthorizationTime = time.Now()
	m.process_stage = connectionConnected
	m.connection_error = ""
	m.mu.Unlock()
	if err := m.ctx.Config.Save(); err != nil {
		m.mu.Lock()
		m.connection_error = fmt.Sprintf("save EQLDB access token: %v", err)
		m.mu.Unlock()
	}
	m.invalidateView()
}

func requestDeviceAuthorization(ctx context.Context, client *http.Client) (DeviceAuthorization, error) {
	requestBody := struct {
		ClientID   string `json:"client_id"`
		DeviceName string `json:"device_name"`
	}{
		ClientID:   eqldbClientID,
		DeviceName: "Desktop PC",
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return DeviceAuthorization{}, fmt.Errorf("encode EQLDB connection request: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(eqldbBaseURL, "/")+"/api/v1/device/connect/",
		bytes.NewReader(body),
	)
	if err != nil {
		return DeviceAuthorization{}, fmt.Errorf("create EQLDB connection request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return DeviceAuthorization{}, fmt.Errorf("contact EQLDB: %w", err)
	}
	defer response.Body.Close()

	limitedBody := io.LimitReader(response.Body, eqldbMaxResponseBytes)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return DeviceAuthorization{}, decodeAPIError(response, limitedBody)
	}

	var result struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := json.NewDecoder(limitedBody).Decode(&result); err != nil {
		return DeviceAuthorization{}, fmt.Errorf("decode EQLDB connection response: %w", err)
	}
	if result.DeviceCode == "" || result.UserCode == "" || result.VerificationURIComplete == "" || result.ExpiresIn <= 0 {
		return DeviceAuthorization{}, errors.New("EQLDB returned an incomplete connection response")
	}

	interval := time.Duration(result.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return DeviceAuthorization{
		DeviceCode:              result.DeviceCode,
		UserCode:                result.UserCode,
		VerificationURI:         result.VerificationURI,
		VerificationURIComplete: result.VerificationURIComplete,
		ExpiresAt:               time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
		Interval:                interval,
	}, nil
}

func waitForToken(ctx context.Context, client *http.Client, authorization DeviceAuthorization) (Token, error) {
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

		token, err := requestToken(ctx, client, authorization.DeviceCode)
		if err == nil {
			return token, nil
		}
		var apiError *APIError
		if !errors.As(err, &apiError) {
			return Token{}, err
		}
		switch apiError.Code {
		case "authorization_pending":
			continue
		case "slow_down":
			if apiError.RetryAfter > interval {
				interval = apiError.RetryAfter
			} else {
				interval += time.Second
			}
			continue
		default:
			return Token{}, apiError
		}
	}
}

func requestToken(ctx context.Context, client *http.Client, deviceCode string) (Token, error) {
	requestBody := struct {
		ClientID   string `json:"client_id"`
		DeviceCode string `json:"device_code"`
	}{ClientID: eqldbClientID, DeviceCode: deviceCode}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return Token{}, fmt.Errorf("encode EQLDB token request: %w", err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(eqldbBaseURL, "/")+"/api/v1/device/token/",
		bytes.NewReader(body),
	)
	if err != nil {
		return Token{}, fmt.Errorf("create EQLDB token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return Token{}, fmt.Errorf("contact EQLDB: %w", err)
	}
	defer response.Body.Close()
	limitedBody := io.LimitReader(response.Body, eqldbMaxResponseBytes)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Token{}, decodeAPIError(response, limitedBody)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
		ConnectionID string `json:"connection_id"`
	}
	if err := json.NewDecoder(limitedBody).Decode(&result); err != nil {
		return Token{}, fmt.Errorf("decode EQLDB token response: %w", err)
	}
	if result.AccessToken == "" {
		return Token{}, errors.New("EQLDB returned an empty access token")
	}
	return Token{
		AccessToken:  result.AccessToken,
		TokenType:    result.TokenType,
		Scope:        result.Scope,
		ConnectionID: result.ConnectionID,
	}, nil
}

func decodeAPIError(response *http.Response, body io.Reader) error {
	var result struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		Message          string `json:"message"`
	}
	_ = json.NewDecoder(body).Decode(&result)
	description := result.ErrorDescription
	if description == "" {
		description = result.Message
	}
	return &APIError{
		Status:      response.StatusCode,
		Code:        result.Error,
		Description: description,
		RetryAfter:  parseRetryAfter(response.Header.Get("Retry-After")),
	}
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

func (m *Module) invalidateView() {
	if m.invalidate != nil {
		m.invalidate()
	}

}
