package content

import (
	"strings"
	"time"
)

type Series struct {
	Name  string
	Order int
}

type ArticleMeta struct {
	Title   string
	Slug    string
	Date    time.Time
	Updated time.Time

	Tags     []string
	Category string

	Series      Series
	Description string
	Summary     string
	Cover       string

	Sticky int
	Hidden bool
	Draft  bool

	Aliases []string

	// 由解析阶段填充（但仍属于 domain 数据）
	WordCount int
	ReadMin   int

	// 结构化信息：渲染 / TOC / 相关文章 / 知识图谱 会用
	Headings []Heading
	OutLinks []string // slugs or URLs
	ShortID  string
}

type Heading struct {
	Level int
	ID    string
	Text  string
}

type BodyRef struct {
	SourcePath  string
	ContentHash string
}

type Article struct {
	Meta ArticleMeta
	Body BodyRef
}

func (m *ArticleMeta) Normalize() {
	m.Title = strings.TrimSpace(m.Title)
	m.Slug = strings.TrimSpace(m.Slug)
	m.Category = strings.TrimSpace(m.Category)

	m.Tags = normalizeStrings(m.Tags)
	m.Aliases = normalizeStrings(m.Aliases)
	m.Series.Name = strings.TrimSpace(m.Series.Name)
	if m.Series.Order < 0 {
		m.Series.Order = 0
	}
}

func normalizeStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		item = strings.ToLower(item)
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
