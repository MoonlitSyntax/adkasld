package index

import (
	"encoding/json"
	bolt "go.etcd.io/bbolt"
	"mygo/internal/domain/content"
	"strings"
)

type RebuildOptions struct {
	IncludeDraft bool
}

func (s *Store) Rebuild(articles []content.Article, opt RebuildOptions) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		_ = tx.DeleteBucket(bMeta)
		_ = tx.DeleteBucket(bAlias)
		_ = tx.DeleteBucket(bShort)
		_ = tx.DeleteBucket(bIdx)
		_ = tx.DeleteBucket(bIdxTag)
		_ = tx.DeleteBucket(bIdxCat)
		_ = tx.DeleteBucket(bIdxSeries)
		_ = tx.DeleteBucket(bIdxUpdated)
		_ = tx.DeleteBucket(bIdxCreated)

		metaB, _ := tx.CreateBucket(bMeta)
		aliasB, _ := tx.CreateBucket(bAlias)
		shortB, _ := tx.CreateBucket(bShort)

		idxUpdatedB, _ := tx.CreateBucket(bIdxUpdated)
		idxCreatedB, _ := tx.CreateBucket(bIdxCreated)

		idxTagB, _ := tx.CreateBucket(bIdxTag)
		idxCatB, _ := tx.CreateBucket(bIdxCat)
		idxSeriesB, _ := tx.CreateBucket(bIdxSeries)

		for _, a := range articles {
			m := a.Meta
			if m.Draft && !opt.IncludeDraft {
				continue
			}
			if strings.TrimSpace(m.Slug) == "" {
				continue
			}
			mb, err := json.Marshal(m)
			if err != nil {
				return err
			}
			if err := metaB.Put([]byte(m.Slug), mb); err != nil {
				return err
			}

			uKey := makeStickyTimeSlugKey(m.Sticky, m.Updated.UnixNano(), m.Slug)
			if err := idxUpdatedB.Put(uKey, []byte{1}); err != nil {
				return err
			}

			cKey := makeStickyTimeSlugKey(m.Sticky, m.Date.UnixNano(), m.Slug)
			if err := idxCreatedB.Put(cKey, []byte{1}); err != nil {
				return err
			}

			for _, tag := range m.Tags {
				if tag == "" {
					continue
				}
				sb, err := idxTagB.CreateBucketIfNotExists([]byte(tag))
				if err != nil {
					return err
				}
				if err := sb.Put(uKey, []byte{1}); err != nil {
					return err
				}

			}

			if cat := strings.TrimSpace(m.Category); cat != "" {
				sb, err := idxCatB.CreateBucketIfNotExists([]byte(cat))
				if err != nil {
					return err
				}
				if err := sb.Put(uKey, []byte{1}); err != nil {
					return err
				}
			}

			if sn := strings.TrimSpace(m.Series.Name); sn != "" {
				sb, err := idxSeriesB.CreateBucketIfNotExists([]byte(sn))
				if err != nil {
					return err
				}
				sKey := makeSeriesKey(m.Series.Order, m.Updated.UnixNano(), m.Slug)
				if err := sb.Put(sKey, []byte{1}); err != nil {
					return err
				}
			}
			for _, old := range m.Aliases {
				old = strings.TrimSpace(old)
				if old == "" {
					continue
				}
				if err := aliasB.Put([]byte(old), []byte(m.Slug)); err != nil {
					return err
				}
			}
			if sid := strings.TrimSpace(m.ShortID); sid != "" {
				if err := shortB.Put([]byte(sid), []byte(m.Slug)); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func makeSeriesKey(order int, updatedUnixNano int64, slug string) []byte {
	buf := make([]byte, 0, 8+8+1+len(slug))
	tmp := make([]byte, 8)

	if order < 0 {
		order = 0
	}
	putU64(tmp, uint64(order))
	buf = append(buf, tmp...)

	putU64(tmp, ^uint64(updatedUnixNano))
	buf = append(buf, tmp...)
	buf = append(buf, 0x00)
	buf = append(buf, []byte(slug)...)
	return buf
}

func putU64(dst []byte, v uint64) {
	dst[0] = byte(v >> 56)
	dst[1] = byte(v >> 48)
	dst[2] = byte(v >> 40)
	dst[3] = byte(v >> 32)
	dst[4] = byte(v >> 24)
	dst[5] = byte(v >> 16)
	dst[6] = byte(v >> 8)
	dst[7] = byte(v)
}
