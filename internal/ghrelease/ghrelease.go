// Package ghrelease resolves GitHub tokens and downloads release assets with
// progress callbacks, for the module binaries the wizard fetches. Ported from
// lib/github.sh.
package ghrelease

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Org is the GitHub org releases live under.
func Org() string {
	if v := os.Getenv("MUXCORE_GITHUB_ORG"); v != "" {
		return v
	}
	return "Muxcore-Media"
}

// Token resolves a GitHub token from env vars or well-known token files.
func Token() string {
	for _, k := range []string{"GITHUB_TOKEN", "GH_TOKEN", "MUXCORE_GITHUB_TOKEN"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	home, _ := os.UserHomeDir()
	candidates := []string{os.Getenv("MUXCORE_GITHUB_TOKEN_FILE"), filepath.Join(home, ".config", "muxcore", "github.token")}
	for _, f := range candidates {
		if f == "" {
			continue
		}
		if b, err := os.ReadFile(f); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

// SaveToken persists a token the user pasted in, matching the old script's
// ~/.config/muxcore/github.token convention.
func SaveToken(tok string) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "muxcore")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "github.token"), []byte(tok+"\n"), 0o600)
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// AssetURL builds the direct download URL for a named release asset.
func AssetURL(repo, tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", Org(), repo, tag, asset)
}

// ProgressFunc reports (bytesRead, totalBytes) periodically; total may be 0
// when the server did not send Content-Length.
type ProgressFunc func(read, total int64)

// Download fetches url into dest, retrying once with a token on 401/403/404.
// Returns the HTTP status of the final attempt.
func Download(url, dest string, token string, progress ProgressFunc) (int, error) {
	code, err := download(url, dest, token, progress)
	if err == nil && code == 200 {
		return code, nil
	}
	return code, err
}

func download(url, dest, token string, progress ProgressFunc) (int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/octet-stream")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, nil
	}
	tmp := dest + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return resp.StatusCode, err
	}
	total := resp.ContentLength
	var read int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(tmp)
				return resp.StatusCode, werr
			}
			read += int64(n)
			if progress != nil {
				progress(read, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			os.Remove(tmp)
			return resp.StatusCode, rerr
		}
	}
	out.Close()
	if err := os.Rename(tmp, dest); err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

// releaseAsset mirrors the subset of the GitHub API response we need.
type releaseAsset struct {
	Name string `json:"name"`
	ID   int64  `json:"id"`
}
type release struct {
	Assets []releaseAsset `json:"assets"`
}

// AssetIDByName looks up a release asset's numeric ID via the API (needed to
// download private-release assets through the authenticated assets/ID endpoint).
func AssetIDByName(repo, tag, name, token string) (int64, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", Org(), repo, tag)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("github api %s: HTTP %d", url, resp.StatusCode)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return 0, err
	}
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.ID, nil
		}
	}
	return 0, fmt.Errorf("asset %q not found in %s@%s", name, repo, tag)
}

// AssetByIDURL is the authenticated download endpoint for a private asset.
func AssetByIDURL(repo string, id int64) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", Org(), repo, id)
}
