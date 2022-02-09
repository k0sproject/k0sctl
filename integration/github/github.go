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
	r, err := k0sversion.LatestReleaseByPrerelease(preok)
	if err != nil {
		return "", err
	}

	for _, a := range r.Assets {
		if !strings.Contains(a.Name, "-"+arch) {
			continue
		}

		if strings.HasSuffix(a.Name, ".exe") {
			if osKind == "windows" {
				return a.URL, nil
			}
		} else if osKind != "windows" {
			return a.URL, nil
		}
	}

	return "", fmt.Errorf("failed to find a k0s release")
}

// LatestK0sVersion returns the latest k0s version number (without v prefix)
func LatestK0sVersion(preok bool) (string, error) {
	r, err := k0sversion.LatestReleaseByPrerelease(preok)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(r.TagName, "v"), nil
}

// LatestRelease returns the semantically sorted latest k0sctl version from github
func LatestRelease(preok bool) (Release, error) {
	var releases []Release
	if err := unmarshalURLBody("https://api.github.com/repos/k0sproject/k0sctl/releases?per_page=20&page=1", &releases); err != nil {
		return Release{}, err
	}

	var versions k0sversion.Collection
	for _, v := range releases {
		if v.PreRelease && !preok {
			continue
		}
		if version, err := k0sversion.NewVersion(strings.TrimPrefix(v.TagName, "v")); err == nil {
			versions = append(versions, version)
		}
	}
	sort.Sort(versions)

	latest := versions[len(versions)-1].String()

	for _, v := range releases {
		if strings.TrimPrefix(v.TagName, "v") == latest {
			return v, nil
		}
	}

	return Release{}, fmt.Errorf("failed to get the latest version information")
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
