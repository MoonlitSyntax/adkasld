package ingest

import (
	"mygo/internal/domain/content"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Warning struct {
	Path string
	Msg  string
}
type Result struct {
	Article content.Article
	Warns   []Warning
	Skip    bool
	Err     error
}

type Options struct {
	SourceDir string
}

func Ingest(sourceDir string) ([]content.Article, []Warning, error) {
	files, err := DiscoverSource(sourceDir)
	if err != nil {
		return nil, nil, err
	}

	workers := runtime.GOMAXPROCS(0)
	jobs := make(chan SourceFile)
	results := make(chan Result)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sf := range jobs {
				st, statErr := os.Stat(sf.Path)
				if statErr != nil {
					results <- Result{Err: statErr}
					continue
				}
				raw, readErr := os.ReadFile(sf.Path)
				if readErr != nil {
					results <- Result{Err: readErr}
					continue
				}
				contentHash := HashBytes(raw)

				fm, _, fmErr := ParseFrontMatter(raw)

				var warns []Warning
				if fmErr != nil && fmErr != errNoFrontMatter {
					warns = append(warns, Warning{
						Path: sf.Path,
						Msg:  "failed to parse front matter: " + fmErr.Error(),
					})
					results <- Result{Warns: warns, Skip: true}
					continue
				}
				if fm.Hidden {
					results <- Result{Skip: true}
					continue
				}
				slug := ResolveSlug(fm, sf.Path)
				if slug == "" {
					warns = append(warns, Warning{Path: sf.Path, Msg: "empty slug"})
					results <- Result{Warns: warns, Skip: true}
					continue
				}
				meta := content.ArticleMeta{
					Title:    fm.Title,
					Slug:     slug,
					Tags:     fm.Tags,
					Category: fm.Category,
					Sticky:   fm.Sticky,
					Hidden:   fm.Hidden,
					Draft:    fm.Draft,
					Cover:    fm.Cover,
					Aliases:  fm.Aliases,
				}
				meta.Series = content.Series{Name: fm.Series.Name, Order: fm.Series.Order}
				mt := st.ModTime().In(time.Local)
				meta.Date = ParseTime(fm.Date)
				meta.Updated = ParseTime(fm.Updated)
				if meta.Date.IsZero() {
					meta.Date = mt
					warns = append(warns, Warning{
						Path: sf.Path,
						Msg:  "using file modification time for date",
					})
				}
				if meta.Updated.IsZero() {
					meta.Updated = meta.Date
				}
				if strings.TrimSpace(meta.Title) == "" {
					warns = append(warns, Warning{Path: sf.Path, Msg: "title is empty"})
				}
				meta.Normalize()
				results <- Result{
					Article: content.Article{
						Meta: meta,
						Body: content.BodyRef{
							SourcePath:  sf.Path,
							ContentHash: contentHash,
						},
					},
					Warns: warns,
				}
			}
		}()
	}

	go func() {
		for _, f := range files {
			jobs <- f
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	var out []content.Article
	var warns []Warning
	for r := range results {
		if r.Err != nil {
			return nil, nil, r.Err
		}
		if len(r.Warns) > 0 {
			warns = append(warns, r.Warns...)
		}
		if r.Skip {
			continue
		}
		out = append(out, r.Article)
	}
	seen := make(map[string]struct{}, len(out))
	filtered := make([]content.Article, 0, len(out))
	for _, a := range out {
		if _, ok := seen[a.Meta.Slug]; ok {
			warns = append(warns, Warning{Path: a.Body.SourcePath, Msg: "slug 冲突（重复），已跳过: " + a.Meta.Slug})
			continue
		}
		seen[a.Meta.Slug] = struct{}{}
		filtered = append(filtered, a)
	}
	return filtered, warns, nil
}
