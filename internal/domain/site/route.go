package site

import (
	"fmt"
	"strings"
)

type RouteKind string

const (
	RouteIndex    RouteKind = "index"
	RoutePost     RouteKind = "post"
	RouteTag      RouteKind = "tag"
	RouteCategory RouteKind = "category"
	RouteArchive  RouteKind = "archive"
	RouteAbout    RouteKind = "about"
	RouteLinks    RouteKind = "links"
	RouteRSS      RouteKind = "rss"
	RouteAtom     RouteKind = "atom"
	RouteSitemap  RouteKind = "sitemap"
	RouteRobots   RouteKind = "robots"
	RouteNotFound RouteKind = "404"
	RouteShort    RouteKind = "short"
	RouteSeries   RouteKind = "series"
)

type Route struct {
	Kind    RouteKind
	Slug    string
	Key     string
	Page    int
	OutPath string
}

func (r Route) String() string {
	var parts []string
	parts = append(parts, string(r.Kind))
	if r.Slug != "" {
		parts = append(parts, "slug="+r.Slug)
	}
	if r.Key != "" {
		parts = append(parts, "key="+r.Key)
	}
	if r.Page > 0 {
		parts = append(parts, fmt.Sprintf("page=%d", r.Page))
	}
	if r.OutPath != "" {
		parts = append(parts, "out="+r.OutPath)
	}
	return strings.Join(parts, " ")
}
