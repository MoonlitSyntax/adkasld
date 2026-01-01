package serve

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"html/template"
	"log"
	"mygo/internal/domain/config"
	"mygo/internal/domain/content"
	"mygo/internal/index"
	"mygo/internal/ingest"
	"mygo/internal/render"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Server struct {
	cfg config.Config

	indexPath string
	idx       *index.Store
	md        *render.MarkdownRenderer
	tpl       render.Renderer

	mu       sync.RWMutex
	articles map[string]content.Article

	sseMu     sync.Mutex
	sseConns  map[chan string]struct{}
	watcher   *fsnotify.Watcher
	watchOnce sync.Once
}

func New(cfg config.Config, indexPath string, themeDir, themeName string) (*Server, error) {
	md := render.NewMarkdownRenderer()
	tpl, err := render.NewTemplateRenderer(themeDir, themeName)
	if err != nil {
		return nil, fmt.Errorf("serve: failed to create template renderer: %w", err)
	}
	st, err := index.Open(index.OpenOptions{Path: indexPath})
	if err != nil {
		return nil, fmt.Errorf("serve: failed to open index: %w", err)
	}

	s := &Server{
		cfg:       cfg,
		indexPath: indexPath,
		idx:       st,
		md:        md,
		tpl:       tpl,
		articles:  make(map[string]content.Article),
		sseConns:  make(map[chan string]struct{}),
	}
	return s, nil
}

func (s *Server) Close() error {
	if s.watcher != nil {
		_ = s.watcher.Close()
	}
	if s.idx != nil {
		return s.idx.Close()
	}
	return nil
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	if err := s.rebuild(ctx); err != nil {
		return err
	}

	// 启动文件监控
	if err := s.startWatch(ctx); err != nil {
		return err
	}

	mux := http.NewServeMux()

	// 静态路由
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/post/", s.handlePost)
	mux.HandleFunc("/series/", s.handleSeries)
	mux.HandleFunc("/tags/", s.handleTag)
	mux.HandleFunc("/categories/", s.handleCategory)
	mux.HandleFunc("/archives", s.handleArchives)
	mux.HandleFunc("/tags", s.handleTagsRoot)
	mux.HandleFunc("/categories", s.handleCategoriesRoot)

	mux.HandleFunc("/about", s.handleStaticSlug("about"))
	mux.HandleFunc("/links", s.handleStaticSlug("links"))

	// dev SSE
	mux.HandleFunc("/dev/events", s.handleSSE)

	staticDir := filepath.Join(s.cfg.Build.ThemeDir, s.cfg.Site.Theme, "static")
	fileServer := http.FileServer(http.Dir(staticDir))

	mux.Handle("/css/", fileServer)
	mux.Handle("/js/", fileServer)
	mux.Handle("/images/", fileServer)
	mux.Handle("/favicon.ico", fileServer)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 支持 ctx 取消
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	log.Printf("[serve] listening on %s", addr)
	return srv.ListenAndServe()
}

func (s *Server) rebuild(ctx context.Context) error {
	sourceDir := s.cfg.Build.SourceDir
	log.Printf("[serve] ingest from %s ...", sourceDir)
	arts, warns, err := ingest.Ingest(sourceDir)
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	for _, w := range warns {
		log.Printf("[warn] %s: %s", w.Path, w.Msg)
	}
	log.Printf("[serve] ingested %d articles", len(arts))

	if err := s.idx.Rebuild(arts, index.RebuildOptions{
		IncludeDraft: true,
	}); err != nil {
		return fmt.Errorf("index rebuild: %w", err)
	}

	m := make(map[string]content.Article, len(arts))
	for _, a := range arts {
		if strings.TrimSpace(a.Meta.Slug) == "" {
			continue
		}
		m[a.Meta.Slug] = a
	}
	s.mu.Lock()
	s.articles = m
	s.mu.Unlock()

	log.Printf("[serve] rebuild complete")
	s.broadcastSSE("reload")

	return nil
}

func (s *Server) startWatch(ctx context.Context) error {
	var err error
	s.watchOnce.Do(func() {
		w, e := fsnotify.NewWatcher()
		if e != nil {
			err = e
			return
		}
		s.watcher = w

		go s.watchLoop(ctx)

		err = filepath.Walk(s.cfg.Build.SourceDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return w.Add(path)
			}
			return nil
		})
	})
	return err
}

func (s *Server) watchLoop(ctx context.Context) {
	log.Printf("[serve] watching for file changes ...")
	debounce := time.NewTicker(time.Hour)
	debounce.Stop()

	trigger := func() {
		select {
		case <-debounce.C:
		default:
		}
		debounce.Reset(200 * time.Millisecond)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				trigger()
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[warn] watcher error: %v", err)
		case <-debounce.C:
			ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := s.rebuild(ctx2); err != nil {
				log.Printf("[serve] rebuild error: %v", err)
			}
			cancel()
		}
	}
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 8)

	s.sseMu.Lock()
	s.sseConns[ch] = struct{}{}
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseConns, ch)
		close(ch)
		s.sseMu.Unlock()
	}()
	fmt.Fprintf(w, "data: %s\n\n", "hello")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastSSE(msg string) {
	s.sseMu.Lock()
	defer s.sseMu.Unlock()
	for ch := range s.sseConns {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.handleNotFound(w, r)
		return
	}

	opt := index.ListOptions{
		Sort:         s.cfg.Site.SortMode,
		Page:         1,
		Size:         20,
		IncludeDraft: true,
	}
	items, err := s.idx.HomeItems(opt)
	if err != nil {
		http.Error(w, "home query error", http.StatusInternalServerError)
		return
	}

	var viewItems []render.HomeItem
	for _, it := range items {
		switch it.Kind {
		case index.HomePost:
			viewItems = append(viewItems, render.HomeItem{
				Kind: render.HomeItemPost,
				Post: &render.HomePostItem{Meta: *it.Meta},
			})
		case index.HomeSeries:
			var rep content.ArticleMeta
			if it.Series.RepresentativeSlug != "" {
				if m, err := s.idx.GetMeta(it.Series.RepresentativeSlug); err == nil {
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
		Site:      s.cfg.Site,
		Items:     viewItems,
		Page:      1,
		PageSize:  opt.Size,
		Generated: time.Now(),
		PageTitle: "Home",
	}
	htmlBytes, err := s.tpl.RenderHome(r.Context(), page)
	if err != nil {
		log.Printf("render home error: %v", err)
		http.Error(w, "render home error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, htmlBytes)
}

// 文章详情页：/post/YYYY/MM/DD/slug/ 或 /post/YYYY/MM/DD/slug
func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/post/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		s.handleNotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		s.handleNotFound(w, r)
		return
	}
	slug := parts[len(parts)-1]

	s.mu.RLock()
	art, ok := s.articles[slug]
	s.mu.RUnlock()
	if !ok {
		s.handleNotFound(w, r)
		return
	}
	meta := art.Meta

	src, err := os.ReadFile(art.Body.SourcePath)
	if err != nil {
		log.Printf("read source error: %v", err)
		http.Error(w, "read source error", http.StatusInternalServerError)
		return
	}
	_, body, fmErr := ingest.ParseFrontMatter(src)
	if fmErr != nil {
		body = src
	}

	mdResult, err := s.md.Render(body)
	if err != nil {
		log.Printf("markdown render error: %v", err)
		http.Error(w, "markdown render error", http.StatusInternalServerError)
		return
	}

	var seriesList []content.ArticleMeta
	if meta.Series.Name != "" {
		seriesList, _ = s.idx.ListSeries(meta.Series.Name, index.ListOptions{
			Sort:         s.cfg.Site.SortMode,
			Page:         1,
			Size:         1000,
			IncludeDraft: true,
		})
	}

	pp := render.PostPage{
		Site:       s.cfg.Site,
		Meta:       meta,
		HTML:       template.HTML(mdResult.HTML),
		TOC:        mdResult.Headings,
		IsDraft:    meta.Draft,
		SeriesName: meta.Series.Name,
		SeriesList: seriesList,
		PageTitle:  meta.Title,
	}

	htmlBytes, err := s.tpl.RenderPost(r.Context(), pp)
	if err != nil {
		log.Printf("render post error: %v", err)
		http.Error(w, "render post error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, htmlBytes)
}

// 系列页：/series/<name>/
func (s *Server) handleSeries(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/series/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		s.handleNotFound(w, r)
		return
	}
	name := path

	items, err := s.idx.ListSeries(name, index.ListOptions{
		Sort:         s.cfg.Site.SortMode,
		Page:         1,
		Size:         1000,
		IncludeDraft: true,
	})
	if err != nil || len(items) == 0 {
		s.handleNotFound(w, r)
		return
	}
	sum, err := s.idx.GetSeriesSummary(name, true)
	if err != nil {
		s.handleNotFound(w, r)
		return
	}

	sp := render.SeriesPage{
		Site:   s.cfg.Site,
		Name:   name,
		Items:  items,
		Count:  sum.Count,
		Latest: sum.LatestUpdated,
	}
	htmlBytes, err := s.tpl.RenderSeries(r.Context(), sp)
	if err != nil {
		log.Printf("render series error: %v", err)
		http.Error(w, "render series error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, htmlBytes)
}

// 标签页：/tags/<tag>/
func (s *Server) handleTag(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tags/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		s.handleNotFound(w, r)
		return
	}
	tag := path

	items, err := s.idx.ListByTag(tag, index.ListOptions{
		Sort:         s.cfg.Site.SortMode,
		Page:         1,
		Size:         1000,
		IncludeDraft: true,
	})
	if err != nil || len(items) == 0 {
		s.handleNotFound(w, r)
		return
	}

	lp := render.ListPage{
		Site:      s.cfg.Site,
		Title:     fmt.Sprintf("Tag: %s", tag),
		Items:     items,
		Page:      1,
		PageSize:  len(items),
		Tag:       tag,
		Generated: time.Now(),
	}
	htmlBytes, err := s.tpl.RenderList(r.Context(), lp)
	if err != nil {
		log.Printf("render tag error: %v", err)
		http.Error(w, "render tag error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, htmlBytes)
}

// 分类页：/categories/<cat>/
func (s *Server) handleCategory(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/categories/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		s.handleNotFound(w, r)
		return
	}
	cat := path

	items, err := s.idx.ListByCategory(cat, index.ListOptions{
		Sort:         s.cfg.Site.SortMode,
		Page:         1,
		Size:         1000,
		IncludeDraft: true,
	})
	if err != nil || len(items) == 0 {
		s.handleNotFound(w, r)
		return
	}

	lp := render.ListPage{
		Site:      s.cfg.Site,
		Title:     fmt.Sprintf("Category: %s", cat),
		Items:     items,
		Page:      1,
		PageSize:  len(items),
		Category:  cat,
		Generated: time.Now(),
	}
	htmlBytes, err := s.tpl.RenderList(r.Context(), lp)
	if err != nil {
		log.Printf("render category error: %v", err)
		http.Error(w, "render category error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, htmlBytes)
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	page := render.NotFoundPage{
		Site: s.cfg.Site,
		Path: r.URL.Path,
	}
	htmlBytes, err := s.tpl.RenderNotFound(r.Context(), page)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	writeHTML(w, htmlBytes)
}

func (s *Server) handleArchives(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/archives" && r.URL.Path != "/archives/" {
		s.handleNotFound(w, r)
		return
	}

	metas, err := s.idx.List(index.ListOptions{
		Sort:         config.SortCreated,
		Page:         1,
		Size:         1000000,
		IncludeDraft: true,
	})
	if err != nil {
		log.Printf("archives query error: %v", err)
		http.Error(w, "archives query error", http.StatusInternalServerError)
		return
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Date.After(metas[j].Date)
	})

	groupsMap := make(map[int][]content.ArticleMeta)
	for _, m := range metas {
		if m.Hidden {
			continue
		}
		y := m.Date.Year()
		groupsMap[y] = append(groupsMap[y], m)
	}

	years := make([]int, 0, len(groupsMap))
	for y := range groupsMap {
		years = append(years, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(years)))

	groups := make([]render.ArchivesGroup, 0, len(years))
	total := 0
	for _, y := range years {
		posts := groupsMap[y]
		total += len(posts)
		groups = append(groups, render.ArchivesGroup{
			Year:  y,
			Posts: posts,
			Count: len(posts),
		})
	}

	page := render.ArchivesPage{
		Site:   s.cfg.Site,
		Groups: groups,
		Total:  total,
	}

	htmlBytes, err := s.tpl.RenderArchives(r.Context(), page)
	if err != nil {
		log.Printf("render archives error: %v", err)
		http.Error(w, "render archives error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, htmlBytes)
}

func (s *Server) handleTagsRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/tags" && r.URL.Path != "/tags/" {
		s.handleNotFound(w, r)
		return
	}

	metas, err := s.idx.List(index.ListOptions{
		Sort:         s.cfg.Site.SortMode,
		Page:         1,
		Size:         1000000,
		IncludeDraft: true,
	})
	if err != nil {
		log.Printf("tags query error: %v", err)
		http.Error(w, "tags query error", http.StatusInternalServerError)
		return
	}

	counts := make(map[string]int)
	for _, m := range metas {
		if m.Hidden {
			continue
		}
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
		stats = append(stats, render.TagStat{Name: name, Count: c})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})

	page := render.TagsPage{
		Site:  s.cfg.Site,
		Tags:  stats,
		Total: len(stats),
	}
	htmlBytes, err := s.tpl.RenderTagsPage(r.Context(), page)
	if err != nil {
		log.Printf("render tags overview error: %v", err)
		http.Error(w, "render tags overview error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, htmlBytes)
}

func (s *Server) handleCategoriesRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/categories" && r.URL.Path != "/categories/" {
		s.handleNotFound(w, r)
		return
	}

	metas, err := s.idx.List(index.ListOptions{
		Sort:         s.cfg.Site.SortMode,
		Page:         1,
		Size:         1000000,
		IncludeDraft: true,
	})
	if err != nil {
		log.Printf("categories query error: %v", err)
		http.Error(w, "categories query error", http.StatusInternalServerError)
		return
	}

	counts := make(map[string]int)
	for _, m := range metas {
		if m.Hidden {
			continue
		}
		c := strings.TrimSpace(m.Category)
		if c == "" {
			continue
		}
		counts[c]++
	}

	stats := make([]render.CategoryStat, 0, len(counts))
	for name, c := range counts {
		stats = append(stats, render.CategoryStat{Name: name, Count: c})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count == stats[j].Count {
			return stats[i].Name < stats[j].Name
		}
		return stats[i].Count > stats[j].Count
	})

	page := render.CategoriesPage{
		Site:       s.cfg.Site,
		Categories: stats,
		Total:      len(stats),
	}
	htmlBytes, err := s.tpl.RenderCategoriesPage(r.Context(), page)
	if err != nil {
		log.Printf("render categories overview error: %v", err)
		http.Error(w, "render categories overview error", http.StatusInternalServerError)
		return
	}
	writeHTML(w, htmlBytes)
}

func (s *Server) handleStaticSlug(slug string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+slug && r.URL.Path != "/"+slug+"/" {
			s.handleNotFound(w, r)
			return
		}

		s.mu.RLock()
		art, ok := s.articles[slug]
		s.mu.RUnlock()
		if !ok {
			s.handleNotFound(w, r)
			return
		}

		meta := art.Meta
		src, err := os.ReadFile(art.Body.SourcePath)
		if err != nil {
			log.Printf("read source error: %v", err)
			http.Error(w, "read source error", http.StatusInternalServerError)
			return
		}
		_, body, fmErr := ingest.ParseFrontMatter(src)
		if fmErr != nil {
			body = src
		}

		mdResult, err := s.md.Render(body)
		if err != nil {
			log.Printf("markdown render error: %v", err)
			http.Error(w, "markdown render error", http.StatusInternalServerError)
			return
		}

		pp := render.PostPage{
			Site:       s.cfg.Site,
			Meta:       meta,
			HTML:       template.HTML(mdResult.HTML),
			TOC:        mdResult.Headings,
			IsDraft:    meta.Draft,
			SeriesName: meta.Series.Name,
			PageTitle:  meta.Title,
		}

		htmlBytes, err := s.tpl.RenderPost(r.Context(), pp)
		if err != nil {
			log.Printf("render page error: %v", err)
			http.Error(w, "render page error", http.StatusInternalServerError)
			return
		}
		writeHTML(w, htmlBytes)
	}
}

// ===================== 工具 =====================

func writeHTML(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}
