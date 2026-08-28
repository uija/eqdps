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
)

type ItemMetadata struct {
	ID   string         `json:"id"`
	Data map[string]any `json:"data"`
}

func GetItemMetadata(accessToken string, ids ...string) ([]ItemMetadata, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("EQLDB access token is empty")
	}
	if len(ids) < 1 || len(ids) > 500 {
		return nil, errors.New("EQLDB item metadata request must contain 1 to 500 IDs")
	}
	for index, id := range ids {
		if !validItemID(id) {
			return nil, fmt.Errorf("EQLDB item metadata ID %d is invalid: %q", index, id)
		}
	}

	body, err := json.Marshal(struct {
		IDs []string `json:"ids"`
	}{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("encode item metadata request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/api/v1/items/metadata/",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create item metadata request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("retrieve item metadata: %w", err)
	}
	defer response.Body.Close()

	limitedBody := io.LimitReader(response.Body, maxResponseSize)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, decodeAPIError(response, limitedBody)
	}

	var result []ItemMetadata
	if err := json.NewDecoder(limitedBody).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode item metadata response: %w", err)
	}
	if len(result) != len(ids) {
		return nil, fmt.Errorf(
			"item metadata response contains %d entries, expected %d",
			len(result),
			len(ids),
		)
	}
	for index := range result {
		if result[index].ID != ids[index] {
			return nil, fmt.Errorf(
				"item metadata response ID %d is %q, expected %q",
				index,
				result[index].ID,
				ids[index],
			)
		}
	}
	return result, nil
}

func validItemID(id string) bool {
	if len(id) < 1 || len(id) > 20 {
		return false
	}
	positive := false
	for _, character := range id {
		if character < '0' || character > '9' {
			return false
		}
		if character != '0' {
			positive = true
		}
	}
	return positive
}
