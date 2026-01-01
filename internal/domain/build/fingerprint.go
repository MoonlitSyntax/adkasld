package build

import (
	"crypto/sha256"
	"encoding/hex"
)

type Fingerprint struct {
	ContentHash  string
	ThemeHash    string
	ConfigHash   string
	RendererHash string
	RenderHash   string
}

func (f *Fingerprint) ComputeRenderHash() {
	h := sha256.New()
	h.Write([]byte(f.ContentHash))
	h.Write([]byte(f.ThemeHash))
	h.Write([]byte(f.ConfigHash))
	h.Write([]byte(f.RendererHash))
	f.RenderHash = hex.EncodeToString(h.Sum(nil))
}
