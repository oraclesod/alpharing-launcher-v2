package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	sourceVanilla  = "vanilla"
	sourceMegaBit  = "megabit"
	sourceKirkland = "kirkland"
	megaReleaseURL = "https://api.github.com/repos/megabitt01/AlphaRing/releases/latest"
	megaListURL    = "https://api.github.com/repos/megabitt01/AlphaRing/releases?per_page=30"
	kirkReleaseURL = "https://api.github.com/repos/kirklandsig/AlphaRing/releases?per_page=30"
)

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

type release struct {
	TagName     string         `json:"tag_name"`
	PublishedAt time.Time      `json:"published_at"`
	Draft       bool           `json:"draft"`
	Assets      []releaseAsset `json:"assets"`
}

func validSource(source string) bool {
	return source == sourceVanilla || source == sourceMegaBit || source == sourceKirkland
}

func sourceLabel(source string) string {
	switch source {
	case sourceMegaBit:
		return "MegaBit"
	case sourceKirkland:
		return "Kirkland"
	case sourceVanilla:
		return "Vanilla"
	case "unknown":
		return "an unrecognized mod DLL"
	default:
		return "an unavailable installation"
	}
}

func dllAsset(item release) (releaseAsset, bool) {
	for _, asset := range item.Assets {
		if asset.Name == modDLLName && asset.URL != "" {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func newestUsableRelease(items []release) (release, error) {
	var newest release
	for _, item := range items {
		if item.Draft || item.TagName == "" || item.PublishedAt.IsZero() {
			continue
		}
		if _, ok := dllAsset(item); !ok {
			continue
		}
		if newest.PublishedAt.IsZero() || item.PublishedAt.After(newest.PublishedAt) {
			newest = item
		}
	}
	if newest.TagName == "" {
		return release{}, fmt.Errorf("no published AlphaRing release with %s was found", modDLLName)
	}
	return newest, nil
}

func githubGet(client *http.Client, url string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "AlphaRing-Launcher")
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		return nil, fmt.Errorf("GitHub returned %s", response.Status)
	}
	return response, nil
}

func latestRelease(client *http.Client, source string) (release, error) {
	if source != sourceMegaBit && source != sourceKirkland {
		return release{}, fmt.Errorf("unknown mod source %q", source)
	}
	url := megaReleaseURL
	if source == sourceKirkland {
		url = kirkReleaseURL
	}
	response, err := githubGet(client, url)
	if err != nil {
		return release{}, err
	}
	defer response.Body.Close()
	if source == sourceKirkland {
		var items []release
		if err := json.NewDecoder(response.Body).Decode(&items); err != nil {
			return release{}, err
		}
		return newestUsableRelease(items)
	}
	var item release
	if err := json.NewDecoder(response.Body).Decode(&item); err != nil {
		return release{}, err
	}
	if _, ok := dllAsset(item); !ok {
		return release{}, fmt.Errorf("%s was not found in the MegaBit release", modDLLName)
	}
	return item, nil
}

func matchPublishedDLL(hash string) (string, string) {
	client := &http.Client{Timeout: 8 * time.Second}
	for _, candidate := range []struct{ source, url string }{
		{sourceMegaBit, megaListURL},
		{sourceKirkland, kirkReleaseURL},
	} {
		response, err := githubGet(client, candidate.url)
		if err != nil {
			continue
		}
		var items []release
		err = json.NewDecoder(response.Body).Decode(&items)
		_ = response.Body.Close()
		if err != nil {
			continue
		}
		for _, item := range items {
			asset, ok := dllAsset(item)
			if ok && strings.EqualFold(asset.Digest, "sha256:"+hash) {
				return candidate.source, item.TagName
			}
		}
	}
	return "", ""
}

func downloadAsset(client *http.Client, url string) ([]byte, error) {
	response, err := githubGet(client, url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return io.ReadAll(response.Body)
}

func expectedChecksum(client *http.Client, item release, dll releaseAsset) (string, error) {
	if strings.HasPrefix(dll.Digest, "sha256:") {
		return strings.TrimPrefix(dll.Digest, "sha256:"), nil
	}
	for _, asset := range item.Assets {
		if asset.Name == modDLLName+".sha256" {
			contents, err := downloadAsset(client, asset.URL)
			if err != nil {
				return "", err
			}
			fields := strings.Fields(string(contents))
			if len(fields) > 0 {
				return fields[0], nil
			}
		}
	}
	return "", fmt.Errorf("release %s has no SHA-256 checksum for %s", item.TagName, modDLLName)
}

func fetchVerifiedDLL(source string) ([]byte, string, string, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	item, err := latestRelease(client, source)
	if err != nil {
		return nil, "", "", err
	}
	asset, _ := dllAsset(item)
	expected, err := expectedChecksum(client, item, asset)
	if err != nil {
		return nil, "", "", err
	}
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil || len(expectedBytes) != sha256.Size {
		return nil, "", "", fmt.Errorf("release %s has an invalid SHA-256 checksum", item.TagName)
	}
	data, err := downloadAsset(client, asset.URL)
	if err != nil {
		return nil, "", "", err
	}
	actual := sha256.Sum256(data)
	if !strings.EqualFold(expected, hex.EncodeToString(actual[:])) {
		return nil, "", "", fmt.Errorf("checksum mismatch for %s release %s", sourceLabel(source), item.TagName)
	}
	return data, item.TagName, hex.EncodeToString(actual[:]), nil
}

func (a *App) LatestModVersion(source string) (string, error) {
	if source == sourceVanilla {
		return "", nil
	}
	item, err := latestRelease(&http.Client{Timeout: 20 * time.Second}, source)
	if err != nil {
		return "", err
	}
	return item.TagName, nil
}
