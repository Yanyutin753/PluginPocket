package marketplace

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Git tree entries carry file modes and immutable blob IDs; Contents API does not.
func ResolveSkillFilesV2(ctx context.Context, o Options, repo, skillPath string) (map[string]SkillFile, error) {
	if !repoPattern.MatchString(repo) || !SafeSkillPath(skillPath) {
		return nil, errors.New("invalid skill location")
	}
	base := o.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	client := &http.Client{Timeout: 20 * time.Second}
	get := func(endpoint string, out any, limit int64) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/repos/"+repo+endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "loadout-marketplace")
		if o.Token != "" {
			req.Header.Set("Authorization", "Bearer "+o.Token)
		}
		response, err := client.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != 200 {
			return fmt.Errorf("github files returned %d", response.StatusCode)
		}
		return json.NewDecoder(http.MaxBytesReader(nil, response.Body, limit)).Decode(out)
	}
	var tree struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Mode string `json:"mode"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
			Size int64  `json:"size"`
		} `json:"tree"`
	}
	if err := get("/git/trees/HEAD?recursive=1", &tree, 20<<20); err != nil {
		return nil, err
	}
	if tree.Truncated {
		return nil, errors.New("github tree truncated")
	}
	files := map[string]SkillFile{}
	var total int64
	for _, entry := range tree.Tree {
		if !strings.HasPrefix(entry.Path, skillPath+"/") {
			continue
		}
		name := strings.TrimPrefix(entry.Path, skillPath+"/")
		if !SafeSkillPath(name) {
			return nil, errors.New("unsafe skill path")
		}
		if entry.Type == "tree" && entry.Mode == "040000" {
			continue
		}
		if entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return nil, errors.New("skill links and special files are forbidden")
		}
		total += entry.Size
		sha, err := hex.DecodeString(entry.SHA)
		if err != nil || len(sha) != 20 || entry.Size < 0 || entry.Size > MaxSkillFileBytes || total > MaxSkillBytes || len(files) >= 32 {
			return nil, errors.New("invalid or oversized skill file")
		}
		if _, exists := files[name]; exists {
			return nil, errors.New("duplicate skill path")
		}
		var blob struct {
			Encoding string `json:"encoding"`
			Content  string `json:"content"`
		}
		if err := get("/git/blobs/"+entry.SHA, &blob, 12<<20); err != nil {
			return nil, err
		}
		raw, err := base64.StdEncoding.Strict().DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
		if err != nil || blob.Encoding != "base64" || int64(len(raw)) != entry.Size {
			return nil, errors.New("invalid github blob")
		}
		files[name] = SkillFile{Content: raw, Executable: entry.Mode == "100755"}
	}
	return files, ValidateSkillFiles(files)
}
