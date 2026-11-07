package gui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// fetchLatestRelease возвращает тег и страницу последнего релиза в GitHub.
func fetchLatestRelease(ctx context.Context) (tag, link string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Veil-update-check")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", "", errors.New(L("no releases yet", "релизов пока нет"))
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf(L("GitHub responded %s", "GitHub ответил %s"), resp.Status)
	}
	var r struct {
		Tag  string `json:"tag_name"`
		HTML string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", "", err
	}
	return r.Tag, r.HTML, nil
}
