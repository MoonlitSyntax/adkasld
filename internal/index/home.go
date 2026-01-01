package index

import (
	"mygo/internal/domain/config"
	"mygo/internal/domain/content"
	"sort"
)

type HomeItemKind string

const (
	HomePost   HomeItemKind = "post"
	HomeSeries HomeItemKind = "series"
)

type HomeItem struct {
	Kind   HomeItemKind
	Meta   *content.ArticleMeta
	Series *SeriesSummary
}

func (s *Store) HomeItems(opt ListOptions) ([]HomeItem, error) {
	metas, err := s.List(opt)
	if err != nil {
		return nil, err
	}
	seenSeries := make(map[string]struct{})
	var items []HomeItem

	for _, m := range metas {
		if m.Series.Name == "" {
			items = append(items, HomeItem{
				Kind: HomePost,
				Meta: &m,
			})
			continue
		}

		if _, ok := seenSeries[m.Series.Name]; ok {
			continue
		}
		seenSeries[m.Series.Name] = struct{}{}

		sum, err := s.GetSeriesSummary(m.Series.Name, opt.IncludeDraft)
		if err != nil {
			continue
		}

		items = append(items, HomeItem{
			Kind:   HomeSeries,
			Series: sum,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		ai, aj := items[i], items[j]

		si, sj := 0, 0
		ti, tj := int64(0), int64(0)

		if ai.Kind == HomePost {
			si = ai.Meta.Sticky
			if opt.Sort == config.SortCreated {
				ti = ai.Meta.Date.UnixNano()
			} else {
				ti = ai.Meta.Updated.UnixNano()
			}
		} else {
			si = ai.Series.MaxSticky
			ti = ai.Series.LatestUpdated.UnixNano()
		}

		if aj.Kind == HomePost {
			sj = aj.Meta.Sticky
			if opt.Sort == config.SortCreated {
				tj = aj.Meta.Date.UnixNano()
			} else {
				tj = aj.Meta.Updated.UnixNano()
			}
		} else {
			si = ai.Series.MaxSticky
			ti = ai.Series.LatestUpdated.UnixNano()
		}

		if si != sj {
			return si > sj
		}
		return ti > tj
	})
	return items, nil
}
