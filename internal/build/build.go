package build

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"mygo/internal/domain/config"
	"mygo/internal/domain/content"
	"mygo/internal/index"
	"mygo/internal/ingest"
	"mygo/internal/render"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Builder struct {
	Cfg       config.Config
	IndexPath string
}

type Result struct {
	Articles int
	Warnings []ingest.Warning
}

func (b *Builder) Run(ctx context.Context) (*Result, error) {
	arts, warns, err := ingest.Ingest(b.Cfg.Build.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("ingest failed: %w", err)
	}

	st, err := index.Open(index.OpenOptions{Path: b.IndexPath})
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}
	defer st.Close()

	if err := st.Rebuild(arts, index.RebuildOptions{
		IncludeDraft: b.Cfg.Build.IncludeDraft,
	}); err != nil {
		return nil, fmt.Errorf("failed to rebuild index: %w", err)
	}

	md := render.NewMarkdownRenderer()
	themeDir := b.Cfg.Build.ThemeDir
	themeName := b.Cfg.Site.Theme
	tpl, err := render.NewTemplateRenderer(themeDir, themeName)

	if err != nil {
		return nil, fmt.Errorf("load themes(%s): %w", themeDir, err)
	}

	outDir := b.Cfg.Build.PublicDir
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir public: %w", err)
	}

	if err := b.buildAll(ctx, st, md, tpl, outDir, arts); err != nil {
		return nil, err
	}

	return &Result{
		Articles: len(arts),
		Warnings: warns,
	}, nil
}

func (b *Builder) buildAll(
	ctx context.Context,
	st *index.Store,
	md *render.MarkdownRenderer,
	tpl render.Renderer,
	outDir string,
	arts []content.Article,
) error {
	if err := b.buildHome(ctx, st, tpl, outDir); err != nil {
		return fmt.Errorf("build home: %w", err)
	}

	if err := b.buildPosts(ctx, st, md, tpl, outDir, arts); err != nil {
		return fmt.Errorf("build posts: %w", err)
	}

	if err := b.buildAllSeries(ctx, st, tpl, outDir); err != nil {
		return fmt.Errorf("build series: %w", err)
	}

	if err := b.buildAllTags(ctx, st, tpl, outDir); err != nil {
		return fmt.Errorf("build tags: %w", err)
	}

	if err := b.buildAllCategories(ctx, st, tpl, outDir); err != nil {
		return fmt.Errorf("build categories: %w", err)
	}

	if err := b.buildNotFound(ctx, tpl, outDir); err != nil {
		return fmt.Errorf("build 404: %w", err)
	}

	if err := b.buildArchives(ctx, st, tpl, outDir); err != nil {
		return fmt.Errorf("build archives: %w", err)
	}

	if err := b.buildTagsOverview(ctx, st, tpl, outDir); err != nil {
		return fmt.Errorf("build tags overview: %w", err)
	}

	if err := b.buildCategoriesOverview(ctx, st, tpl, outDir); err != nil {
		return fmt.Errorf("build categories overview: %w", err)
	}

	if err := b.copyStaticAssets(outDir); err != nil {
		return fmt.Errorf("copy static assets: %w", err)
	}
	return nil
}

func (b *Builder) buildHome(
	ctx context.Context,
	st *index.Store,
	tpl render.Renderer,
	outDir string,
) error {
	opt := index.ListOptions{
		Sort:         b.Cfg.Site.SortMode,
		Page:         1,
		Size:         20,
		IncludeDraft: false,
	}
	items, err := st.HomeItems(opt)
	if err != nil {
		return err
	}

	var viewItems []render.HomeItem
	for _, it := range items {
		switch it.Kind {
		case index.HomePost:
			viewItems = append(viewItems, render.HomeItem{
				Kind: render.HomeItemPost,
				Post: &render.HomePostItem{
					Meta: *it.Meta,
				},
			})
		case index.HomeSeries:
			var rep content.ArticleMeta
			if it.Series.RepresentativeSlug != "" {
				if m, err := st.GetMeta(it.Series.RepresentativeSlug); err == nil {
					rep = m
				}
			}
			viewItems = append(viewItems, render.HomeItem{
				Kind: render.HomeItemSeries,
				Series: &render.HomeSeriesItem{
					Name:               it.Series.Name,
					Count:              it.Series.Count,
					LatestUpdated:      it.Series.LatestUpdated,
					MaxSticky:          it.Series.MaxSticky,
					RepresentativePost: rep,
				},
			})
		}
	}

	page := render.HomePage{
		Site:      b.Cfg.Site,
		Items:     viewItems,
		Page:      1,
		PageSize:  opt.Size,
		Generated: b.Cfg.Build.Now,
		PageTitle: "Home",
	}
	htmlBytes, err := tpl.RenderHome(ctx, page)
	if err != nil {
		return err
	}

	return writeFile(outDir, "index.html", htmlBytes)
}

func (b *Builder) buildPosts(
	ctx context.Context,
	st *index.Store,
	md *render.MarkdownRenderer,
	tpl render.Renderer,
	outDir string,
	arts []content.Article,
) error {
	for _, a := range arts {
		meta := a.Meta

		// build 阶段：hidden 一律不输出，draft 默认也不输出
		if meta.Hidden {
			continue
		}
		if meta.Draft && !b.Cfg.Build.IncludeDraft {
			continue
		}

		// 读取 markdown 原文
		src, err := os.ReadFile(a.Body.SourcePath)
		if err != nil {
			return fmt.Errorf("read post source(%s): %w", a.Body.SourcePath, err)
		}

		// 去掉 frontmatter，只保留正文
		fm, body, fmErr := ingest.ParseFrontMatter(src)
		if fmErr == nil && fm.Title != "" {
			_ = fm // 这里只是利用 ParseFrontMatter 切掉头部
		} else {
			body = src
		}

		// markdown -> HTML
		mdResult, err := md.Render(body)
		if err != nil {
			return fmt.Errorf("markdown render(%s): %w", meta.Slug, err)
		}

		// 系列信息：用于详情页 sidebar 展开
		var seriesList []content.ArticleMeta
		if meta.Series.Name != "" {
			seriesList, _ = st.ListSeries(meta.Series.Name, index.ListOptions{
				Sort:         b.Cfg.Site.SortMode,
				Page:         1,
				Size:         1000,
				IncludeDraft: false,
			})
		}

		pp := render.PostPage{
			Site:      b.Cfg.Site,
			Meta:      meta,
			HTML:      template.HTML(mdResult.HTML),
			TOC:       mdResult.Headings,
			IsDraft:   meta.Draft,
			PageTitle: meta.Title,
		}
		pp.SeriesName = meta.Series.Name
		pp.SeriesList = seriesList
		// TODO: Related 相关文章后面单独做，这里先留空切片

		htmlBytes, err := tpl.RenderPost(ctx, pp)
		if err != nil {
			return fmt.Errorf("render post(%s): %w", meta.Slug, err)
		}

		// 路径：/post/YYYY/MM/DD/slug/index.html
		y, mo, d := meta.Date.Date()
		outPath := filepath.Join(
			"post",
			fmt.Sprintf("%04d", y),
			fmt.Sprintf("%02d", mo),
			fmt.Sprintf("%02d", d),
			meta.Slug,
			"index.html",
		)
		if err := writeFile(outDir, outPath, htmlBytes); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) buildAllSeries(
	ctx context.Context,
	st *index.Store,
	tpl render.Renderer,
	outDir string,
) error {
	names, err := st.ListAllSeriesNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		// 系列文章列表
		items, err := st.ListSeries(name, index.ListOptions{
			Sort:         b.Cfg.Site.SortMode,
			Page:         1,
			Size:         1000,
			IncludeDraft: false,
		})
		if err != nil {
			return err
		}
		if len(items) == 0 {
			continue
		}
		sum, err := st.GetSeriesSummary(name, false)
		if err != nil {
			continue
		}

		sp := render.SeriesPage{
			Site:   b.Cfg.Site,
			Name:   name,
			Items:  items,
			Count:  sum.Count,
			Latest: sum.LatestUpdated,
		}

		htmlBytes, err := tpl.RenderSeries(ctx, sp)
		if err != nil {
			return fmt.Errorf("render series(%s): %w", name, err)
		}

		outPath := filepath.Join("series", safePathSegment(name), "index.html")
		if err := writeFile(outDir, outPath, htmlBytes); err != nil {
			return err
		}
	}
	return nil
}

// =============== tags /tags/<tag>/index.html ===============

func (b *Builder) buildAllTags(
	ctx context.Context,
	st *index.Store,
	tpl render.Renderer,
	outDir string,
) error {
	// 简单做法：从全站 meta 里收集 tags
	metas, err := st.List(index.ListOptions{
		Sort:         b.Cfg.Site.SortMode,
		Page:         1,
		Size:         1000000,
		IncludeDraft: false,
	})
	if err != nil {
		return err
	}

	tagSet := make(map[string]struct{})
	for _, m := range metas {
		for _, t := range m.Tags {
			if t == "" {
				continue
			}
			tagSet[t] = struct{}{}
		}
	}

	for tag := range tagSet {
		items, err := st.ListByTag(tag, index.ListOptions{
			Sort:         b.Cfg.Site.SortMode,
			Page:         1,
			Size:         1000,
			IncludeDraft: false,
		})
		if err != nil {
			return err
		}
		if len(items) == 0 {
			continue
		}

		lp := render.ListPage{
			Site:      b.Cfg.Site,
			Title:     fmt.Sprintf("Tag: %s", tag),
			SubTitle:  "",
			Items:     items,
			Page:      1,
			PageSize:  len(items),
			Tag:       tag,
			Generated: b.Cfg.Build.Now,
		}

		htmlBytes, err := tpl.RenderList(ctx, lp)
		if err != nil {
			return fmt.Errorf("render tag(%s): %w", tag, err)
		}

		outPath := filepath.Join("tags", safePathSegment(tag), "index.html")
		if err := writeFile(outDir, outPath, htmlBytes); err != nil {
			return err
		}
	}
	return nil
}

// =============== categories /categories/<cat>/index.html ===============

func (b *Builder) buildAllCategories(
	ctx context.Context,
	st *index.Store,
	tpl render.Renderer,
	outDir string,
) error {
	metas, err := st.List(index.ListOptions{
		Sort:         b.Cfg.Site.SortMode,
		Page:         1,
		Size:         1000000,
		IncludeDraft: false,
	})
	if err != nil {
		return err
	}

	catSet := make(map[string]struct{})
	for _, m := range metas {
		if c := strings.TrimSpace(m.Category); c != "" {
			catSet[c] = struct{}{}
		}
	}

	for cat := range catSet {
		items, err := st.ListByCategory(cat, index.ListOptions{
			Sort:         b.Cfg.Site.SortMode,
			Page:         1,
			Size:         1000,
			IncludeDraft: false,
		})
		if err != nil {
			return err
		}
		if len(items) == 0 {
			continue
		}

		lp := render.ListPage{
			Site:      b.Cfg.Site,
			Title:     fmt.Sprintf("Category: %s", cat),
			Items:     items,
			Page:      1,
			PageSize:  len(items),
			Category:  cat,
			Generated: b.Cfg.Build.Now,
		}

		htmlBytes, err := tpl.RenderList(ctx, lp)
		if err != nil {
			return fmt.Errorf("render category(%s): %w", cat, err)
		}

		outPath := filepath.Join("categories", safePathSegment(cat), "index.html")
		if err := writeFile(outDir, outPath, htmlBytes); err != nil {
			return err
		}
	}
	return nil
}

// =============== 404 /404.html ===============

func (b *Builder) buildNotFound(
	ctx context.Context,
	tpl render.Renderer,
	outDir string,
) error {
	page := render.NotFoundPage{
		Site: b.Cfg.Site,
		Path: "",
	}
	htmlBytes, err := tpl.RenderNotFound(ctx, page)
	if err != nil {
		return err
	}
	return writeFile(outDir, "404.html", htmlBytes)
}

func writeFile(root, rel string, data []byte) error {
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

func safePathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "untitled"
	}
	repl := func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}
	return strings.Map(repl, s)
}

func (b *Builder) buildArchives(
	ctx context.Context,
	st *index.Store,
	tpl render.Renderer,
	outDir string,
) error {
	metas, err := st.List(index.ListOptions{
		Sort:         b.Cfg.Site.SortMode,
		Page:         1,
		Size:         1000000,
		IncludeDraft: false,
	})
	if err != nil {
		return err
	}

	groupsMap := make(map[int][]content.ArticleMeta)
	for _, m := range metas {
		y := m.Date.Year()
		groupsMap[y] = append(groupsMap[y], m)
	}

	years := make([]int, 0, len(groupsMap))
	for y := range groupsMap {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	groups := make([]render.ArchivesGroup, 0, len(years))
	total := len(metas)
	for _, y := range years {
		posts := groupsMap[y]
		groups = append(groups, render.ArchivesGroup{
			Year:  y,
			Posts: posts,
			Count: len(posts),
		})
	}

	page := render.ArchivesPage{
		Site:   b.Cfg.Site,
		Groups: groups,
		Total:  total,
	}

	htmlBytes, err := tpl.RenderArchives(ctx, page)
	if err != nil {
		return err
	}
	return writeFile(outDir, filepath.Join("archives", "index.html"), htmlBytes)
}

func (b *Builder) buildTagsOverview(
	ctx context.Context,
	st *index.Store,
	tpl render.Renderer,
	outDir string,
) error {
	metas, err := st.List(index.ListOptions{
		Sort:         b.Cfg.Site.SortMode,
		Page:         1,
		Size:         1000000,
		IncludeDraft: false,
	})
	if err != nil {
		return err
	}

	counts := make(map[string]int)
	for _, m := range metas {
		for _, t := range m.Tags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			counts[t]++
		}
	}

	stats := make([]render.TagStat, 0, len(counts))
	for name, c := range counts {
		stats = append(stats, render.TagStat{
			Name:  name,
			Count: c,
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		// 按数量降序，数量相同按名字排序
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})

	page := render.TagsPage{
		Site:  b.Cfg.Site,
		Tags:  stats,
		Total: len(stats),
	}
	htmlBytes, err := tpl.RenderTagsPage(ctx, page)
	if err != nil {
		return err
	}
	return writeFile(outDir, filepath.Join("tags", "index.html"), htmlBytes)
}

func (b *Builder) buildCategoriesOverview(
	ctx context.Context,
	st *index.Store,
	tpl render.Renderer,
	outDir string,
) error {
	metas, err := st.List(index.ListOptions{
		Sort:         b.Cfg.Site.SortMode,
		Page:         1,
		Size:         1000000,
		IncludeDraft: false,
	})
	if err != nil {
		return err
	}

	counts := make(map[string]int)
	for _, m := range metas {
		c := strings.TrimSpace(m.Category)
		if c == "" {
			continue
		}
		counts[c]++
	}

	stats := make([]render.CategoryStat, 0, len(counts))
	for name, c := range counts {
		stats = append(stats, render.CategoryStat{
			Name:  name,
			Count: c,
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})

	page := render.CategoriesPage{
		Site:       b.Cfg.Site,
		Categories: stats,
		Total:      len(stats),
	}
	htmlBytes, err := tpl.RenderCategoriesPage(ctx, page)
	if err != nil {
		return err
	}
	return writeFile(outDir, filepath.Join("categories", "index.html"), htmlBytes)
}

func (b *Builder) copyStaticAssets(outDir string) error {
	src := filepath.Join(b.Cfg.Build.ThemeDir, b.Cfg.Site.Theme, "static")
	// 如果没有 static 目录就算了
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(outDir, rel)

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}

		in, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, in, 0o644)
	})
}
