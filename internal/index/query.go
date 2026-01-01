package index

import (
	"encoding/json"
	"errors"
	bolt "go.etcd.io/bbolt"
	"mygo/internal/domain/config"
	"mygo/internal/domain/content"
	"strings"
)

var ErrNotFound = errors.New("not found")

type ListOptions struct {
	Sort         config.SortMode
	Page         int
	Size         int
	IncludeDraft bool
}

func (s *Store) GetMeta(slug string) (content.ArticleMeta, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return content.ArticleMeta{}, ErrNotFound
	}
	var m content.ArticleMeta
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bMeta)
		if b == nil {
			return ErrNotFound
		}
		v := b.Get([]byte(slug))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &m)
	})
	return m, err
}

func (s *Store) ResolveAlias(slugOrOld string) (string, error) {
	slugOrOld = strings.TrimSpace(slugOrOld)
	if slugOrOld == "" {
		return "", ErrNotFound
	}

	if _, err := s.GetMeta(slugOrOld); err == nil {
		return slugOrOld, nil
	}

	var mapped string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bAlias)
		if b == nil {
			return ErrNotFound
		}
		v := b.Get([]byte(slugOrOld))
		if v == nil {
			return ErrNotFound
		}
		mapped = string(v)
		return nil
	})
	return mapped, err
}

func (s *Store) GetByShortID(shortID string) (string, error) {
	shortID = strings.TrimSpace(shortID)
	if shortID == "" {
		return "", ErrNotFound
	}
	var slug string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bShort)
		if b == nil {
			return ErrNotFound
		}
		v := b.Get([]byte(shortID))
		if v == nil {
			return ErrNotFound
		}
		slug = string(v)
		return nil
	})
	return slug, err
}

func normalizePaging(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func (s *Store) List(opt ListOptions) ([]content.ArticleMeta, error) {
	opt.Page, opt.Size = normalizePaging(opt.Page, opt.Size)

	var idxBucketName []byte
	switch opt.Sort {
	case config.SortCreated:
		idxBucketName = bIdxCreated
	default:
		idxBucketName = bIdxUpdated
	}
	var out []content.ArticleMeta
	err := s.db.View(func(tx *bolt.Tx) error {
		idx := tx.Bucket(idxBucketName)
		metaB := tx.Bucket(bMeta)
		if idx == nil || metaB == nil {
			return nil
		}

		skip := (opt.Page - 1) * opt.Size
		cur := idx.Cursor()

		for k, _ := cur.First(); k != nil; k, _ = cur.Next() {
			slug := slugFromStickyTimeSlugKey(k)
			if slug == "" {
				continue
			}
			v := metaB.Get([]byte(slug))
			if v == nil {
				continue
			}

			var m content.ArticleMeta
			if err := json.Unmarshal(v, &m); err != nil {
				continue
			}
			if m.Hidden {
				continue
			}
			if m.Draft && !opt.IncludeDraft {
				continue
			}
			if skip > 0 {
				skip--
				continue
			}
			out = append(out, m)
			if len(out) >= opt.Size {
				break
			}
		}
		return nil
	})
	return out, err
}

func (s *Store) ListByTag(tag string, opt ListOptions) ([]content.ArticleMeta, error) {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" {
		return nil, nil
	}
	opt.Page, opt.Size = normalizePaging(opt.Page, opt.Size)

	var out []content.ArticleMeta
	err := s.db.View(func(tx *bolt.Tx) error {
		parent := tx.Bucket(bIdxTag)
		metaB := tx.Bucket(bMeta)
		if parent == nil || metaB == nil {
			return nil
		}
		sb := parent.Bucket([]byte(tag))
		if sb == nil {
			return nil
		}

		skip := (opt.Page - 1) * opt.Size
		cur := sb.Cursor()
		for k, _ := cur.First(); k != nil; k, _ = cur.Next() {
			slug := slugFromStickyTimeSlugKey(k)
			v := metaB.Get([]byte(slug))
			if v == nil {
				continue
			}
			var m content.ArticleMeta
			if err := json.Unmarshal(v, &m); err != nil {
				continue
			}
			if m.Hidden {
				continue
			}
			if m.Draft && !opt.IncludeDraft {
				continue
			}
			if skip > 0 {
				skip--
				continue
			}
			out = append(out, m)
			if len(out) >= opt.Size {
				break
			}
		}
		return nil
	})
	return out, err
}

func (s *Store) ListByCategory(cat string, opt ListOptions) ([]content.ArticleMeta, error) {
	cat = strings.TrimSpace(cat)
	if cat == "" {
		return nil, nil
	}
	opt.Page, opt.Size = normalizePaging(opt.Page, opt.Size)

	var out []content.ArticleMeta
	err := s.db.View(func(tx *bolt.Tx) error {
		parent := tx.Bucket(bIdxCat)
		metaB := tx.Bucket(bMeta)
		if parent == nil || metaB == nil {
			return nil
		}
		sb := parent.Bucket([]byte(cat))
		if sb == nil {
			return nil
		}

		skip := (opt.Page - 1) * opt.Size
		cur := sb.Cursor()
		for k, _ := cur.First(); k != nil; k, _ = cur.Next() {
			slug := slugFromStickyTimeSlugKey(k)
			v := metaB.Get([]byte(slug))
			if v == nil {
				continue
			}
			var m content.ArticleMeta
			if err := json.Unmarshal(v, &m); err != nil {
				continue
			}
			if m.Hidden {
				continue
			}
			if m.Draft && !opt.IncludeDraft {
				continue
			}
			if skip > 0 {
				skip--
				continue
			}
			out = append(out, m)
			if len(out) >= opt.Size {
				break
			}
		}
		return nil
	})
	return out, err
}

func (s *Store) ListSeries(name string, opt ListOptions) ([]content.ArticleMeta, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	opt.Page, opt.Size = normalizePaging(opt.Page, opt.Size)

	var out []content.ArticleMeta
	err := s.db.View(func(tx *bolt.Tx) error {
		parent := tx.Bucket(bIdxSeries)
		metaB := tx.Bucket(bMeta)
		if parent == nil || metaB == nil {
			return nil
		}
		sb := parent.Bucket([]byte(name))
		if sb == nil {
			return nil
		}

		skip := (opt.Page - 1) * opt.Size
		cur := sb.Cursor()
		for k, _ := cur.First(); k != nil; k, _ = cur.Next() {
			// series key 的 slug 在 0x00 后
			slug := slugFromSeriesKey(k)
			if slug == "" {
				continue
			}
			v := metaB.Get([]byte(slug))
			if v == nil {
				continue
			}
			var m content.ArticleMeta
			if err := json.Unmarshal(v, &m); err != nil {
				continue
			}
			if m.Hidden {
				continue
			}
			if m.Draft && !opt.IncludeDraft {
				continue
			}
			if skip > 0 {
				skip--
				continue
			}
			out = append(out, m)
			if len(out) >= opt.Size {
				break
			}
		}
		return nil
	})
	return out, err
}

func slugFromSeriesKey(k []byte) string {
	// order(8) + invUpdated(8) + 0x00 + slug
	if len(k) < 8+8+2 {
		return ""
	}
	// 找 0x00 分隔
	for i := 16; i < len(k); i++ {
		if k[i] == 0x00 && i+1 < len(k) {
			return string(k[i+1:])
		}
	}
	return ""
}

func (s *Store) ListAllSeriesNames() ([]string, error) {
	var names []string
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bIdxSeries)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			names = append(names, string(k))
			return nil
		})
	})
	return names, err
}

func (s *Store) GetSeriesSummary(name string, includeDraft bool) (*SeriesSummary, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNotFound
	}
	var sum SeriesSummary
	sum.Name = name
	err := s.db.View(func(tx *bolt.Tx) error {
		parent := tx.Bucket(bIdxSeries)
		metaB := tx.Bucket(bMeta)
		if parent == nil || metaB == nil {
			return ErrNotFound
		}
		sb := parent.Bucket([]byte(name))
		if sb == nil {
			return ErrNotFound
		}
		c := sb.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			slug := slugFromSeriesKey(k)
			if slug == "" {
				continue
			}
			v := metaB.Get([]byte(slug))
			if v == nil {
				continue
			}
			var m content.ArticleMeta
			if err := json.Unmarshal(v, &m); err != nil {
				continue
			}
			if m.Hidden {
				continue
			}
			if m.Draft && !includeDraft {
				continue
			}

			sum.Count++
			if m.Updated.After(sum.LatestUpdated) {
				sum.LatestUpdated = m.Updated
				sum.RepresentativeSlug = m.Slug
			}
			if m.Sticky > sum.MaxSticky {
				sum.MaxSticky = m.Sticky
			}
		}
		if sum.Count == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &sum, nil
}
