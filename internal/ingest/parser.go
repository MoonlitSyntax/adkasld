package ingest

import (
	"bytes"
	"errors"
	"gopkg.in/yaml.v3"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var errNoFrontMatter = errors.New("no front matter found")
var errInvalidFrontMatter = errors.New("invalid front matter")

type FrontMatter struct {
	Title   string `yaml:"title"`
	Slug    string `yaml:"slug"`
	Date    string `yaml:"date"`
	Updated string `yaml:"updated"`

	Tags     []string `yaml:"tags"`
	Category string   `yaml:"category"`

	Sticky int    `yaml:"sticky"`
	Hidden bool   `yaml:"hidden"`
	Draft  bool   `yaml:"draft"`
	Cover  string `yaml:"cover"`

	Aliases []string `yaml:"aliases"`
	Series  struct {
		Name  string `yaml:"name"`
		Order int    `yaml:"order"`
	} `yaml:"series"`

	ShortID string `yaml:"short"`
}

func ParseFrontMatter(raw []byte) (FrontMatter, []byte, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return FrontMatter{}, raw, errNoFrontMatter
	}

	// 统一换行符
	norm := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	norm = bytes.ReplaceAll(norm, []byte("\r"), []byte("\n"))

	const (
		sep      = "---"
		sepLine  = sep + "\n"
		closeMid = "\n" + sep + "\n"
	)

	if !bytes.HasPrefix(norm, []byte(sepLine)) {
		return FrontMatter{}, raw, errNoFrontMatter
	}

	// 去掉首行 "---\n"
	rest := norm[len(sepLine):]

	var yamlPart, bodyPart []byte

	// 优先走最常见的情况：中间有 "\n---\n"
	if parts := bytes.SplitN(rest, []byte(closeMid), 2); len(parts) == 2 {
		yamlPart = parts[0]
		bodyPart = parts[1]
	} else {
		// 可能是结尾是 "\n---" 且无正文
		if bytes.HasSuffix(rest, []byte("\n"+sep)) {
			yamlPart = rest[:len(rest)-len("\n"+sep)]
			bodyPart = nil
		} else if bytes.Equal(bytes.TrimSpace(rest), []byte(sep)) {
			// 处理 "---\n---" 这种“空 front matter，无正文”
			yamlPart = nil
			bodyPart = nil
		} else {
			return FrontMatter{}, raw, errInvalidFrontMatter
		}
	}

	yamlPart = bytes.TrimSpace(yamlPart)
	bodyPart = bytes.TrimSpace(bodyPart)

	var fm FrontMatter
	if len(yamlPart) > 0 {
		if err := yaml.Unmarshal(yamlPart, &fm); err != nil {
			return FrontMatter{}, raw, err
		}
	}
	if fm.Cover == "" {
		fm.Cover = "https://cdn.example.com/default-cover.jpg"
	}

	return fm, bodyPart, nil
}

func ResolveSlug(fm FrontMatter, path string) string {
	if s := strings.TrimSpace(fm.Slug); s != "" {
		return slugify(s)
	}
	if t := strings.TrimSpace(fm.Title); t != "" {
		return slugify(t)
	}
	base := filepath.Base(path)
	return slugify(strings.TrimSuffix(base, filepath.Ext(base)))
}

func ParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339,
		time.DateOnly,
		"2006-01-02 15:04",
		time.DateTime,
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func slugify(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var out []rune
	lastDash := false

	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]

		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if r <= unicode.MaxASCII {
				if 'A' <= r && r <= 'Z' {
					r = r + ('a' - 'A')
				}
			}
			out = append(out, r)
			lastDash = false
		case r == '-' || r == '_' || r == '.' || unicode.IsSpace(r):
			if !lastDash && len(out) > 0 {
				out = append(out, '-')
				lastDash = true
			}

		default:
			if !lastDash && len(out) > 0 {
				out = append(out, '-')
				lastDash = true
			}
		}
	}
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	return string(out)
}
