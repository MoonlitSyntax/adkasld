package app

import (
	"fmt"
	"mygo/internal/domain/content"
	"mygo/internal/domain/site"
	"mygo/internal/index"
	"path/filepath"
)

type RouteBuilder struct {
	Index *index.Store
}

func (rb *RouteBuilder) BuildPostRoutes(articles []content.ArticleMeta) []site.Route {
	var routes []site.Route
	for _, m := range articles {
		y, mo, d := m.Date.Date()
		out := filepath.Join(
			"post",
			fmt.Sprintf("%04d", y),
			fmt.Sprintf("%02d", mo),
			fmt.Sprintf("%02d", d),
			m.Slug,
			"index.html",
		)
		routes = append(routes, site.Route{
			Kind:    site.RoutePost,
			Slug:    m.Slug,
			OutPath: out,
		})
	}
	return routes
}

func (rb *RouteBuilder) BuildSeriesRoutes() ([]site.Route, error) {
	names, err := rb.Index.ListAllSeriesNames()
	if err != nil {
		return nil, err
	}
	var routes []site.Route
	for _, name := range names {
		out := filepath.Join("series", name, "index.html")
		routes = append(routes, site.Route{
			Kind:    site.RouteSeries,
			Slug:    name,
			OutPath: out,
		})
	}
	return routes, nil
}
