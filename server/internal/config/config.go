package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/Yanyutin753/loadout/server/internal/filestore"
)

type Config struct {
	FileStorageEndpoint        string
	FileStorageRegion          string
	FileStorageBucket          string
	FileStorageAccessKeyID     string
	FileStorageSecretAccessKey string
	FileStorageSessionToken    string
	FileStoragePrefix          string
	FileStoragePathStyle       bool
	FileStorageAllowHTTP       bool
	FileStorageInlineMaxBytes  int64
	RedisURL                   string
	RedisNamespace             string
	Addr                       string
	WebDir                     string
	DatabaseURL                string
	PublicURL                  string
	EncryptionKey              []byte
	AdminUsername              string
	AdminPassword              string
	AllowPrivateUpstreams      bool
	StdioCommands              map[string]string
	GitHubClientID             string
	GitHubClientSecret         string
	GitHubOrg                  string
	GitHubAPIURL               string
	GitHubToken                string
	RateTokenPerMinute         int64
	RateUserPerDay             int64
	SMTPAddress                string
	SMTPFrom                   string
	SMTPUsername               string
	SMTPPassword               string
	SMTPAllowLocalInsecure     bool
	InitialCredits             int64
}

func (cfg Config) FileStorageOptions() filestore.Options {
	return filestore.Options{
		Endpoint: cfg.FileStorageEndpoint, Region: cfg.FileStorageRegion, Bucket: cfg.FileStorageBucket,
		AccessKeyID: cfg.FileStorageAccessKeyID, SecretAccessKey: cfg.FileStorageSecretAccessKey,
		SessionToken: cfg.FileStorageSessionToken, Prefix: cfg.FileStoragePrefix,
		PathStyle: cfg.FileStoragePathStyle, AllowHTTP: cfg.FileStorageAllowHTTP, InlineMaxBytes: cfg.FileStorageInlineMaxBytes,
	}
}

func Load() (Config, error) {
	cfg := Config{Addr: os.Getenv("LOADOUT_ADDR"), WebDir: os.Getenv("LOADOUT_WEB_DIR")}
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8787"
	}
	if cfg.WebDir == "" {
		cfg.WebDir = "web/dist"
	}
	_, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return Config{}, fmt.Errorf("LOADOUT_ADDR must be host:port")
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 0 || value > 65535 {
		return Config{}, fmt.Errorf("LOADOUT_ADDR port must be between 0 and 65535")
	}
	cfg.InitialCredits = 1000
	if raw := os.Getenv("LOADOUT_INITIAL_CREDITS"); raw != "" {
		cfg.InitialCredits, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || cfg.InitialCredits < 0 || cfg.InitialCredits > 1_000_000_000_000 {
			return Config{}, fmt.Errorf("LOADOUT_INITIAL_CREDITS must be an integer between 0 and 1000000000000")
		}
	}
	cfg.RedisURL = os.Getenv("LOADOUT_REDIS_URL")
	cfg.RedisNamespace = os.Getenv("LOADOUT_REDIS_NAMESPACE")
	if cfg.RedisNamespace == "" {
		cfg.RedisNamespace = "loadout"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(cfg.RedisNamespace) {
		return Config{}, fmt.Errorf("LOADOUT_REDIS_NAMESPACE must be 1-64 letters, digits, underscores or hyphens")
	}
	if cfg.RedisURL != "" {
		u, e := url.Parse(cfg.RedisURL)
		if e != nil || (u.Scheme != "redis" && u.Scheme != "rediss") || u.Host == "" {
			return Config{}, fmt.Errorf("LOADOUT_REDIS_URL must be a Redis URL")
		}
	}
	cfg.DatabaseURL = os.Getenv("LOADOUT_DATABASE_URL")
	if cfg.DatabaseURL != "" {
		u, e := url.Parse(cfg.DatabaseURL)
		if e != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
			return Config{}, fmt.Errorf("LOADOUT_DATABASE_URL must be a PostgreSQL URL")
		}
	}
	cfg.PublicURL = os.Getenv("LOADOUT_PUBLIC_URL")
	if cfg.PublicURL != "" {
		u, e := url.Parse(cfg.PublicURL)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return Config{}, fmt.Errorf("LOADOUT_PUBLIC_URL must be an HTTP(S) origin")
		}
		cfg.PublicURL = u.Scheme + "://" + u.Host
	}
	if key := os.Getenv("LOADOUT_ENCRYPTION_KEY"); key != "" {
		cfg.EncryptionKey, err = base64.StdEncoding.DecodeString(key)
		if err != nil || len(cfg.EncryptionKey) != 32 {
			return Config{}, fmt.Errorf("LOADOUT_ENCRYPTION_KEY must encode 32 bytes as base64")
		}
	}
	cfg.AdminUsername = os.Getenv("LOADOUT_ADMIN_USERNAME")
	cfg.AdminPassword = os.Getenv("LOADOUT_ADMIN_PASSWORD")
	if (cfg.AdminUsername == "") != (cfg.AdminPassword == "") {
		return Config{}, fmt.Errorf("admin bootstrap requires both username and password")
	}
	if raw := os.Getenv("LOADOUT_ALLOW_PRIVATE_UPSTREAMS"); raw != "" {
		cfg.AllowPrivateUpstreams, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("LOADOUT_ALLOW_PRIVATE_UPSTREAMS must be boolean")
		}
	}
	if raw := os.Getenv("LOADOUT_STDIO_COMMANDS"); raw != "" {
		if err = json.Unmarshal([]byte(raw), &cfg.StdioCommands); err != nil {
			return Config{}, fmt.Errorf("LOADOUT_STDIO_COMMANDS must be a name/path JSON object")
		}
		for name, p := range cfg.StdioCommands {
			if name == "" || !filepath.IsAbs(p) {
				return Config{}, fmt.Errorf("stdio commands require a name and absolute executable path")
			}
		}
	}
	cfg.GitHubClientID = os.Getenv("LOADOUT_GITHUB_CLIENT_ID")
	cfg.GitHubClientSecret = os.Getenv("LOADOUT_GITHUB_CLIENT_SECRET")
	cfg.GitHubOrg = os.Getenv("LOADOUT_GITHUB_ORG")
	cfg.GitHubAPIURL = os.Getenv("LOADOUT_GITHUB_API")
	cfg.GitHubToken = os.Getenv("LOADOUT_GITHUB_TOKEN")
	cfg.RateTokenPerMinute, _ = strconv.ParseInt(os.Getenv("LOADOUT_RATE_TOKEN_PER_MINUTE"), 10, 64)
	cfg.RateUserPerDay, _ = strconv.ParseInt(os.Getenv("LOADOUT_RATE_USER_PER_DAY"), 10, 64)
	if (cfg.GitHubClientID == "") != (cfg.GitHubClientSecret == "") || (cfg.GitHubClientID != "" && cfg.PublicURL == "") {
		return Config{}, fmt.Errorf("GitHub login requires client id, secret, and public URL")
	}
	if cfg.GitHubOrg != "" && !regexp.MustCompile(`^[A-Za-z0-9-]{1,39}$`).MatchString(cfg.GitHubOrg) {
		return Config{}, fmt.Errorf("invalid GitHub organization")
	}
	cfg.SMTPAddress = os.Getenv("LOADOUT_SMTP_ADDRESS")
	cfg.SMTPFrom = os.Getenv("LOADOUT_SMTP_FROM")
	cfg.SMTPUsername = os.Getenv("LOADOUT_SMTP_USERNAME")
	cfg.SMTPPassword = os.Getenv("LOADOUT_SMTP_PASSWORD")
	if cfg.SMTPAddress != "" || cfg.SMTPFrom != "" {
		_, _, addressErr := net.SplitHostPort(cfg.SMTPAddress)
		_, fromErr := mail.ParseAddress(cfg.SMTPFrom)
		if addressErr != nil || fromErr != nil || cfg.PublicURL == "" {
			return Config{}, fmt.Errorf("SMTP requires host:port, sender, and public URL")
		}
	}
	if (cfg.SMTPUsername == "") != (cfg.SMTPPassword == "") {
		return Config{}, fmt.Errorf("SMTP auth requires username and password")
	}
	if raw := os.Getenv("LOADOUT_SMTP_ALLOW_LOCAL_INSECURE"); raw != "" {
		cfg.SMTPAllowLocalInsecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("LOADOUT_SMTP_ALLOW_LOCAL_INSECURE must be boolean")
		}
	}

	cfg.FileStorageEndpoint = os.Getenv("LOADOUT_FILE_STORAGE_ENDPOINT")
	cfg.FileStorageRegion = os.Getenv("LOADOUT_FILE_STORAGE_REGION")
	cfg.FileStorageBucket = os.Getenv("LOADOUT_FILE_STORAGE_BUCKET")
	cfg.FileStorageAccessKeyID = os.Getenv("LOADOUT_FILE_STORAGE_ACCESS_KEY_ID")
	cfg.FileStorageSecretAccessKey = os.Getenv("LOADOUT_FILE_STORAGE_SECRET_ACCESS_KEY")
	cfg.FileStorageSessionToken = os.Getenv("LOADOUT_FILE_STORAGE_SESSION_TOKEN")
	cfg.FileStoragePrefix = os.Getenv("LOADOUT_FILE_STORAGE_PREFIX")
	cfg.FileStorageInlineMaxBytes = 256 << 10
	if raw := os.Getenv("LOADOUT_FILE_STORAGE_INLINE_MAX_BYTES"); raw != "" {
		cfg.FileStorageInlineMaxBytes, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || cfg.FileStorageInlineMaxBytes < 1 || cfg.FileStorageInlineMaxBytes > 16<<20 {
			return Config{}, fmt.Errorf("LOADOUT_FILE_STORAGE_INLINE_MAX_BYTES must be between 1 and 16777216")
		}
	}
	for key, target := range map[string]*bool{"LOADOUT_FILE_STORAGE_PATH_STYLE": &cfg.FileStoragePathStyle, "LOADOUT_FILE_STORAGE_ALLOW_HTTP": &cfg.FileStorageAllowHTTP} {
		if raw := os.Getenv(key); raw != "" {
			*target, err = strconv.ParseBool(raw)
			if err != nil {
				return Config{}, fmt.Errorf("%s must be boolean", key)
			}
		}
	}
	return cfg, nil
}
