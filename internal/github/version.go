package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/uija/eqdps/releases/latest"

type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

// CheckNewVersion checks the latest published GitHub release. If its tag is
// different from currentVersion, it returns the release and true.
func CheckNewVersion(currentVersion string) (Release, bool, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	return checkNewVersion(client, latestReleaseURL, currentVersion)
}

func checkNewVersion(client *http.Client, endpoint, currentVersion string) (Release, bool, error) {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, false, fmt.Errorf("create GitHub release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := client.Do(request)
	if err != nil {
		return Release{}, false, fmt.Errorf("check GitHub release: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return Release{}, false, fmt.Errorf("GitHub release request returned %s", response.Status)
	}

	var release Release
	if err := json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&release); err != nil {
		return Release{}, false, fmt.Errorf("decode GitHub release response: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return Release{}, false, fmt.Errorf("GitHub release response has no tag_name")
	}

	if strings.TrimSpace(currentVersion) == strings.TrimSpace(release.TagName) {
		return Release{}, false, nil
	}
	return release, true, nil
}
