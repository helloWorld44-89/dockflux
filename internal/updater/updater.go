package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	githubRepo = "jconder44/dockflux"
	apiURL     = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	cacheTTL   = 24 * time.Hour
)

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

type cacheEntry struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

var refreshOnce sync.Once

// RefreshCacheAsync spawns a background goroutine to update the cached latest version
// if the cache is stale. Safe to call on every invocation; runs at most once per process.
func RefreshCacheAsync() {
	refreshOnce.Do(func() {
		go func() {
			if _, err := latestFromCache(); err == nil {
				return // cache still fresh
			}
			latest, err := FetchLatest()
			if err != nil {
				return
			}
			_ = writeCache(latest)
		}()
	})
}

// CheckForUpdate returns the latest release tag if it is newer than current,
// or "" if already up to date. Reads only from the local cache — call
// RefreshCacheAsync to keep it warm.
func CheckForUpdate(current string) string {
	latest, err := latestFromCache()
	if err != nil {
		return ""
	}
	if IsNewer(current, latest) {
		return latest
	}
	return ""
}

// FetchLatest hits the GitHub releases API and returns the latest tag.
func FetchLatest() (string, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// IsNewer reports whether latest is a higher semver than current.
func IsNewer(current, latest string) bool {
	c := parseSemver(current)
	l := parseSemver(latest)
	if c == nil || l == nil {
		return false
	}
	for i := range c {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}

// Apply downloads the given release version, verifies its SHA256 checksum,
// and atomically replaces the running binary.
func Apply(version string) error {
	assetName := fmt.Sprintf("dockflux-%s-%s", runtime.GOOS, runtime.GOARCH)
	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", githubRepo, version)

	expected, err := fetchChecksum(baseURL+"/checksums.txt", assetName)
	if err != nil {
		return fmt.Errorf("fetch checksums: %w", err)
	}

	data, err := httpGet(baseURL + "/" + assetName)
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}

	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return fmt.Errorf("checksum mismatch — aborting update")
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	tmp := exe + ".new"
	if err := os.WriteFile(tmp, data, 0755); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w\nhint: try 'sudo dockflux update'", err)
	}
	return nil
}

func fetchChecksum(url, assetName string) (string, error) {
	data, err := httpGet(url)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s", assetName)
}

func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		p = strings.SplitN(p, "-", 2)[0] // strip pre-release suffix
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}

func cacheDir() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(d, "dockflux")
	return dir, os.MkdirAll(dir, 0755)
}

func latestFromCache() (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "update_check.json"))
	if err != nil {
		return "", err
	}
	var c cacheEntry
	if err := json.Unmarshal(data, &c); err != nil {
		return "", err
	}
	if time.Since(c.CheckedAt) > cacheTTL {
		return "", fmt.Errorf("cache expired")
	}
	return c.Latest, nil
}

func writeCache(latest string) error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	data, err := json.Marshal(cacheEntry{CheckedAt: time.Now(), Latest: latest})
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "update_check.json.tmp")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "update_check.json"))
}
