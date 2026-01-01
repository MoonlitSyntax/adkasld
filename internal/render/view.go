package render

import (
	"html/template"
	"mygo/internal/domain/config"
	"mygo/internal/domain/content"
	"time"
)

type Heading struct {
	Level int
	ID    string
	Text  string
}

type PostPage struct {
	Site config.SiteConfig
	Meta content.ArticleMeta
	HTML template.HTML
	TOC  []Heading

	SeriesName string
	SeriesList []content.ArticleMeta

	Related []content.ArticleMeta
	IsDraft bool
	Title   string
}

type ListPage struct {
	Site      config.SiteConfig
	Title     string
	SubTitle  string
	Items     []content.ArticleMeta
	Page      int
	PageSize  int
	Total     int
	Tag       string
	Category  string
	Generated time.Time
}

type SeriesPage struct {
	Site   config.SiteConfig
	Name   string
	Intro  string
	Items  []content.ArticleMeta
	Count  int
	Latest time.Time
	Title  string
}

type HomeItemKind string

const (
	HomeItemPost   HomeItemKind = "post"
	HomeItemSeries HomeItemKind = "series"
)

type HomePostItem struct {
	Meta content.ArticleMeta
}

type HomeSeriesItem struct {
	Name               string
	Count              int
	LatestUpdated      time.Time
	MaxSticky          int
	RepresentativePost content.ArticleMeta
}

type HomeItem struct {
	Kind   HomeItemKind
	Post   *HomePostItem
	Series *HomeSeriesItem
}

type HomePage struct {
	Site      config.SiteConfig
	Items     []HomeItem
	Page      int
	PageSize  int
	Generated time.Time
	Title     string
}

type NotFoundPage struct {
	Site config.SiteConfig
	Path string
}

type ArchivesGroup struct {
	Year  int
	Posts []content.ArticleMeta
	Count int
}

type ArchivesPage struct {
	Site   config.SiteConfig
	Groups []ArchivesGroup
	Total  int
	Title  string
}

type TagStat struct {
	Name  string
	Count int
}

type TagsPage struct {
	Site  config.SiteConfig
	Tags  []TagStat
	Total int
	Title string
}

type CategoryStat struct {
	Name  string
	Count int
}

type CategoriesPage struct {
	Site       config.SiteConfig
	Categories []CategoryStat
	Total      int
	Title      string
}
