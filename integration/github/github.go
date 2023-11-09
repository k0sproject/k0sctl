package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	k0sversion "github.com/k0sproject/version"
)

const timeOut = time.Second * 10

// Asset describes a github asset
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release describes a github release
type Release struct {
	URL        string  `json:"html_url"`
	TagName    string  `json:"tag_name"`
	PreRelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

func (r *Release) IsNewer(b string) bool {
	this, err := k0sversion.NewVersion(r.TagName)
	if err != nil {
		return false
	}
	other, err := k0sversion.NewVersion(b)
	if err != nil {
		return false
	}
	return this.GreaterThan(other)
}

// LatestK0sBinaryURL returns the url for the latest k0s release by arch and os
func LatestK0sBinaryURL(arch, osKind string, preok bool) (string, error) {
	r, err := k0sversion.LatestByPrerelease(preok)
	if err != nil {
		return "", err
	}
	return r.DownloadURL(osKind, arch), nil
}

// LatestK0sVersion returns the latest k0s version number (without v prefix)
func LatestK0sVersion(preok bool) (string, error) {
	r, err := k0sversion.LatestByPrerelease(preok)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(r.String(), "v"), nil
}

// LatestRelease returns the latest k0sctl version from github
func LatestRelease(preok bool) (Release, error) {
	latestRelease, err := fetchLatestRelease()
	if err != nil {
		return Release{}, fmt.Errorf("failed to fetch the latest release: %w", err)
	}

	if latestRelease.PreRelease && !preok {
		latestRelease, err = fetchLatestNonPrereleaseRelease()
		if err != nil {
			return Release{}, fmt.Errorf("failed to fetch the latest non-prerelease release: %w", err)
		}
	}

	return latestRelease, nil
}

// fetchLatestRelease fetches the latest release from the GitHub API
func fetchLatestRelease() (Release, error) {
	var release Release
	if err := unmarshalURLBody("https://api.github.com/repos/k0sproject/k0sctl/releases/latest", &release); err != nil {
		return Release{}, err
	}
	return release, nil
}

func unmarshalURLBody(url string, o interface{}) error {
	client := &http.Client{
		Timeout: timeOut,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	if resp.Body == nil {
		return fmt.Errorf("nil body")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("backend returned http %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := resp.Body.Close(); err != nil {
		return err
	}

	return json.Unmarshal(body, o)
}

// fetchLatestNonPrereleaseRelease fetches the latest non-prerelease from the GitHub API
func fetchLatestNonPrereleaseRelease() (Release, error) {
	var releases []Release
	if err := unmarshalURLBody("https://api.github.com/repos/k0sproject/k0sctl/releases", &releases); err != nil {
		return Release{}, err
	}

	var versions k0sversion.Collection
	for _, v := range releases {
		if v.PreRelease {
			continue
		}
		if version, err := k0sversion.NewVersion(v.TagName); err == nil {
			versions = append(versions, version)
		}
	}
	sort.Sort(versions)

	latest := versions[len(versions)-1].String()

	for _, v := range releases {
		if v.TagName == latest {
			return v, nil
		}
	}

	return Release{}, fmt.Errorf("no release found")
}
