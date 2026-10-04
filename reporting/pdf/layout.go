package pdf

import (
	"fmt"
	"strings"
)

// Layout constants (points). A4 page, 1-inch-ish margins.
const (
	marginLeft   = 54.0
	marginRight  = 54.0
	marginTop    = 54.0
	marginBottom = 54.0

	bodySize    = 10.0
	headingSize = 14.0
	titleSize   = 20.0
	leading     = 1.35 // line-height multiplier
)

// contentWidth is the usable horizontal space.
const contentWidth = pageWidthPt - marginLeft - marginRight

// layout is a top-down flow engine over a document. It owns a cursor and starts
// new pages automatically when content passes the bottom margin.
type layout struct {
	doc  *document
	cur  *page
	curY float64 // current baseline-top position from the page top (grows down)
}

// newLayout starts a layout on a fresh first page.
func newLayout(doc *document) *layout {
	l := &layout{doc: doc}
	l.newPage()
	return l
}

func (l *layout) newPage() {
	l.cur = l.doc.addPage()
	l.curY = marginTop
}

// remaining returns the vertical space left before the bottom margin.
func (l *layout) remaining() float64 {
	return (pageHeightPt - marginBottom) - l.curY
}

// ensure makes sure at least h points are available, starting a new page if not.
func (l *layout) ensure(h float64) {
	if l.remaining() < h {
		l.newPage()
	}
}

// pdfY converts a top-origin Y (points from page top) to PDF user space
// (origin bottom-left).
func pdfY(topY float64) float64 { return pageHeightPt - topY }

// drawText writes a single line of text at (x, current baseline). It does NOT
// advance the cursor; callers that flow text use writeLine/paragraph.
func (l *layout) drawText(x float64, font FontName, size float64, s string) {
	res := l.doc.resourceName(font)
	fmt.Fprintf(&l.cur.content, "BT /%s %s %s %s Td (%s) Tj ET\n",
		res, ftoa(size), ftoa(x), ftoa(pdfY(l.curY+size)), escapeText(s))
}

// writeLine draws one pre-measured line and advances the cursor by its height.
func (l *layout) writeLine(x float64, font FontName, size float64, s string) {
	lineH := size * leading
	l.ensure(lineH)
	l.drawText(x, font, size, s)
	l.curY += lineH
}

// gap advances the cursor by h points (vertical spacing).
func (l *layout) gap(h float64) { l.curY += h }

// title draws the report title.
func (l *layout) title(s string) {
	l.writeLine(marginLeft, HelveticaBold, titleSize, s)
	l.gap(6)
}

// heading draws a bold section heading with spacing and a rule above.
func (l *layout) heading(s string) {
	l.gap(8)
	l.ensure(headingSize*leading + 6)
	l.writeLine(marginLeft, HelveticaBold, headingSize, s)
	// Rule under the heading.
	l.rule(marginLeft, marginLeft+contentWidth)
	l.gap(4)
}

// rule draws a thin horizontal line at the current cursor.
func (l *layout) rule(x0, x1 float64) {
	y := pdfY(l.curY)
	fmt.Fprintf(&l.cur.content, "%s %s m %s %s l 0.5 w S\n",
		ftoa(x0), ftoa(y), ftoa(x1), ftoa(y))
	l.gap(3)
}

// paragraph flows wrapped body text across lines (and pages).
func (l *layout) paragraph(font FontName, size float64, s string) {
	for _, line := range wrap(font, size, s, contentWidth) {
		l.writeLine(marginLeft, font, size, line)
	}
}

// keyValue draws a bold key followed by its (wrapped) value, indented.
func (l *layout) keyValue(key, value string) {
	keyText := key + ": "
	keyW := TextWidth(HelveticaBold, bodySize, keyText)
	valX := marginLeft + keyW
	avail := contentWidth - keyW
	if avail < 60 {
		avail = contentWidth
	}
	lines := wrap(Helvetica, bodySize, value, avail)
	if len(lines) == 0 {
		lines = []string{""}
	}
	// First line: key + first value line on the same baseline.
	lineH := bodySize * leading
	l.ensure(lineH)
	l.drawText(marginLeft, HelveticaBold, bodySize, keyText)
	l.drawText(valX, Helvetica, bodySize, lines[0])
	l.curY += lineH
	for _, line := range lines[1:] {
		l.writeLine(valX, Helvetica, bodySize, line)
	}
}

// table draws a header row + body rows with measured columns and rules. Column
// widths are proportional to the given weights.
func (l *layout) table(headers []string, rows [][]string, weights []float64) {
	if len(headers) == 0 {
		return
	}
	cols := columnWidths(weights, contentWidth)
	// Header.
	l.tableRow(headers, cols, HelveticaBold)
	l.rule(marginLeft, marginLeft+contentWidth)
	// Body.
	for _, r := range rows {
		l.tableRow(r, cols, Helvetica)
	}
	l.gap(4)
}

// tableRow draws one row, wrapping each cell and sizing the row to the tallest
// cell. A page break before the row keeps it intact when possible.
func (l *layout) tableRow(cells []string, cols []float64, font FontName) {
	n := len(cols)
	wrapped := make([][]string, n)
	maxLines := 1
	for i := 0; i < n; i++ {
		var text string
		if i < len(cells) {
			text = cells[i]
		}
		w := cols[i] - 4 // small cell padding
		if w < 10 {
			w = cols[i]
		}
		wrapped[i] = wrap(font, bodySize, text, w)
		if len(wrapped[i]) == 0 {
			wrapped[i] = []string{""}
		}
		if len(wrapped[i]) > maxLines {
			maxLines = len(wrapped[i])
		}
	}
	rowH := float64(maxLines) * bodySize * leading
	l.ensure(rowH)
	startY := l.curY
	for li := 0; li < maxLines; li++ {
		x := marginLeft
		for ci := 0; ci < n; ci++ {
			if li < len(wrapped[ci]) {
				l.drawText(x, font, bodySize, wrapped[ci][li])
			}
			x += cols[ci]
		}
		l.curY += bodySize * leading
	}
	// Make sure the cursor advanced the full row height even if a cell was
	// short (keeps columns aligned on subsequent rows).
	if l.curY < startY+rowH {
		l.curY = startY + rowH
	}
}

// bullet draws a wrapped bullet-list item.
func (l *layout) bullet(s string) {
	indent := 12.0
	lines := wrap(Helvetica, bodySize, s, contentWidth-indent)
	if len(lines) == 0 {
		lines = []string{""}
	}
	lineH := bodySize * leading
	l.ensure(lineH)
	l.drawText(marginLeft, Helvetica, bodySize, "-")
	l.drawText(marginLeft+indent, Helvetica, bodySize, lines[0])
	l.curY += lineH
	for _, line := range lines[1:] {
		l.writeLine(marginLeft+indent, Helvetica, bodySize, line)
	}
}

// columnWidths distributes total width across weights.
func columnWidths(weights []float64, total float64) []float64 {
	var sum float64
	for _, w := range weights {
		sum += w
	}
	if sum == 0 {
		sum = float64(len(weights))
		for i := range weights {
			weights[i] = 1
		}
	}
	out := make([]float64, len(weights))
	for i, w := range weights {
		out[i] = total * w / sum
	}
	return out
}

// wrap splits s into lines no wider than maxWidth points, breaking on spaces
// and hard-breaking any single token that is itself too wide (e.g. a hash).
func wrap(font FontName, size float64, s string, maxWidth float64) []string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil
	}
	var lines []string
	var cur string
	for _, word := range fields {
		for TextWidth(font, size, word) > maxWidth && len(word) > 1 {
			// Hard-break an over-wide token.
			cut := breakToken(font, size, word, maxWidth)
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			lines = append(lines, word[:cut])
			word = word[cut:]
		}
		candidate := word
		if cur != "" {
			candidate = cur + " " + word
		}
		if TextWidth(font, size, candidate) <= maxWidth {
			cur = candidate
		} else {
			if cur != "" {
				lines = append(lines, cur)
			}
			cur = word
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// breakToken returns the byte count of the longest prefix of word that fits in
// maxWidth. It operates on runes to stay valid-UTF-8 safe.
func breakToken(font FontName, size float64, word string, maxWidth float64) int {
	var w float64
	count := 0
	for _, r := range word {
		rw := float64(RuneWidth(font, r)) / 1000.0 * size
		if w+rw > maxWidth && count > 0 {
			return count
		}
		w += rw
		count += len(string(r))
	}
	if count == 0 {
		return len(word)
	}
	return count
}

// escapeText escapes a string for a PDF literal string: backslash, parentheses
// and control characters. Non-ASCII runes are dropped to stay within
// WinAnsi/ASCII (the report content is ASCII; this is a safety net).
func escapeText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '(':
			b.WriteString("\\(")
		case ')':
			b.WriteString("\\)")
		case '\n', '\r', '\t':
			b.WriteByte(' ')
		default:
			if r >= 32 && r <= 126 {
				b.WriteRune(r)
			}
			// runes outside printable ASCII are omitted
		}
	}
	return b.String()
}

// ftoa formats a float for PDF output with up to 2 decimals, trimming trailing
// zeros, so numeric output is compact and deterministic.
func ftoa(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}
