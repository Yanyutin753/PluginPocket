// Package marketplace 同步 GitHub 上热门的 MCP server 仓库为市场条目。
package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const DefaultBaseURL = "https://api.github.com"

type Options struct {
	BaseURL string // GitHub API 根地址，默认 https://api.github.com；测试注入 fixture
	Token   string // 可选 Bearer 令牌，提升搜索限流额度
}

// Item 是一次同步产出的候选条目（传输方式未知，安装时由管理员显式提供）。
type Item struct {
	Slug, Name, Description, RepoURL, Homepage string
	Stars                                      int
}

// Fetch 按星标排序发现热门 MCP server 仓库。
func Fetch(ctx context.Context, o Options) ([]Item, error) {
	base := o.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/search/repositories?q=mcp-server+in%3Aname+topic%3Amcp-server&sort=stars&order=desc&per_page=30", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "loadout-marketplace")
	if o.Token != "" {
		req.Header.Set("Authorization", "Bearer "+o.Token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github search returned %d", response.StatusCode)
	}
	var payload struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Name        string `json:"name"`
			Description string `json:"description"`
			HTMLURL     string `json:"html_url"`
			Homepage    string `json:"homepage"`
			Stars       int    `json:"stargazers_count"`
		} `json:"items"`
	}
	if err = json.NewDecoder(http.MaxBytesReader(nil, response.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(payload.Items))
	for _, repo := range payload.Items {
		if repo.FullName == "" || repo.HTMLURL == "" {
			continue
		}
		items = append(items, Item{
			Slug: slug(repo.FullName), Name: repo.Name, Description: repo.Description,
			RepoURL: repo.HTMLURL, Homepage: repo.Homepage, Stars: repo.Stars,
		})
	}
	return items, nil
}

// slug 把 "owner/repo" 折叠进 tools.key 兼容的 [a-zA-Z0-9_-]{3,32}。
func slug(repo string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(repo) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	if len(out) > 32 {
		sum := sha256.Sum256([]byte(repo))
		out = out[:26] + "-" + hex.EncodeToString(sum[:])[:5]
	}
	return out
}

// Sync 拉取并按 slug upsert；只更新 github 来源的行，curated 条目不被覆盖。返回写入行数。
func Sync(ctx context.Context, pool *pgxpool.Pool, o Options) (int, error) {
	items, err := Fetch(ctx, o)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, errors.New("github search returned no repositories")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	count := 0
	for _, item := range items {
		var id int64
		err = tx.QueryRow(ctx, `INSERT INTO marketplace_items(slug,name,description,source,repo_url,homepage,transport,stars,synced_at)
		 VALUES($1,$2,$3,'github',$4,$5,'unknown',$6,now())
		 ON CONFLICT(slug) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,repo_url=EXCLUDED.repo_url,homepage=EXCLUDED.homepage,stars=EXCLUDED.stars,synced_at=now(),updated_at=now()
		 WHERE marketplace_items.source='github' RETURNING id`,
			item.Slug, item.Name, item.Description, item.RepoURL, item.Homepage, item.Stars).Scan(&id)
		// slug 与 curated 条目冲突时 WHERE 不成立、UPDATE 跳过且不返回行：保留 curated，不计数。
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		count++
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

// ResolveSkillFiles 把 GitHub 上的技能目录解析为 {相对路径: 内容}。
// CLI 永远只连 Loadout 服务，GitHub 限流与令牌由服务端统一承担。
func ResolveSkillFiles(ctx context.Context, o Options, repo, skillPath string) (map[string]string, error) {
	if !repoPattern.MatchString(repo) || skillPath == "" || strings.Contains(skillPath, "..") || strings.HasPrefix(skillPath, "/") {
		return nil, errors.New("invalid skill location")
	}
	files := map[string]string{}
	if err := resolveContents(ctx, o, repo, skillPath, skillPath, files, 0); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("skill directory is empty")
	}
	if _, ok := files["SKILL.md"]; !ok {
		return nil, errors.New("skill directory has no SKILL.md")
	}
	return files, nil
}

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func resolveContents(ctx context.Context, o Options, repo, dir, root string, files map[string]string, depth int) error {
	if depth > 3 || len(files) > 32 {
		return errors.New("skill directory too large")
	}
	base := o.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/repos/"+repo+"/contents/"+strings.TrimPrefix(dir, "/")+"?ref=HEAD", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "loadout-marketplace")
	if o.Token != "" {
		req.Header.Set("Authorization", "Bearer "+o.Token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("github contents returned %d", response.StatusCode)
	}
	var entries []struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		Type     string `json:"type"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err = json.NewDecoder(http.MaxBytesReader(nil, response.Body, 4<<20)).Decode(&entries); err != nil {
		return err
	}
	for _, entry := range entries {
		relative := strings.TrimPrefix(entry.Path, strings.TrimSuffix(root, "/")+"/")
		if relative == "" || relative == entry.Path || strings.Contains(relative, "..") || strings.ContainsAny(relative, "\x00\\") {
			return errors.New("unsafe file path in skill")
		}
		switch entry.Type {
		case "file":
			// GitHub directory listings contain metadata, not file bodies.
			if entry.Encoding == "" {
				fileReq, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, base+"/repos/"+repo+"/contents/"+entry.Path+"?ref=HEAD", nil)
				if requestErr != nil {
					return requestErr
				}
				fileReq.Header = req.Header.Clone()
				fileResponse, requestErr := client.Do(fileReq)
				if requestErr != nil {
					return requestErr
				}
				decodeErr := func() error {
					defer func() { _ = fileResponse.Body.Close() }()
					if fileResponse.StatusCode != http.StatusOK {
						return fmt.Errorf("github contents returned %d", fileResponse.StatusCode)
					}
					return json.NewDecoder(http.MaxBytesReader(nil, fileResponse.Body, 512*1024)).Decode(&entry)
				}()
				if decodeErr != nil {
					return decodeErr
				}
			}
			if entry.Encoding != "base64" || len(files) >= 32 {
				return errors.New("unsupported or oversized skill file")
			}
			raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(entry.Content, "\n", ""))
			if err != nil || len(raw) > 256*1024 {
				return errors.New("unsupported or oversized skill file")
			}
			files[relative] = string(raw)
		case "dir":
			if err := resolveContents(ctx, o, repo, entry.Path, root, files, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
