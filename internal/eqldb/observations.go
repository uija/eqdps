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

type KillObservation struct {
	Time      time.Time `json:"-"`
	Timestamp string    `json:"timestamp"`
	Zone      string    `json:"zone"`
	Mob       string    `json:"mob"`
}

func NewKillObservation(timestamp time.Time, zone string, mob string) KillObservation {
	return KillObservation{
		Time:      timestamp,
		Timestamp: timestamp.Format("Mon Jan 02 15:04:05 2006"),
		Zone:      zone,
		Mob:       mob,
	}
}

type DropObservation struct {
	Timestamp string `json:"timestamp"`
	Zone      string `json:"zone"`
	Mob       string `json:"mob"`
	Item      string `json:"item"`
	Amount    int    `json:"amount"`
}

func NewDropObservation(timestamp time.Time, zone string, mob string, item string, amount int) DropObservation {
	return DropObservation{
		Timestamp: timestamp.Format("Mon Jan 02 15:04:05 2006"),
		Zone:      zone,
		Mob:       mob,
		Item:      item,
		Amount:    amount,
	}
}

func UploadKillObservations(accessToken string, events ...KillObservation) error {
	return uploadObservations(
		accessToken,
		"/api/v1/observations/kills/",
		"kill observations",
		events,
	)
}

func UploadDropObservations(accessToken string, events ...DropObservation) error {
	return uploadObservations(
		accessToken,
		"/api/v1/observations/drops/",
		"drop observations",
		events,
	)
}

func uploadObservations[T any](accessToken, endpoint, description string, events []T) error {
	if strings.TrimSpace(accessToken) == "" {
		return errors.New("EQLDB access token is empty")
	}
	if len(events) < 1 || len(events) > 2000 {
		return fmt.Errorf("%s batch must contain 1 to 2000 events", description)
	}

	requestBody := struct {
		Events []T `json:"events"`
	}{
		Events: events,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("encode %s: %w", description, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(baseURL, "/")+endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create %s request: %w", description, err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("upload %s: %w", description, err)
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
		return fmt.Errorf("decode %s response: %w", description, err)
	}
	if !result.Success {
		return fmt.Errorf("EQLDB did not confirm the %s", description)
	}
	return nil
}
