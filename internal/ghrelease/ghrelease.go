// Package ghrelease resolves release tokens and downloads release assets with
// progress callbacks. GitHub Releases
// (https://github.com/Muxcore-Media/<repo>/releases) is the sole origin.
package ghrelease

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Org is the GitHub org that hosts releases.
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

// SaveToken persists a GitHub token the user pasted in.
func SaveToken(tok string) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "muxcore")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "github.token"), []byte(tok+"\n"), 0o600)
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// GitHubAssetURL builds the GitHub Releases download URL.
func GitHubAssetURL(repo, tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", Org(), repo, tag, asset)
}

// AssetURL returns the direct GitHub Releases download URL.
func AssetURL(repo, tag, asset string) string {
	return GitHubAssetURL(repo, tag, asset)
}

// ProgressFunc reports (bytesRead, totalBytes) periodically; total may be 0
// when the server did not send Content-Length.
type ProgressFunc func(read, total int64)

// Download fetches url into dest. Returns the HTTP status of the final attempt.
func Download(url, dest string, token string, progress ProgressFunc) (int, error) {
	return download(url, dest, token, progress)
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

// DownloadReleaseAsset downloads a release asset from GitHub Releases, falling
// back to the authenticated API endpoint for private assets when a token is set.
func DownloadReleaseAsset(repo, tag, asset, dest string, progress ProgressFunc) (int, error) {
	ghToken := Token()
	code, err := Download(GitHubAssetURL(repo, tag, asset), dest, ghToken, progress)
	if err == nil && code == 200 {
		return code, nil
	}
	if ghToken != "" {
		if id, idErr := AssetIDByName(repo, tag, asset, ghToken); idErr == nil {
			code, err = Download(AssetByIDURL(repo, id), dest, ghToken, progress)
		}
	}
	return code, err
}

type releaseAsset struct {
	Name string `json:"name"`
	ID   int64  `json:"id"`
}
type release struct {
	Assets []releaseAsset `json:"assets"`
}

// AssetIDByName looks up a GitHub release asset's numeric ID via the API.
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

// AssetByIDURL is the authenticated GitHub download endpoint for a private asset.
func AssetByIDURL(repo string, id int64) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d", Org(), repo, id)
}

// DownloadSHA256SUMS fetches SHA256SUMS for a release from GitHub Releases.
func DownloadSHA256SUMS(repo, tag, dest string) error {
	code, err := DownloadReleaseAsset(repo, tag, "SHA256SUMS", dest, nil)
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("SHA256SUMS for %s@%s: HTTP %d", repo, tag, code)
	}
	info, err := os.Stat(dest)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("SHA256SUMS for %s@%s is missing or empty", repo, tag)
	}
	return nil
}

// VerifySHA256SUMS checks file against a SHA256SUMS manifest (GNU format).
func VerifySHA256SUMS(sumsPath, filePath string) error {
	data, err := os.ReadFile(sumsPath)
	if err != nil {
		return fmt.Errorf("read SHA256SUMS: %w", err)
	}
	name := filepath.Base(filePath)
	var want string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		entryName := fields[1]
		if entryName == name || entryName == "*"+name {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no SHA256 entry for %s in %s", name, sumsPath)
	}
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("checksum mismatch for %s (got %s want %s)", name, got, want)
	}
	return nil
}
