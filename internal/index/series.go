package index

import "time"

type SeriesSummary struct {
	Name               string
	Count              int
	LatestUpdated      time.Time
	MaxSticky          int
	RepresentativeSlug string
}
