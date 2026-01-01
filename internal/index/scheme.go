package index

var (
	bMeta      = []byte("meta")       // slug -> metaBytes
	bAlias     = []byte("alias")      // old -> newSlug
	bShort     = []byte("short")      // shortID -> slug
	bIdx       = []byte("idx")        // parent bucket for indices
	bIdxTag    = []byte("idx_tag")    // tag -> sub-bucket
	bIdxCat    = []byte("idx_cat")    // cat -> sub-bucket
	bIdxSeries = []byte("idx_series") // seriesName -> sub-bucket

	bIdxUpdated = []byte("idx_updated")
	bIdxCreated = []byte("idx_created")
)
