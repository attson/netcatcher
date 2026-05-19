package updater

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newGHServer(t *testing.T, body any, statusCode int, headers map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		if etag := r.Header.Get("If-None-Match"); etag != "" && etag == headers["Etag"] {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestGitHubLatestSelectsMatchingAsset(t *testing.T) {
	body := map[string]any{
		"tag_name":   "v1.5.0",
		"body":       "release notes",
		"html_url":   "https://github.com/attson/netcatcher/releases/tag/v1.5.0",
		"assets": []map[string]any{
			{"name": "NetCatcher-amd64.app.tar.gz", "size": 12345, "browser_download_url": "https://example.test/a"},
			{"name": "NetCatcher-arm64.app.tar.gz", "size": 67890, "browser_download_url": "https://example.test/b"},
			{"name": "SHA256SUMS", "size": 256, "browser_download_url": "https://example.test/sums"},
			{"name": "SHA256SUMS.sig", "size": 88, "browser_download_url": "https://example.test/sig"},
		},
	}
	srv := newGHServer(t, body, 200, map[string]string{"Etag": "abc123"})
	defer srv.Close()

	c := newGitHubClient("attson/netcatcher", srv.Client(), srv.URL)
	rel, _, err := c.fetchLatest(context.Background(), "")
	if err != nil {
		t.Fatalf("fetchLatest: %v", err)
	}
	if rel.TagName != "v1.5.0" {
		t.Fatalf("tag: %q", rel.TagName)
	}

	asset, err := rel.AssetForPlatform("darwin", "arm64")
	if err != nil {
		t.Fatalf("AssetForPlatform: %v", err)
	}
	if asset.Name != "NetCatcher-arm64.app.tar.gz" {
		t.Fatalf("got %q", asset.Name)
	}

	sums, sig, err := rel.ChecksumAssets()
	if err != nil {
		t.Fatalf("ChecksumAssets: %v", err)
	}
	if sums.Name != "SHA256SUMS" || sig.Name != "SHA256SUMS.sig" {
		t.Fatalf("unexpected checksum assets: %s / %s", sums.Name, sig.Name)
	}
}

func TestGitHubLatestRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newGitHubClient("attson/netcatcher", srv.Client(), srv.URL)
	_, _, err := c.fetchLatest(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error on 403")
	}
	if !isRateLimit(err) {
		t.Fatalf("expected rate-limit error, got %v", err)
	}
}

func TestGitHubLatestETagNotModified(t *testing.T) {
	body := map[string]any{"tag_name": "v1.0.0"}
	srv := newGHServer(t, body, 200, map[string]string{"Etag": "etag-1"})
	defer srv.Close()

	c := newGitHubClient("attson/netcatcher", srv.Client(), srv.URL)
	_, etag, err := c.fetchLatest(context.Background(), "")
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if etag != "etag-1" {
		t.Fatalf("got etag %q", etag)
	}

	_, _, err = c.fetchLatest(context.Background(), "etag-1")
	if err != ErrNotModified {
		t.Fatalf("expected ErrNotModified, got %v", err)
	}
}

func TestAssetForPlatformMissingReturnsTypedError(t *testing.T) {
	rel := &Release{
		TagName: "v1.0.0",
		Assets:  []Asset{{Name: "SHA256SUMS"}},
	}
	_, err := rel.AssetForPlatform("darwin", "arm64")
	if err == nil {
		t.Fatalf("expected ErrNoAssetForPlatform, got nil")
	}
	var typed *ErrNoAssetForPlatform
	if !errors.As(err, &typed) {
		t.Fatalf("expected *ErrNoAssetForPlatform, got %T: %v", err, err)
	}
	if typed.Want != "NetCatcher-arm64.app.tar.gz" {
		t.Fatalf("Want = %q", typed.Want)
	}
}
