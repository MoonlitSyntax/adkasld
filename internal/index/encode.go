package index

import (
	"bytes"
	"encoding/binary"
)

func clampSticky(s int) uint16 {
	if s < 0 {
		s = 0
	}
	if s > 100 {
		s = 100
	}
	return uint16(s)
}

// key = invSticky(2) + invTime(8) + 0x00 + slug
func makeStickyTimeSlugKey(sticky int, unixNano int64, slug string) []byte {
	invSticky := ^clampSticky(sticky)
	invTime := ^uint64(unixNano)

	buf := make([]byte, 0, 2+8+1+len(slug))

	tmp2 := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp2, invSticky)
	buf = append(buf, tmp2...)

	tmp8 := make([]byte, 8)
	binary.BigEndian.PutUint64(tmp8, invTime)
	buf = append(buf, tmp8...)

	buf = append(buf, 0x00)
	buf = append(buf, []byte(slug)...)
	return buf
}

func slugFromStickyTimeSlugKey(k []byte) string {
	// invSticky(2) + invTime(8) + 0x00 + slug
	if len(k) < 2+8+2 {
		return ""
	}
	i := bytes.IndexByte(k[10:], 0x00)
	if i < 0 {
		return ""
	}
	pos := 10 + i
	if pos+1 >= len(k) {
		return ""
	}
	return string(k[pos+1:])
}
