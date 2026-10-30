// Package content converts the raw XHTML served by WeRead into the small,
// renderer-facing Block model. The renderer must never see vendor markup.
package content

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"my-weread-desk/internal/weread"
)

// ImageResolver maps an original image source to a locally served URL.
// Returning false leaves the block without a source, so the reader shows a
// placeholder instead of a broken authenticated request.
type ImageResolver func(src string) (string, bool)

// Parse converts an HTML fragment into blocks.
func Parse(fragment string, resolve ImageResolver) []weread.Block {
	root, err := html.Parse(strings.NewReader("<body>" + fragment + "</body>"))
	if err != nil {
		return []weread.Block{{Kind: "para", Text: collapseWhitespace(stripTags(fragment))}}
	}
	body := findBody(root)
	if body == nil {
		return nil
	}
	p := &parser{resolve: resolve}
	p.walkChildren(body)
	return normalise(p.blocks)
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == atom.Body {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findBody(c); found != nil {
			return found
		}
	}
	return nil
}

type parser struct {
	blocks  []weread.Block
	resolve ImageResolver
}

func (p *parser) walkChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.walk(c)
	}
}

func (p *parser) add(b weread.Block) {
	if b.Kind == "para" || b.Kind == "quote" || b.Kind == "pre" {
		b.Text = strings.TrimSpace(b.Text)
		if b.Text == "" {
			return
		}
	}
	p.blocks = append(p.blocks, b)
}

func (p *parser) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		text := normaliseText(n.Data)
		if strings.TrimSpace(text) != "" {
			p.add(weread.Block{Kind: "para", Text: text})
		}
	case html.ElementNode:
		p.walkElement(n)
	}
}

func (p *parser) walkElement(n *html.Node) {
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Head, atom.Noscript, atom.Iframe, atom.Svg:
		return
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := int(n.Data[1] - '0')
		text := collapseWhitespace(inlineText(n))
		if text != "" {
			p.add(weread.Block{Kind: "heading", Level: level, Text: text})
		}
		return
	case atom.Hr:
		p.add(weread.Block{Kind: "divider"})
		return
	case atom.Img:
		p.addImage(n)
		return
	case atom.Image:
		// Some EPUBs use <image xlink:href>.
		if src := attr(n, "xlink:href"); src != "" {
			p.addImageSrc(src, attr(n, "alt"))
		} else if src := attr(n, "href"); src != "" {
			p.addImageSrc(src, attr(n, "alt"))
		}
		return
	case atom.Blockquote:
		text := collapseWhitespace(inlineText(n))
		if text != "" {
			p.add(weread.Block{Kind: "quote", Text: text})
		}
		return
	case atom.Pre:
		text := strings.TrimRight(textContent(n), "\n")
		if strings.TrimSpace(text) != "" {
			p.add(weread.Block{Kind: "pre", Text: text})
		}
		return
	case atom.Ul, atom.Ol:
		p.walkList(n)
		return
	case atom.Table:
		p.walkTable(n)
		return
	case atom.P:
		p.addParagraph(n)
		return
	case atom.Div, atom.Section, atom.Article, atom.Main, atom.Header, atom.Footer, atom.Aside, atom.Figcaption, atom.Li:
		if hasBlockChildren(n) {
			p.walkChildren(n)
			return
		}
		p.addParagraph(n)
		return
	case atom.Body:
		p.walkChildren(n)
		return
	}
	// Inline element at block level: fold it into a paragraph.
	p.addParagraph(n)
}

func (p *parser) addParagraph(n *html.Node) {
	text := collapseWhitespace(inlineText(n))
	images := collectImages(n)
	if len(images) > 0 {
		// Figures often wrap the image in the same paragraph as a literal
		// placeholder ("[图片]") or a caption. The placeholder is noise and must
		// not hide the image, which is what happened when any text made the
		// parser take the text branch and ignore the <img>.
		if text != "" && !isImagePlaceholder(text) {
			kind := "para"
			if hasClass(n, "quote") || hasClass(n, "blockquote") {
				kind = "quote"
			}
			p.add(weread.Block{Kind: kind, Text: text, Class: classHint(n)})
		}
		for _, img := range images {
			p.addImage(img)
		}
		return
	}
	if text == "" {
		return
	}
	kind := "para"
	if hasClass(n, "quote") || hasClass(n, "blockquote") {
		kind = "quote"
	}
	p.add(weread.Block{Kind: kind, Text: text, Class: classHint(n)})
}

func (p *parser) addImage(n *html.Node) {
	src := firstOf(attr(n, "src"), attr(n, "data-src"), attr(n, "data-original"), attr(n, "data-lazy-src"))
	if src == "" {
		if srcset := attr(n, "srcset"); srcset != "" {
			candidate := strings.TrimSpace(strings.Split(srcset, ",")[0])
			if fields := strings.Fields(candidate); len(fields) > 0 {
				src = fields[0]
			}
		}
	}
	if src == "" {
		return
	}
	p.addImageSrc(src, firstOf(attr(n, "alt"), attr(n, "title")))
}

func (p *parser) addImageSrc(src, alt string) {
	if strings.HasPrefix(src, "data:") {
		p.add(weread.Block{Kind: "image", Src: src, Alt: alt})
		return
	}
	if p.resolve != nil {
		if local, ok := p.resolve(src); ok {
			p.add(weread.Block{Kind: "image", Src: local, Alt: alt})
			return
		}
	}
	p.add(weread.Block{Kind: "image", Alt: firstOf(alt, "图片"), Class: "missing"})
}

func (p *parser) walkList(n *html.Node) {
	ordered := n.DataAtom == atom.Ol
	index := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.DataAtom != atom.Li {
			continue
		}
		index++
		text := collapseWhitespace(inlineText(c))
		if text == "" {
			continue
		}
		marker := "•"
		if ordered {
			marker = itoa(index) + "."
		}
		p.add(weread.Block{Kind: "para", Text: marker + " " + text, Class: "list"})
	}
}

func (p *parser) walkTable(n *html.Node) {
	// Tables are flattened into rows of "cell · cell" lines: the reader has no
	// table layout engine and a lost table is worse than a plain listing.
	rows := collect(n, atom.Tr)
	for _, row := range rows {
		var cells []string
		for _, cell := range collect(row, atom.Td) {
			if t := collapseWhitespace(inlineText(cell)); t != "" {
				cells = append(cells, t)
			}
		}
		for _, cell := range collect(row, atom.Th) {
			if t := collapseWhitespace(inlineText(cell)); t != "" {
				cells = append(cells, t)
			}
		}
		if len(cells) > 0 {
			p.add(weread.Block{Kind: "para", Text: strings.Join(cells, " · "), Class: "table"})
		}
	}
}

func collect(n *html.Node, kind atom.Atom) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			if node.DataAtom == kind {
				out = append(out, node)
				return
			}
			if node.DataAtom == atom.Table && kind != atom.Table && len(out) > 0 {
				return
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func collectImages(n *html.Node) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.DataAtom {
			case atom.Img, atom.Image:
				out = append(out, node)
				return
			case atom.Script, atom.Style:
				return
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

// isImagePlaceholder reports whether text is only the reader's stand-in marker
// for an image (for example "[图片]" or "插图"), not a real caption.
func isImagePlaceholder(text string) bool {
	t := strings.ToLower(strings.TrimSpace(strings.Trim(text, "[]【】()（）<>：:")))
	switch t {
	case "图片", "图", "插图", "配图", "image", "img", "photo", "picture":
		return true
	}
	return false
}

func hasBlockChildren(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.DataAtom {
		case atom.P, atom.Div, atom.Section, atom.Article, atom.Blockquote,
			atom.Ul, atom.Ol, atom.Table, atom.H1, atom.H2, atom.H3, atom.H4,
			atom.H5, atom.H6, atom.Hr, atom.Figure, atom.Pre:
			return true
		}
	}
	return false
}

// inlineText collects text from inline descendants, adding line breaks for
// <br> and ignoring nested block structure.
func inlineText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		switch node.Type {
		case html.TextNode:
			b.WriteString(node.Data)
			return
		case html.ElementNode:
			switch node.DataAtom {
			case atom.Br:
				b.WriteString("\n")
				return
			case atom.Script, atom.Style:
				return
			case atom.Img, atom.Image:
				return
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// normaliseText treats a bare text node at block level as a paragraph, keeping
// its internal line breaks as spaces.
func normaliseText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\r", "")
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, " ")
}

func collapseWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", " ")
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func normalise(blocks []weread.Block) []weread.Block {
	out := make([]weread.Block, 0, len(blocks))
	for _, b := range blocks {
		if b.Kind == "para" && len(out) > 0 && out[len(out)-1].Kind == "para" &&
			out[len(out)-1].Class == "" && b.Class == "" && len(b.Text) < 40 {
			// Merge stray short fragments produced by malformed markup.
			if !endsSentence(out[len(out)-1].Text) {
				out[len(out)-1].Text += " " + b.Text
				continue
			}
		}
		out = append(out, b)
	}
	return out
}

func endsSentence(s string) bool {
	if s == "" {
		return true
	}
	last := []rune(s)
	switch last[len(last)-1] {
	case '。', '！', '？', '；', '…', '.', '!', '?', ';', '”', '"':
		return true
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) || strings.EqualFold(a.Namespace+a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, name string) bool {
	cls := attr(n, "class")
	return strings.Contains(strings.ToLower(cls), name)
}

func classHint(n *html.Node) string {
	cls := strings.ToLower(attr(n, "class") + " " + attr(n, "style"))
	switch {
	case strings.Contains(cls, "center") || strings.Contains(cls, "text-align: center"):
		return "center"
	case strings.Contains(cls, "right"):
		return "right"
	case strings.Contains(cls, "no-indent") || strings.Contains(cls, "noindent"):
		return "no-indent"
	case strings.Contains(cls, "code"):
		return "code"
	}
	// Short standalone paragraphs are usually captions or scene separators.
	if text := collapseWhitespace(inlineText(n)); len([]rune(text)) <= 18 && strings.HasPrefix(text, "图") {
		return "caption"
	}
	return ""
}

func firstOf(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
