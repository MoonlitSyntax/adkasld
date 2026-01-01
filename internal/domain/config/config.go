package config

import (
	"gopkg.in/yaml.v3"
	domainerr "mygo/internal/domain/errors"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	Site   SiteConfig   `yaml:"site"`
	Build  BuildConfig  `yaml:"build"`
	Assets AssetsConfig `yaml:"assets"`
}

type SiteConfig struct {
	Title       string   `yaml:"title"`
	Subtitle    string   `yaml:"subtitle"`
	Author      string   `yaml:"author"`
	SiteURL     string   `yaml:"site_url"`
	Theme       string   `yaml:"themes"`
	SortMode    SortMode `yaml:"sort_mode"`
	TimeZone    string   `yaml:"time_zone"`
	Language    string   `yaml:"language"`
	Description string   `yaml:"description"`
}

type SortMode string

const (
	SortUpdated SortMode = "updated"
	SortCreated SortMode = "created"
)

type BuildConfig struct {
	SourceDir    string    `yaml:"source_dir"`
	PublicDir    string    `yaml:"public_dir"`
	ThemeDir     string    `yaml:"theme_dir"`
	BasePath     string    `yaml:"base_path"`
	IncludeDraft bool      `yaml:"include_draft"`
	Now          time.Time `yaml:"-"`
}

type AssetsConfig struct {
}

func Default() Config {
	return Config{
		Site: SiteConfig{
			Title:    "MyGo",
			Theme:    "default",
			SortMode: SortUpdated,
			Language: "zh-CN",
		},
		Build: BuildConfig{
			SourceDir:    "source",
			PublicDir:    "public",
			ThemeDir:     "themes",
			BasePath:     "",
			IncludeDraft: false,
			Now:          time.Now(),
		},
	}
}

func (c Config) Validate() error {
	var ve domainerr.ValidationError

	if strings.TrimSpace(c.Site.Title) == "" {
		ve.Add("site.title", "must not be empty")
	}

	if strings.TrimSpace(c.Site.SiteURL) == "" {
		ve.Add("site.site_url", "must not be empty")
	} else if !isValidAbsURL(c.Site.SiteURL) {
		ve.Add("site.site_url", "must be a valid absolute URL")
	}

	switch c.Site.SortMode {
	case "", SortUpdated:
	// default ok
	case SortCreated:
	default:
		ve.Add("site.sort_mode", "must be 'updated' or 'created'")
	}

	if strings.TrimSpace(c.Site.Theme) == "" {
		ve.Add("site.themes", "must not be empty")
	}

	if strings.TrimSpace(c.Build.SourceDir) == "" {
		ve.Add("build.source_dir", "must not be empty")
	}
	if strings.TrimSpace(c.Build.PublicDir) == "" {
		ve.Add("build.public_dir", "must not be empty")
	}
	if strings.TrimSpace(c.Build.ThemeDir) == "" {
		ve.Add("build.theme_dir", "must not be empty")
	}
	if bp := strings.TrimSpace(c.Build.BasePath); bp != "" {
		if !strings.HasPrefix(bp, "/") {
			ve.Add("build.base_path", "must start with '/'")
		}
		if strings.HasSuffix(bp, "/") && bp != "/" {
			ve.Add("build.base_path", "must not end with '/'")
		}
	}

	if ve.HasAny() {
		return ve
	}
	return nil
}

func isValidAbsURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	// 直接 Unmarshal 到 cfg 上：文件中写到的字段覆盖默认值，其他字段保留 Default
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	// 没指定 Now 的话用当前时间
	if cfg.Build.Now.IsZero() {
		cfg.Build.Now = time.Now()
	}

	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func LoadOrDefault(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在：用默认配置，但要注意 Validate 可能会因为 SiteURL 之类失败
			if err := cfg.Validate(); err != nil {
				return cfg, err
			}
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Build.Now.IsZero() {
		cfg.Build.Now = time.Now()
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}
