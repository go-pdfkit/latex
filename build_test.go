package latex

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// A face is a font a test page draws in: the name that says what it is, and a
// map from the codes the content stream uses to the characters they stand for,
// which is how a real math font tells a reader that code 11 is alpha.
type face struct {
	base string
	uni  map[byte]rune
}

// pageWith builds a one-page document from a content stream and the fonts it
// names. The page is 200 by 200 unless the test says otherwise.
func pageWith(t *testing.T, content string, faces map[string]face) *reader.Document {
	t.Helper()
	return pagesWith(t, []string{content}, faces, [4]int{0, 0, 200, 200})
}

// pagesWith builds a document of several pages, all with the same fonts and box.
func pagesWith(t *testing.T, contents []string, faces map[string]face, box [4]int) *reader.Document {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	res := reader.Dict{}
	if len(faces) > 0 {
		fonts := reader.Dict{}
		for name, f := range faces {
			fonts[reader.Name(name)] = w.Add(fontDict(w, f))
		}
		res["Font"] = fonts
	}
	kids := reader.Array{}
	for _, c := range contents {
		kids = append(kids, w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef,
			"MediaBox": reader.Array{reader.Integer(box[0]), reader.Integer(box[1]),
				reader.Integer(box[2]), reader.Integer(box[3])},
			"Contents":  w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(c)}),
			"Resources": res,
		}))
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": kids, "Count": reader.Integer(len(kids))})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// fontDict is a simple font of even width, so that a test can work out where
// every character lands: half an em each.
func fontDict(w *reader.Writer, f face) reader.Dict {
	widths := make(reader.Array, 0, 224)
	for i := 32; i < 256; i++ {
		widths = append(widths, reader.Integer(500))
	}
	d := reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
		"BaseFont": reader.Name(f.base), "FirstChar": reader.Integer(32),
		"LastChar": reader.Integer(255), "Widths": widths,
		"Encoding": reader.Name("WinAnsiEncoding"),
	}
	if f.uni != nil {
		d["ToUnicode"] = w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(toUnicode(f.uni))})
	}
	return d
}

// toUnicode writes the CMap that says what each code stands for.
func toUnicode(m map[byte]rune) string {
	var b strings.Builder
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	b.WriteString("1 begincodespacerange\n<00> <FF>\nendcodespacerange\n")
	fmt.Fprintf(&b, "%d beginbfchar\n", len(m))
	for code, r := range m {
		fmt.Fprintf(&b, "<%02X> <%04X>\n", code, r)
	}
	b.WriteString("endbfchar\nendcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return b.String()
}

// show is one piece of text drawn at a place, in a font, at a size.
func show(font string, size float64, x, y float64, text string) string {
	// A PDF string literal escapes its backslashes and its parentheses.
	lit := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`).Replace(text)
	return fmt.Sprintf("BT /%s %g Tf 1 0 0 1 %g %g Tm (%s) Tj ET\n", font, size, x, y, lit)
}

// bar is a filled rectangle, which is how pdfTeX draws a rule.
func bar(x, y, w, h float64) string {
	return fmt.Sprintf("%g %g %g %g re f\n", x, y, w, h)
}
