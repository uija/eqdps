package eqldb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	upload_idle      = -1
	upload_detected  = 2
	upload_uploading = 3
	upload_success   = 4
	upload_error     = 5
)

type UploadResult struct {
	Status     string `json:"status"`
	Character  string `json:"character"`
	Server     string `json:"server"`
	ProfileURL string `json:"profile_url"`
	Message    string `json:"message"`
}

// UploadFile snapshots the pending export while CheckUploadData holds m.mu and
// performs the actual upload asynchronously.
func (m *Module) UploadFile() {
	if m.last_export == nil {
		return
	}
	accessToken := strings.TrimSpace(m.ctx.Config.EQLDbConfig.AccessToken)
	if accessToken == "" {
		log.Printf("EQLDB inventory upload skipped: eqdps is not connected")
		return
	}
	export := *m.last_export
	logPath := m.current_logpath

	inventoryPath, err := inventoryExportPath(logPath, export.Filename)
	if err != nil {
		log.Printf("EQLDB inventory upload skipped: %v", err)
		return
	}

	var classInfo *ClassInfo
	if m.last_who_result != nil {
		copy := *m.last_who_result
		copy.Classes = append([]string(nil), m.last_who_result.Classes...)
		classInfo = &copy
	}
	m.upload_status = upload_uploading
	m.invalidate()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := uploadInventory(ctx, eqldbHTTPClient, accessToken, inventoryPath, classInfo)
		m.upload_timer = time.Now()
		if err != nil {
			m.handleInventoryUploadError(accessToken, err)
			m.upload_status = upload_error
			m.invalidate()
			return
		}
		m.upload_status = upload_success
		m.invalidate()
		log.Printf("EQLDB inventory uploaded for %s on %s", result.Character, result.Server)
	}()
}

func inventoryExportPath(logPath, filename string) (string, error) {
	if logPath == "" {
		return "", errors.New("no EverQuest log file is open")
	}
	if filename == "" || filepath.Base(filename) != filename {
		return "", fmt.Errorf("invalid inventory filename %q", filename)
	}

	// EverQuest stores inventory exports in its root directory and log files in
	// the Logs subdirectory.
	return filepath.Join(filepath.Dir(filepath.Dir(logPath)), filename), nil
}

func uploadInventory(
	ctx context.Context,
	client *http.Client,
	accessToken string,
	inventoryPath string,
	classInfo *ClassInfo,
) (UploadResult, error) {
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

	if classInfo != nil {
		for _, class := range classInfo.Classes {
			if err := writer.WriteField("classes[]", class); err != nil {
				return UploadResult{}, fmt.Errorf("add inventory classes: %w", err)
			}
		}
		if classInfo.Race != "" {
			if err := writer.WriteField("race", classInfo.Race); err != nil {
				return UploadResult{}, fmt.Errorf("add inventory race: %w", err)
			}
		}
		if classInfo.Level > 0 {
			if err := writer.WriteField("level", strconv.Itoa(classInfo.Level)); err != nil {
				return UploadResult{}, fmt.Errorf("add inventory level: %w", err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		return UploadResult{}, fmt.Errorf("finish inventory upload: %w", err)
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(eqldbBaseURL, "/")+"/api/v1/inventory/upload/",
		&body,
	)
	if err != nil {
		return UploadResult{}, fmt.Errorf("create inventory request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := client.Do(request)
	if err != nil {
		return UploadResult{}, fmt.Errorf("upload inventory: %w", err)
	}
	defer response.Body.Close()

	limitedBody := io.LimitReader(response.Body, eqldbMaxResponseBytes)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return UploadResult{}, decodeAPIError(response, limitedBody)
	}

	var result UploadResult
	if err := json.NewDecoder(limitedBody).Decode(&result); err != nil {
		return UploadResult{}, fmt.Errorf("decode inventory response: %w", err)
	}
	return result, nil
}

func (m *Module) handleInventoryUploadError(accessToken string, err error) {
	log.Printf("EQLDB inventory upload failed: %v", err)

	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Status != http.StatusUnauthorized {
		return
	}

	m.mu.Lock()
	if m.ctx.Config.EQLDbConfig.AccessToken == accessToken {
		m.ctx.Config.EQLDbConfig.AccessToken = ""
		m.ctx.Config.EQLDbConfig.AuthorizationTime = time.Time{}
		if saveErr := m.ctx.Config.Save(); saveErr != nil {
			log.Printf("Unable to remove revoked EQLDB token: %v", saveErr)
		}
	}
	m.mu.Unlock()
	m.invalidateView()
}
