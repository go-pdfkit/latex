package latex

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// built is a one-page document assembled by hand, for the tests that need more
// of a page than a content stream and a font.
type built struct {
	w        *reader.Writer
	res      reader.Dict
	content  string
	box      [4]int
	contents reader.Object
}

func newBuilt() *built {
	return &built{w: reader.NewWriter("1.7"), res: reader.Dict{}, box: [4]int{0, 0, 200, 200}}
}

func (b *built) open(t *testing.T) *reader.Document {
	t.Helper()
	pagesRef := b.w.Reserve()
	contents := b.contents
	if contents == nil {
		contents = b.w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(b.content)})
	}
	page := b.w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": reader.Array{reader.Integer(b.box[0]), reader.Integer(b.box[1]),
			reader.Integer(b.box[2]), reader.Integer(b.box[3])},
		"Contents": contents, "Resources": b.res,
	})
	b.w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := b.w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := b.w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAPictureOnThePageBecomesAFigure(t *testing.T) {
	b := newBuilt()
	img := b.w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Image"),
		"Width": reader.Integer(2), "Height": reader.Integer(2),
		"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("DeviceGray"),
	}, Raw: []byte{0, 64, 128, 255}})
	tiny := b.w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Image"),
		"Width": reader.Integer(1), "Height": reader.Integer(1),
		"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("DeviceGray"),
	}, Raw: []byte{0}})
	b.res["XObject"] = reader.Dict{"Im0": img, "Im1": tiny}
	b.res["Font"] = reader.Dict{"R": b.w.Add(fontDict(b.w, roman))}
	b.content = show("R", 10, 20, 180, "Text above the picture on this page.") +
		"q 100 0 0 60 20 80 cm /Im0 Do Q\n" +
		"q 4 0 0 4 20 40 cm /Im1 Do Q\n" +
		show("R", 10, 20, 20, "Text below the picture on this page.")
	doc, err := Reconstruct(b.open(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Files) != 1 || !strings.Contains(doc.Body, `\includegraphics`) {
		t.Fatalf("got %d files:\n%s", len(doc.Files), doc.Body)
	}
	if !strings.Contains(doc.Body, "width=100.0pt,height=60.0pt") {
		t.Errorf("got %s", doc.Body)
	}
	// The four-point picture is a rule or a bullet, not a figure.
	if strings.Count(doc.Body, `\includegraphics`) != 1 {
		t.Errorf("got %s", doc.Body)
	}
}

func TestATitleAndItsMaketitle(t *testing.T) {
	b := newBuilt()
	b.box = [4]int{0, 0, 400, 300}
	b.res["Font"] = reader.Dict{
		"R": b.w.Add(fontDict(b.w, roman)),
		"H": b.w.Add(fontDict(b.w, face{base: "CMBX17"})),
	}
	b.content = show("H", 17, 113, 280, "A Title") +
		show("R", 10, 20, 240, "Body text that runs right across the page here.") +
		show("R", 10, 20, 228, "More body text that runs across the page as well.") +
		show("R", 10, 20, 216, "Still more body text to weigh the body size down.")
	doc, err := Reconstruct(b.open(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Preamble, `\title{A Title}`) {
		t.Errorf("preamble is\n%s", doc.Preamble)
	}
	if !strings.HasPrefix(doc.Body, `\maketitle`) || strings.Contains(doc.Body, "A Title") {
		t.Errorf("body is\n%s", doc.Body)
	}
}

func TestTheSameFontNameInAPageAndInAForm(t *testing.T) {
	// The page's own font wins, which is the wrong answer only for a
	// document that gives one name to two different faces.
	b := newBuilt()
	form := b.w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Form"),
		"Resources": reader.Dict{"Font": reader.Dict{"R": b.w.Add(fontDict(b.w, bold))}},
	}, Raw: []byte("")})
	b.res["Font"] = reader.Dict{"R": b.w.Add(fontDict(b.w, roman))}
	b.res["XObject"] = reader.Dict{"F": form}
	b.content = show("R", 10, 20, 150, "plain")
	if got, err := Reconstruct(b.open(t), Options{}); err != nil {
		t.Fatal(err)
	} else if strings.Contains(got.Body, `\textbf`) {
		t.Errorf("the form's font won: %s", got.Body)
	}
}

func TestARunWhoseCharactersCannotBeWorkedOut(t *testing.T) {
	// A symbolic font with no map and glyphs named after numbers says
	// nothing about what its codes stand for. extract reports the run as
	// unreadable with no text in it, and there is nothing to write.
	b := newBuilt()
	b.res["Font"] = reader.Dict{"N": b.w.Add(reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
		"BaseFont": reader.Name("Odd"), "FirstChar": reader.Integer(65),
		"LastChar": reader.Integer(66), "Widths": reader.Array{reader.Integer(500), reader.Integer(500)},
		"Encoding": reader.Dict{"Differences": reader.Array{reader.Integer(65),
			reader.Name("g12"), reader.Name("g13")}},
	})}
	b.content = show("N", 10, 20, 150, "AB")
	doc, err := Reconstruct(b.open(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(doc.Body) != "" {
		t.Errorf("got %q", doc.Body)
	}
}

func TestAPageWhoseContentCannotBeRead(t *testing.T) {
	b := newBuilt()
	b.contents = b.w.Add(&reader.Stream{
		Dict: reader.Dict{"Filter": reader.Name("FlateDecode")},
		Raw:  []byte("not compressed at all"),
	})
	if _, err := Source(b.open(t)); err == nil {
		t.Error("a page whose content will not decode read without error")
	}
}
