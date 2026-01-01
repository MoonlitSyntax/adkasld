package render

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"mygo/internal/domain/content"
	"os"
	"path/filepath"
	"time"
)

type TemplateRenderer struct {
	tpl *template.Template
}

func NewTemplateRenderer(themeDir, themeName string) (*TemplateRenderer, error) {
	pattern := filepath.Join(themeDir, themeName, "templates", "*tmpl")
	tpl, err := template.New("").Funcs(templateFuncs()).ParseGlob(pattern)
	if err != nil {
		return nil, err
	}
	return &TemplateRenderer{tpl: tpl}, nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"date": func(t interface{}, layout string) string {
			switch v := t.(type) {
			case nil:
				return ""
			case string:
				return v
			case interface{ Format(string) string }:
				return v.Format(layout)
			default:
				return ""
			}
		},
		"nowYear": func() int {
			return time.Now().Year()
		},
		"postURL": func(m content.ArticleMeta) string {
			d := m.Date
			return fmt.Sprintf("/post/%04d/%02d/%02d/%s/",
				d.Year(), int(d.Month()), d.Day(), m.Slug,
			)
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}
}

func (r *TemplateRenderer) RenderHome(ctx context.Context, page HomePage) ([]byte, error) {
	return r.exec("home.tmpl", page)
}

func (r *TemplateRenderer) RenderPost(ctx context.Context, page PostPage) ([]byte, error) {
	return r.exec("post.tmpl", page)
}

func (r *TemplateRenderer) RenderSeries(ctx context.Context, page SeriesPage) ([]byte, error) {
	return r.exec("series.tmpl", page)
}

func (r *TemplateRenderer) RenderList(ctx context.Context, page ListPage) ([]byte, error) {
	return r.exec("list.tmpl", page)
}

func (r *TemplateRenderer) RenderNotFound(ctx context.Context, page NotFoundPage) ([]byte, error) {
	return r.exec("404.tmpl", page)
}
func (r *TemplateRenderer) RenderArchives(ctx context.Context, page ArchivesPage) ([]byte, error) {
	return r.exec("archives.tmpl", page)
}

func (r *TemplateRenderer) RenderTagsPage(ctx context.Context, page TagsPage) ([]byte, error) {
	return r.exec("tags-all.tmpl", page)
}

func (r *TemplateRenderer) RenderCategoriesPage(ctx context.Context, page CategoriesPage) ([]byte, error) {
	return r.exec("categories-all.tmpl", page)
}

func (r *TemplateRenderer) exec(name string, data interface{}) ([]byte, error) {
	t := r.tpl.Lookup(name)
	if t == nil {
		return nil, fmt.Errorf("template %s not found", name)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func CheckThemeTemplates(themeDir string) error {
	required := []string{
		"home.tmpl",
		"post.tmpl",
		"series.tmpl",
		"list.tmpl",
		"404.tmpl",
		"archives.tmpl",
		"tags-all.tmpl",
		"categories-all.tmpl",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(themeDir, name)); err != nil {
			return fmt.Errorf("missing template: %s", name)
		}
	}
	return nil
}
