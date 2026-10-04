package pdf

// Core font metrics for the PDF standard-14 base fonts used by the layout
// engine. The widths are the published Adobe AFM character widths (in 1/1000
// of an em) for the WinAnsi/ASCII printable range (code points 32..126). They
// are compiled in so text measurement is exact and fully offline: no AFM file
// is read at runtime and no font program is embedded (the standard-14 fonts are
// guaranteed present in every conforming PDF viewer).
//
// Only the glyphs a forensic report actually emits (printable ASCII) are
// tabled; any code point outside the table falls back to a sane default width
// so measurement never panics.

// FontName identifies one of the supported standard-14 base fonts.
type FontName string

const (
	// Helvetica is the default body font.
	Helvetica FontName = "Helvetica"
	// HelveticaBold is used for headings and key labels.
	HelveticaBold FontName = "Helvetica-Bold"
	// TimesRoman is available for alternate body text.
	TimesRoman FontName = "Times-Roman"
	// Courier is the monospace font for IDs/hashes.
	Courier FontName = "Courier"
)

// coreFonts maps each supported font to its per-rune width table.
var coreFonts = map[FontName]map[rune]int{
	Helvetica:     helveticaWidths,
	HelveticaBold: helveticaBoldWidths,
	TimesRoman:    timesRomanWidths,
	Courier:       courierWidths,
}

// defaultWidth is used for any rune absent from a width table (e.g. non-ASCII).
const defaultWidth = 500

// RuneWidth returns the advance width (in 1/1000 em) of r in font f.
func RuneWidth(f FontName, r rune) int {
	tbl, ok := coreFonts[f]
	if !ok {
		tbl = helveticaWidths
	}
	if w, ok := tbl[r]; ok {
		return w
	}
	return defaultWidth
}

// TextWidth returns the width of s at the given font size (in points).
func TextWidth(f FontName, size float64, s string) float64 {
	var units int
	for _, r := range s {
		units += RuneWidth(f, r)
	}
	return float64(units) / 1000.0 * size
}

// Courier is monospace: every glyph is 600 units.
var courierWidths = func() map[rune]int {
	m := make(map[rune]int, 95)
	for r := rune(32); r <= 126; r++ {
		m[r] = 600
	}
	return m
}()

// helveticaWidths holds the Adobe AFM widths for Helvetica, ASCII 32..126.
var helveticaWidths = map[rune]int{
	' ': 278, '!': 278, '"': 355, '#': 556, '$': 556, '%': 889, '&': 667, '\'': 191,
	'(': 333, ')': 333, '*': 389, '+': 584, ',': 278, '-': 333, '.': 278, '/': 278,
	'0': 556, '1': 556, '2': 556, '3': 556, '4': 556, '5': 556, '6': 556, '7': 556,
	'8': 556, '9': 556, ':': 278, ';': 278, '<': 584, '=': 584, '>': 584, '?': 556,
	'@': 1015, 'A': 667, 'B': 667, 'C': 722, 'D': 722, 'E': 667, 'F': 611, 'G': 778,
	'H': 722, 'I': 278, 'J': 500, 'K': 667, 'L': 556, 'M': 833, 'N': 722, 'O': 778,
	'P': 667, 'Q': 778, 'R': 722, 'S': 667, 'T': 611, 'U': 722, 'V': 667, 'W': 944,
	'X': 667, 'Y': 667, 'Z': 611, '[': 278, '\\': 278, ']': 278, '^': 469, '_': 556,
	'`': 333, 'a': 556, 'b': 556, 'c': 500, 'd': 556, 'e': 556, 'f': 278, 'g': 556,
	'h': 556, 'i': 222, 'j': 222, 'k': 500, 'l': 222, 'm': 833, 'n': 556, 'o': 556,
	'p': 556, 'q': 556, 'r': 333, 's': 500, 't': 278, 'u': 556, 'v': 500, 'w': 722,
	'x': 500, 'y': 500, 'z': 500, '{': 334, '|': 260, '}': 334, '~': 584,
}

// helveticaBoldWidths holds the Adobe AFM widths for Helvetica-Bold, 32..126.
var helveticaBoldWidths = map[rune]int{
	' ': 278, '!': 333, '"': 474, '#': 556, '$': 556, '%': 889, '&': 722, '\'': 238,
	'(': 333, ')': 333, '*': 389, '+': 584, ',': 278, '-': 333, '.': 278, '/': 278,
	'0': 556, '1': 556, '2': 556, '3': 556, '4': 556, '5': 556, '6': 556, '7': 556,
	'8': 556, '9': 556, ':': 333, ';': 333, '<': 584, '=': 584, '>': 584, '?': 611,
	'@': 975, 'A': 722, 'B': 722, 'C': 722, 'D': 722, 'E': 667, 'F': 611, 'G': 778,
	'H': 722, 'I': 278, 'J': 556, 'K': 722, 'L': 611, 'M': 833, 'N': 722, 'O': 778,
	'P': 667, 'Q': 778, 'R': 722, 'S': 667, 'T': 611, 'U': 722, 'V': 667, 'W': 944,
	'X': 667, 'Y': 667, 'Z': 611, '[': 333, '\\': 278, ']': 333, '^': 584, '_': 556,
	'`': 333, 'a': 556, 'b': 611, 'c': 556, 'd': 611, 'e': 556, 'f': 333, 'g': 611,
	'h': 611, 'i': 278, 'j': 278, 'k': 556, 'l': 278, 'm': 889, 'n': 611, 'o': 611,
	'p': 611, 'q': 611, 'r': 389, 's': 556, 't': 333, 'u': 611, 'v': 556, 'w': 778,
	'x': 556, 'y': 556, 'z': 500, '{': 389, '|': 280, '}': 389, '~': 584,
}

// timesRomanWidths holds the Adobe AFM widths for Times-Roman, 32..126.
var timesRomanWidths = map[rune]int{
	' ': 250, '!': 333, '"': 408, '#': 500, '$': 500, '%': 833, '&': 778, '\'': 180,
	'(': 333, ')': 333, '*': 500, '+': 564, ',': 250, '-': 333, '.': 250, '/': 278,
	'0': 500, '1': 500, '2': 500, '3': 500, '4': 500, '5': 500, '6': 500, '7': 500,
	'8': 500, '9': 500, ':': 278, ';': 278, '<': 564, '=': 564, '>': 564, '?': 444,
	'@': 921, 'A': 722, 'B': 667, 'C': 667, 'D': 722, 'E': 611, 'F': 556, 'G': 722,
	'H': 722, 'I': 333, 'J': 389, 'K': 722, 'L': 611, 'M': 889, 'N': 722, 'O': 722,
	'P': 556, 'Q': 722, 'R': 667, 'S': 556, 'T': 611, 'U': 722, 'V': 722, 'W': 944,
	'X': 722, 'Y': 722, 'Z': 611, '[': 333, '\\': 278, ']': 333, '^': 469, '_': 500,
	'`': 333, 'a': 444, 'b': 500, 'c': 444, 'd': 500, 'e': 444, 'f': 333, 'g': 500,
	'h': 500, 'i': 278, 'j': 278, 'k': 500, 'l': 278, 'm': 778, 'n': 500, 'o': 500,
	'p': 500, 'q': 500, 'r': 333, 's': 389, 't': 278, 'u': 500, 'v': 500, 'w': 722,
	'x': 500, 'y': 500, 'z': 444, '{': 480, '|': 200, '}': 480, '~': 541,
}
