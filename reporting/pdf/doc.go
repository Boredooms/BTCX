// Package pdf renders a report view to a real, pure-Go PDF-1.4 document. It is
// NETWORK-FORBIDDEN and depends on NO third-party PDF library, no browser and
// no external renderer: the object model, cross-reference table and trailer are
// written by hand. Output is deterministic for a given view (the /ID is the
// snapshot hash, never a timestamp; no date is written inside any content
// stream).
package pdf

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// page is one laid-out page: a content stream of drawing operators.
type page struct {
	content bytes.Buffer
}

// document is the in-memory PDF model. The object order is FIXED so byte output
// is reproducible:
//
//	1  Catalog
//	2  Pages
//	3  Font Helvetica
//	4  Font Helvetica-Bold
//	5  Font Courier
//	6.. page objects then their content-stream objects, interleaved
//	    (page[0], content[0], page[1], content[1], ...)
type document struct {
	pages    []*page
	fontObjs []fontObj
}

type fontObj struct {
	name     FontName
	baseFont string
	resName  string // e.g. "F1"
}

// newDocument creates a document with the fixed standard-14 font set.
func newDocument() *document {
	return &document{
		fontObjs: []fontObj{
			{name: Helvetica, baseFont: "Helvetica", resName: "F1"},
			{name: HelveticaBold, baseFont: "Helvetica-Bold", resName: "F2"},
			{name: Courier, baseFont: "Courier", resName: "F3"},
		},
	}
}

// addPage appends a new empty page and returns it.
func (d *document) addPage() *page {
	p := &page{}
	d.pages = append(d.pages, p)
	return p
}

// resourceName returns the PDF resource name (e.g. "F1") for a font.
func (d *document) resourceName(f FontName) string {
	for _, fo := range d.fontObjs {
		if fo.name == f {
			return fo.resName
		}
	}
	return "F1"
}

const (
	// pageWidthPt / pageHeightPt are A4 at 72 dpi (595.28 x 841.89 pt), rounded
	// to integers for stable output.
	pageWidthPt  = 595
	pageHeightPt = 842
)

// write serializes the document to a byte slice. idHex is the snapshot hash,
// used verbatim as the PDF /ID so two renders of the same snapshot are
// byte-identical and the ID is content-derived (never time-derived).
func (d *document) write(idHex string) ([]byte, error) {
	if len(d.pages) == 0 {
		// A valid PDF needs at least one page; this should not happen because
		// the renderer always starts a page, but guard rather than emit a
		// structurally invalid file.
		d.addPage()
	}

	// Fixed object numbering.
	const (
		objCatalog = 1
		objPages   = 2
	)
	firstFontObj := 3
	firstPageObj := firstFontObj + len(d.fontObjs) // after the font objects

	// Each page uses two objects: the page dict and its content stream.
	// Layout: page0, content0, page1, content1, ...
	pageObjNum := func(i int) int { return firstPageObj + i*2 }
	contentObjNum := func(i int) int { return firstPageObj + i*2 + 1 }

	totalObjs := firstPageObj + len(d.pages)*2 - 1 // highest object number

	var buf bytes.Buffer
	// Byte offsets of each object (index 0 unused; offsets[n] = offset of obj n).
	offsets := make([]int, totalObjs+1)

	buf.WriteString("%PDF-1.4\n")
	// Binary marker comment so tools treat the file as binary.
	buf.WriteString("%\xE2\xE3\xCF\xD3\n")

	writeObj := func(num int, body string) {
		offsets[num] = buf.Len()
		buf.WriteString(strconv.Itoa(num))
		buf.WriteString(" 0 obj\n")
		buf.WriteString(body)
		buf.WriteString("\nendobj\n")
	}

	// 1. Catalog.
	writeObj(objCatalog, "<< /Type /Catalog /Pages "+ref(objPages)+" >>")

	// 2. Pages tree.
	var kids strings.Builder
	kids.WriteByte('[')
	for i := range d.pages {
		if i > 0 {
			kids.WriteByte(' ')
		}
		kids.WriteString(ref(pageObjNum(i)))
	}
	kids.WriteByte(']')
	writeObj(objPages, fmt.Sprintf(
		"<< /Type /Pages /Kids %s /Count %d >>", kids.String(), len(d.pages)))

	// 3.. Fonts.
	for i, fo := range d.fontObjs {
		writeObj(firstFontObj+i, fmt.Sprintf(
			"<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>",
			fo.baseFont))
	}

	// Font resource dictionary shared by every page.
	var fontRes strings.Builder
	fontRes.WriteString("<< ")
	for i, fo := range d.fontObjs {
		fontRes.WriteString("/" + fo.resName + " " + ref(firstFontObj+i) + " ")
	}
	fontRes.WriteString(">>")

	// Pages + content streams.
	for i, p := range d.pages {
		content := p.content.Bytes()
		// Content stream object.
		var cs bytes.Buffer
		cs.WriteString(fmt.Sprintf("<< /Length %d >>\nstream\n", len(content)))
		cs.Write(content)
		cs.WriteString("\nendstream")
		writeObj(contentObjNum(i), cs.String())

		// Page object references its content stream + shared resources.
		pageDict := fmt.Sprintf(
			"<< /Type /Page /Parent %s /MediaBox [0 0 %d %d] "+
				"/Resources << /Font %s >> /Contents %s >>",
			ref(objPages), pageWidthPt, pageHeightPt, fontRes.String(), ref(contentObjNum(i)))
		writeObj(pageObjNum(i), pageDict)
	}

	// Cross-reference table.
	xrefOffset := buf.Len()
	buf.WriteString("xref\n")
	buf.WriteString(fmt.Sprintf("0 %d\n", totalObjs+1))
	// Object 0 is the free list head.
	buf.WriteString("0000000000 65535 f \n")
	for n := 1; n <= totalObjs; n++ {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[n]))
	}

	// Trailer with content-derived /ID (snapshot hash, same value twice).
	id := sanitizeID(idHex)
	buf.WriteString("trailer\n")
	buf.WriteString(fmt.Sprintf(
		"<< /Size %d /Root %s /ID [<%s> <%s>] >>\n",
		totalObjs+1, ref(objCatalog), id, id))
	buf.WriteString("startxref\n")
	buf.WriteString(strconv.Itoa(xrefOffset))
	buf.WriteString("\n%%EOF\n")

	return buf.Bytes(), nil
}

// ref renders an indirect object reference "N 0 R".
func ref(n int) string { return strconv.Itoa(n) + " 0 R" }

// sanitizeID keeps only hex digits from the snapshot hash so the /ID is a valid
// PDF hex string. The snapshot hash is already lowercase hex; this is a
// defensive filter.
func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "00"
	}
	return b.String()
}
