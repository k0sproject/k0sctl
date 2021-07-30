package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
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

// LatestK0sBinaryURL returns the url for the latest k0s release by arch and os
func LatestK0sBinaryURL(arch, osKind string, preok bool) (string, error) {
	r, err := LatestRelease("k0sproject/k0s", preok)
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
	r, err := LatestRelease("k0sproject/k0s", preok)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(r.TagName, "v"), nil
}

// versionCollection is a type that implements the sort.Interface interface
// so that versions can be sorted.
type versionCollection []*version.Version

func (v versionCollection) Len() int {
	return len(v)
}

func (v versionCollection) Less(i, j int) bool {
	return vCompare(v[i], v[j]) < 0
}

func (v versionCollection) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

// vCompare compares this version to another version. This
// returns -1, 0, or 1 if this version is smaller, equal,
// or larger than the other version, respectively.
func vCompare(a, b *version.Version) int {
	c := a.Compare(b)
	if c != 0 {
		// versions already differ enough to use the version pkg result
		return c
	}

	vA := a.String()

	// go to plain string comparison
	s := []string{vA, b.String()}
	sort.Strings(s)

	if vA == s[0] {
		return -1
	}

	return 1
}

// LatestRelease returns the semantically sorted latest version from github releases page for a repo.
// Set preok true to allow returning pre-release versions.  Assumes the repository has release tags with
// semantic version numbers (optionally v-prefixed).
func LatestRelease(repo string, preok bool) (Release, error) {
	var gotV bool
	var releases []Release
	if err := unmarshalURLBody(fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20&page=1", repo), &releases); err != nil {
		return Release{}, err
	}

	var versions []*version.Version
	for _, v := range releases {
		if v.PreRelease && !preok {
			continue
		}
		if version, err := version.NewVersion(strings.TrimPrefix(v.TagName, "v")); err == nil {
			gotV = strings.HasPrefix(v.TagName, "v")
			versions = append(versions, version)
		}
	}
	vc := versionCollection(versions)
	sort.Sort(vc)

	latest := versions[len(versions)-1].String()
	if gotV {
		latest = "v" + latest
	}

	for _, v := range releases {
		if v.TagName == latest {
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
