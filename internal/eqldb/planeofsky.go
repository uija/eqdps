package eqldb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	baseURL         = "https://eqldb.org"
	maxResponseSize = 1024 * 1024
	requestTimeout  = 30 * time.Second
)

const (
	PlaneOfSkyEventTypeWindRuneReceive = "wind-rune-receive"
	PlaneOfSkyEventTypeWindRuneDelete  = "wind-rune-delete"
	PlaneOfSkyEventTypeQuestTurnIn     = "quest-turn-in"
)

var httpClient = &http.Client{Timeout: requestTimeout}

type PlaneOfSkyEvent struct {
	Type      string `json:"type"`
	Rune      string `json:"rune,omitempty"`
	Amount    int    `json:"amount,omitempty"`
	Quest     string `json:"quest,omitempty"`
	Timestamp string `json:"timestamp"`
}

type APIError struct {
	Status      int
	Code        string
	Description string
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

func IsUnauthorized(err error) bool {
	var apiError *APIError
	return errors.As(err, &apiError) && apiError.Status == http.StatusUnauthorized
}

func UploadPlaneOfSkyEvents(
	accessToken string,
	characterName string,
	server string,
	events ...PlaneOfSkyEvent,
) error {
	if strings.TrimSpace(accessToken) == "" {
		return errors.New("EQLDB access token is empty")
	}
	if strings.TrimSpace(characterName) == "" || strings.TrimSpace(server) == "" {
		return errors.New("EQLDB character and server are required")
	}
	if len(events) < 1 || len(events) > 2000 {
		return errors.New("Plane of Sky event batch must contain 1 to 2000 events")
	}

	requestBody := struct {
		Character string            `json:"character"`
		Server    string            `json:"server"`
		Events    []PlaneOfSkyEvent `json:"events"`
	}{
		Character: characterName,
		Server:    server,
		Events:    events,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("encode Plane of Sky events: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/api/v1/plane-of-sky/events/",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create Plane of Sky events request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("upload Plane of Sky events: %w", err)
	}
	defer response.Body.Close()

	limitedBody := io.LimitReader(response.Body, maxResponseSize)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return decodeAPIError(response, limitedBody)
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(limitedBody).Decode(&result); err != nil {
		return fmt.Errorf("decode Plane of Sky events response: %w", err)
	}
	if !result.Success {
		return errors.New("EQLDB did not confirm the Plane of Sky events")
	}
	return nil
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
	}
}
