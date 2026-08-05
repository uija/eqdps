package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const LatestReleaseURL = "https://api.github.com/repos/uija/eqdps/releases/latest"

type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

func Latest(ctx context.Context, client *http.Client, endpoint string) (Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	response, err := client.Do(request)
	if err != nil {
		return Release{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub release request returned %s", response.Status)
	}

	var release Release
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return Release{}, err
	}
	if release.TagName == "" || release.HTMLURL == "" {
		return Release{}, fmt.Errorf("GitHub release response is incomplete")
	}
	return release, nil
}
