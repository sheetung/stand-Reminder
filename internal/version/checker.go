package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// GitHubAPIURL is the GitHub API endpoint for releases
	GitHubAPIURL = "https://api.github.com/repos/sheetung/stand-Reminder/releases/latest"
	// Timeout for version check requests
	requestTimeout = 5 * time.Second
)

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	HasUpdate      bool   `json:"has_update"`      // Whether a newer version is available
	CurrentVersion string `json:"current_version"` // Current running version
	LatestVersion  string `json:"latest_version"`  // Latest available version
	ReleaseURL     string `json:"release_url"`     // URL to download the latest release
	ReleaseNotes   string `json:"release_notes"`   // Release notes/changelog
}

// GitHubRelease represents a GitHub API release response
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

// CheckUpdate fetches the latest release from GitHub and compares with current version
func CheckUpdate(currentVersion string) UpdateInfo {
	info := UpdateInfo{
		CurrentVersion: currentVersion,
		HasUpdate:      false,
	}

	client := &http.Client{
		Timeout: requestTimeout,
	}

	resp, err := client.Get(GitHubAPIURL)
	if err != nil {
		// Silently fail - network error or timeout
		info.LatestVersion = currentVersion
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		info.LatestVersion = currentVersion
		return info
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		info.LatestVersion = currentVersion
		return info
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		info.LatestVersion = currentVersion
		return info
	}

	info.LatestVersion = release.TagName
	info.ReleaseURL = release.HTMLURL
	info.ReleaseNotes = release.Body

	// Compare versions (simple semver comparison)
	// "v0.6.0" > "v0.5.2" == true
	if compareVersions(release.TagName, currentVersion) > 0 {
		info.HasUpdate = true
	}

	return info
}

// compareVersions compares two semantic versions
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if v1 == v2
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Pad shorter version with zeros
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int

		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &p1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &p2)
		}

		if p1 > p2 {
			return 1
		}
		if p1 < p2 {
			return -1
		}
	}

	return 0
}
