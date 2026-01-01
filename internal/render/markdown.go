package render

import (
	"bytes"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

type MarkdownRenderer struct {
	md goldmark.Markdown
}

func NewMarkdownRenderer() *MarkdownRenderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Linkify,
			extension.Strikethrough,
			extension.Table,
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	return &MarkdownRenderer{md: md}
}

type MarkdownResult struct {
	HTML     []byte
	Headings []Heading
}

func (r *MarkdownRenderer) Render(src []byte) (MarkdownResult, error) {
	var buf bytes.Buffer

	ctx := parser.NewContext()
	reader := text.NewReader(src)
	doc := r.md.Parser().Parse(reader, parser.WithContext(ctx))

	var heads []Heading
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok {
			level := h.Level
			var idStr string
			if id, ok := h.AttributeString("id"); ok {
				switch v := id.(type) {
				case string:
					idStr = v
				case []byte:
					idStr = string(v)
				}
			}
			var textBuf bytes.Buffer
			for c := h.FirstChild(); c != nil; c = c.NextSibling() {
				if seg, ok := c.(*ast.Text); ok {
					textBuf.Write(seg.Segment.Value(src))
				}
			}
			heads = append(heads, Heading{
				Level: level,
				ID:    idStr,
				Text:  textBuf.String(),
			})
		}
		return ast.WalkContinue, nil
	})

	if err := r.md.Renderer().Render(&buf, src, doc); err != nil {
		return MarkdownResult{}, err
	}
	return MarkdownResult{
		HTML:     buf.Bytes(),
		Headings: heads,
	}, nil
}
