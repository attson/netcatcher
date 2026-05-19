package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrNotModified is returned by fetchLatest when the server replied 304.
var ErrNotModified = errors.New("not modified")

// ErrNoAssetForPlatform is returned when the release has no asset matching
// the running OS/arch. The error message names the expected filename so
// the UI can show an actionable message.
type ErrNoAssetForPlatform struct {
	Want string
}

func (e *ErrNoAssetForPlatform) Error() string {
	return fmt.Sprintf("release has no asset for this platform; expected %s", e.Want)
}

// errRateLimited is returned when GitHub responds 403 with
// X-RateLimit-Remaining: 0.
var errRateLimited = errors.New("github api rate limited")

func isRateLimit(err error) bool { return errors.Is(err, errRateLimited) }

// Asset is a single file attached to a release.
type Asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"browser_download_url"`
}

// Release is the subset of the GitHub Release payload the updater needs.
type Release struct {
	TagName string  `json:"tag_name"`
	Body    string  `json:"body"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// AssetForPlatform returns the asset whose name matches the convention
// NetCatcher-<arch>.app.tar.gz on darwin and NetCatcher-<arch>.exe on windows.
func (r *Release) AssetForPlatform(goos, goarch string) (Asset, error) {
	want := PlatformAssetName(goos, goarch)
	for _, a := range r.Assets {
		if a.Name == want {
			return a, nil
		}
	}
	return Asset{}, &ErrNoAssetForPlatform{Want: want}
}

// ChecksumAssets returns the SHA256SUMS and SHA256SUMS.sig assets, or an
// error if either is missing.
func (r *Release) ChecksumAssets() (sums Asset, sig Asset, err error) {
	var foundSums, foundSig bool
	for _, a := range r.Assets {
		switch a.Name {
		case "SHA256SUMS":
			sums = a
			foundSums = true
		case "SHA256SUMS.sig":
			sig = a
			foundSig = true
		}
	}
	if !foundSums || !foundSig {
		return Asset{}, Asset{}, errors.New("release missing SHA256SUMS or SHA256SUMS.sig")
	}
	return sums, sig, nil
}

// PlatformAssetName returns the asset filename convention used by the
// release workflow. Keep in sync with `.github/workflows/release.yaml`.
func PlatformAssetName(goos, goarch string) string {
	if goos == "windows" {
		return fmt.Sprintf("NetCatcher-%s.exe", goarch)
	}
	return fmt.Sprintf("NetCatcher-%s.app.tar.gz", goarch)
}

type githubClient struct {
	repo    string
	http    *http.Client
	baseURL string // override for tests; defaults to https://api.github.com
}

func newGitHubClient(repo string, httpc *http.Client, baseURL string) *githubClient {
	if httpc == nil {
		httpc = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &githubClient{repo: repo, http: httpc, baseURL: baseURL}
}

// fetchLatest hits /repos/<repo>/releases/latest. If etag is non-empty and
// the server replies 304, ErrNotModified is returned. On success the new
// ETag header value is returned alongside the parsed release.
func (c *githubClient) fetchLatest(ctx context.Context, etag string) (*Release, string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, c.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return nil, etag, ErrNotModified
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, "", errRateLimited
		}
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("github 403: %s", string(body))
	case http.StatusOK:
		// fallthrough
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("github %d: %s", resp.StatusCode, string(body))
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, "", fmt.Errorf("decode release: %w", err)
	}
	return &rel, resp.Header.Get("Etag"), nil
}
