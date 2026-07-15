package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
)

func fetchLatestRelease(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPIURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "szr-update-check")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("update check failed: %s", resp.Status)
	}
	return decodeLatestRelease(resp.Body)
}

func decodeLatestRelease(body io.Reader) (Release, error) {
	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		return Release{}, err
	}
	if payload.TagName == "" {
		return Release{}, errors.New("update check returned no release tag")
	}
	return Release{Version: payload.TagName, URL: payload.HTMLURL}, nil
}

func runBrewFormulaeUpdate(ctx context.Context, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "brew", "update", "--quiet")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func runBrewUpgrade(ctx context.Context, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "brew", "upgrade", "szr")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func runGoInstallLatest(ctx context.Context, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "go", "install", goInstallRef)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
