package latex

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// at is one piece of text for the tests that work below the level of a page.
func at(text string, x, y, w, size float64, s shape) atom {
	return atom{text: text, x: x, y: y, width: w, size: size, space: size / 3, sh: s}
}

func TestHowWideAWordSpaceIs(t *testing.T) {
	// What the font says, when what it says is believable.
	a := atom{size: 10, space: 3}
	if a.wordSpace() != 3 {
		t.Errorf("got %v", a.wordSpace())
	}
	// A composite font has no character at 32 and answers with whatever its
	// fallback width is; one real producer answers with the font size, and
	// taking that at face value turns a page into one long word.
	a.space = 10
	if abs(a.wordSpace()-3.3) > 1e-9 {
		t.Errorf("got %v", a.wordSpace())
	}
	a.space = 0.5
	if abs(a.wordSpace()-3.3) > 1e-9 {
		t.Errorf("got %v", a.wordSpace())
	}
	// A math font has no word spaces at all.
	a = atom{size: 10, space: 3, sh: shape{math: mathLetter}}
	if a.wordSpace() != 0 {
		t.Errorf("got %v", a.wordSpace())
	}
}

func TestReadingAPageThatIsNotThere(t *testing.T) {
	d := pageWith(t, "", nil)
	if _, err := readPage(d, 4); err == nil {
		t.Error("page four of a one-page document read without error")
	}
	if _, err := Reconstruct(d, Options{First: 4, Last: 4}); err == nil {
		t.Error("reconstructing a page that is not there succeeded")
	}
	if _, err := Source(d); err != nil {
		t.Error(err)
	}
}

func TestAPageBoxThatCannotBeRead(t *testing.T) {
	d := pageWith(t, "", nil)
	// Not an array, too short, not numbers: each falls back to the default.
	for _, o := range []reader.Object{
		reader.Name("x"),
		reader.Array{reader.Integer(0)},
		reader.Array{reader.Integer(0), reader.Integer(0), reader.Name("x"), reader.Integer(1)},
	} {
		if _, ok := rectangle(d, o); ok {
			t.Errorf("%v read as a rectangle", o)
		}
	}
	// A box written back to front is put the right way round.
	got, ok := rectangle(d, reader.Array{reader.Integer(9), reader.Integer(9),
		reader.Integer(1), reader.Integer(1)})
	if !ok || got != [4]float64{1, 1, 9, 9} {
		t.Errorf("got %v %v", got, ok)
	}
	// A page with no box at all is taken to be letter paper.
	if got := visibleBox(d, reader.Dict{}); got != [4]float64{0, 0, 612, 792} {
		t.Errorf("got %v", got)
	}
}

func TestAFontWithNoName(t *testing.T) {
	d := pageWith(t, "", nil)
	if got := fontName(d, reader.Dict{}); got != "" {
		t.Errorf("got %q", got)
	}
	if got := fontName(d, reader.Dict{"BaseFont": reader.Integer(3)}); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestFontsInsideAForm(t *testing.T) {
	// A form XObject carries resources of its own, and the fonts it names
	// have to be read too or the text it draws comes back unclassified.
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	inner := w.Add(reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Form"),
		"Resources": reader.Dict{"Font": reader.Dict{
			"G": w.Add(fontDict(w, face{base: "CMBX10"})),
			// A font entry that is not a dictionary is skipped.
			"H": reader.Integer(7),
		}},
	})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(200), reader.Integer(200)},
		"Contents": w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte("")}),
		"Resources": reader.Dict{
			"Font":    reader.Dict{"F": w.Add(fontDict(w, face{base: "CMR10"}))},
			"XObject": reader.Dict{"X": inner, "Y": reader.Integer(9)},
		},
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	dict, err := d.Page(1)
	if err != nil {
		t.Fatal(err)
	}
	got := fontShapes(d, dict)
	if !got["G"].bold || got["F"].bold {
		t.Errorf("got %+v", got)
	}
	// A dictionary with no resources contributes nothing, and neither does
	// one nested deeper than a page ever nests.
	empty := map[string]shape{}
	collectFonts(d, reader.Dict{}, empty, 0)
	collectFonts(d, dict, empty, maxResourceDepth+1)
	if len(empty) != 0 {
		t.Errorf("got %+v", empty)
	}
}

func TestWhichBandsALineWillTake(t *testing.T) {
	body := line{y: 100, size: 10, x0: 20, x1: 60}
	// A band far to the side belongs to another column, not to this line.
	far := measured([]atom{at("x", 300, 104, 4, 7, shape{})})
	if accepts(body, far, nil) {
		t.Error("a band in the next column was taken")
	}
	// A band larger than the line is never offered to it.
	big := measured([]atom{at("x", 25, 104, 4, 20, shape{})})
	if accepts(body, big, nil) {
		t.Error("a larger band was taken")
	}
	// A band the same size, level with the line, and nowhere near it
	// vertically, is the next line.
	next := measured([]atom{at("x", 25, 80, 4, 10, shape{})})
	if accepts(body, next, nil) {
		t.Error("the next line was taken")
	}
	// A symbol raised above the line belongs to it; the same symbol dropped
	// below it belongs to the line beneath.
	up := measured([]atom{at("√", 25, 106, 4, 10, shape{math: mathSymbol})})
	down := measured([]atom{at("√", 25, 94, 4, 10, shape{math: mathSymbol})})
	if !accepts(body, up, nil) || accepts(body, down, nil) {
		t.Error("a raised symbol was read wrongly")
	}
}

func TestLimitsOfABigOperator(t *testing.T) {
	op := at("∑", 20, 100, 10, 10, shape{math: mathExt})
	l := line{atoms: []atom{op, at("f", 34, 100, 5, 10, shape{math: mathLetter})},
		y: 100, size: 10, x0: 20, x1: 39}
	// A limit sits over the operator and may be further off than a script.
	limit := measured([]atom{at("n", 21, 111, 4, 7, shape{math: mathLetter})})
	if !accepts(l, limit, nil) {
		t.Error("the limit was not taken")
	}
	// A script the same distance away but not over an operator is not.
	plain := line{atoms: []atom{at("f", 34, 100, 5, 10, shape{})}, y: 100, size: 10, x0: 34, x1: 39}
	if accepts(plain, measured([]atom{at("n", 34, 111, 4, 7, shape{})}), nil) {
		t.Error("a band eleven points away was taken as a script")
	}
	if overOperator(plain, limit) {
		t.Error("a line with no big operator has something over one")
	}
}

func TestWhenARuleBindsTwoBands(t *testing.T) {
	l := line{atoms: []atom{at("d", 20, 100, 5, 10, shape{math: mathLetter})},
		y: 100, size: 10, x0: 20, x1: 25}
	below := measured([]atom{at("dt", 20, 92, 9, 10, shape{math: mathLetter})})
	frac := rule{19, 97.5, 29, 98}
	if !accepts(l, below, []rule{frac}) {
		t.Error("the denominator was not taken")
	}
	// A rule far too wide for what sits under it is a table's line.
	if accepts(l, below, []rule{{19, 97.5, 300, 98}}) {
		t.Error("a table rule was taken as a fraction bar")
	}
	// A band that reaches outside the rule is not one of its halves.
	wideBand := measured([]atom{at("dt", 20, 92, 40, 10, shape{math: mathLetter})})
	if accepts(l, wideBand, []rule{frac}) {
		t.Error("a band wider than the bar was taken")
	}
	// A band too far below to be part of the same fraction.
	away := measured([]atom{at("dt", 20, 60, 9, 10, shape{math: mathLetter})})
	if accepts(l, away, []rule{frac}) {
		t.Error("a band thirty points away was taken")
	}
	// An upright rule is not a fraction bar.
	if accepts(l, below, []rule{{22, 90, 22.5, 110}}) {
		t.Error("a column divider was taken as a fraction bar")
	}
	// A rule outside the two baselines is not between them.
	if accepts(l, below, []rule{{19, 130, 29, 130.5}}) {
		t.Error("a rule above both was taken as a bar between them")
	}
}

func TestPageNumbersAndRunningHeads(t *testing.T) {
	mk := func(y, x1 float64) line {
		return line{y: y, size: 10, x0: 20, x1: x1, left: 20, right: 200}
	}
	lines := []line{mk(190, 30), mk(160, 200), mk(148, 200), mk(20, 30)}
	got := stripRunningHeads(lines, 10)
	if len(got) != 2 || got[0].y != 160 {
		t.Errorf("got %+v", got)
	}
	// A heading at the top of a page is short and set off from what follows
	// it too, and must not be thrown away with the running heads.
	head := line{y: 190, size: 14, x0: 20, x1: 60, left: 20, right: 200}
	if got := stripRunningHeads([]line{head, mk(160, 200)}, 10); len(got) != 2 {
		t.Errorf("the heading was stripped: %+v", got)
	}
	// A single line is left alone.
	if got := stripRunningHeads(lines[:1], 10); len(got) != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestFindingTheGutter(t *testing.T) {
	if _, ok := gutter(nil, 100); ok {
		t.Error("a page with nothing on it has a gutter")
	}
	if _, ok := gutter([]atom{at("x", 0, 0, 5, 10, shape{})}, 0); ok {
		t.Error("a page with no width has a gutter")
	}
	// Text on one side only is one column.
	if _, ok := gutter([]atom{at("x", 0, 0, 5, 10, shape{})}, 100); ok {
		t.Error("text on the left alone made a gutter")
	}
	if _, ok := gutter([]atom{at("x", 90, 0, 5, 10, shape{})}, 100); ok {
		t.Error("text on the right alone made a gutter")
	}
	// Text on both sides with nothing between them is two columns.
	both := []atom{at("x", 0, 0, 40, 10, shape{}), at("y", 60, 0, 40, 10, shape{})}
	x, ok := gutter(both, 100)
	if !ok || x < 40 || x > 60 {
		t.Errorf("got %v %v", x, ok)
	}
	// Text right across the middle is one column.
	if _, ok := gutter([]atom{at("x", 0, 0, 100, 10, shape{})}, 100); ok {
		t.Error("a full-width line made a gutter")
	}
}

func TestSplittingABandAtTheGutter(t *testing.T) {
	one := measured([]atom{at("x", 0, 0, 5, 10, shape{})})
	if got := split([]band{one}, 50); len(got) != 1 {
		t.Errorf("a band on one side was split: %+v", got)
	}
}

func TestWritingAnEquationsPieces(t *testing.T) {
	// A control word runs into a letter, so a space goes between them; it
	// does not run into a brace or a digit.
	if got := join([]string{`\alpha`, "x"}); got != `\alpha x` {
		t.Errorf("got %q", got)
	}
	if got := join([]string{`\alpha`, "1"}); got != `\alpha1` {
		t.Errorf("got %q", got)
	}
	if got := join([]string{"x", "y"}); got != "xy" {
		t.Errorf("got %q", got)
	}
	if got := join([]string{"", "x", ""}); got != "x" {
		t.Errorf("got %q", got)
	}
	if needsGap("", "x") || needsGap("x", "") {
		t.Error("an empty piece needs a gap")
	}
	if endsInControlWord("abc") || !endsInControlWord(`\alpha`) {
		t.Error("endsInControlWord is wrong")
	}
}

func TestWhichCharactersAnEquationWritesPlainly(t *testing.T) {
	for _, r := range "0123456789+=<>()[]/!:;,.|'-*?@" {
		if !mathPlain(r) {
			t.Errorf("%q is not plain", r)
		}
	}
	for _, r := range "$%#&" {
		if mathPlain(r) {
			t.Errorf("%q is plain", r)
		}
	}
}

func TestWhatCountsAsMathematics(t *testing.T) {
	if !mathLike(at("2", 0, 0, 5, 10, shape{})) {
		t.Error("a digit is not math-like")
	}
	if !mathLike(at("x", 0, 0, 5, 10, shape{})) {
		t.Error("a single letter is not math-like")
	}
	if !mathLike(at("sin", 0, 0, 5, 10, shape{})) {
		t.Error("an operator name is not math-like")
	}
	if mathLike(at("and", 0, 0, 5, 10, shape{})) {
		t.Error("a word is math-like")
	}
	if mathLike(at("and?x", 0, 0, 5, 10, shape{})) {
		t.Error("a word with punctuation in it is math-like")
	}
	if !mathLike(at("+ =", 0, 0, 5, 10, shape{})) {
		t.Error("arithmetic with a space in it is not math-like")
	}
}

func TestASingleItalicLetterAmongRoman(t *testing.T) {
	rom, it := shape{}, shape{italic: true}
	atoms := []atom{
		at("(", 0, 0, 5, 10, rom),
		at("L", 5, 0, 5, 10, it),
		at(",", 10, 0, 5, 10, rom),
		at("wide", 15, 0, 20, 10, it),
		at("x", 35, 0, 5, 10, it),
	}
	if !italicVariable(atoms, 1) {
		t.Error("a lone italic letter is not a variable")
	}
	if italicVariable(atoms, 0) {
		t.Error("a roman character is a variable")
	}
	// A word is emphasis, not a variable.
	if italicVariable(atoms, 3) {
		t.Error("an italic word is a variable")
	}
	// A letter with italic beside it is part of italic text.
	if italicVariable(atoms, 4) {
		t.Error("a letter next to italic text is a variable")
	}
	// A digit is not a variable, and neither is anything set in a math font
	// (which is a seed already).
	more := []atom{at("7", 0, 0, 5, 10, it), at("x", 0, 0, 5, 10, shape{math: mathLetter})}
	if italicVariable(more, 0) || italicVariable(more, 1) {
		t.Error("got a variable")
	}
}

func TestSplittingASectionNumberOff(t *testing.T) {
	for _, c := range []struct {
		in    string
		parts int
		rest  string
	}{
		{"1 One", 1, "One"},
		{"1.2 Two", 2, "Two"},
		{"1.2.3 Three", 3, "Three"},
		{"A.1 Appendix", 2, "Appendix"},
		{"Introduction", 0, "Introduction"},
		{"1.", 0, "1."},
		{"1..2 Two", 0, "1..2 Two"},
		{"", 0, ""},
	} {
		parts, rest := splitNumber(c.in)
		if len(parts) != c.parts || rest != c.rest {
			t.Errorf("%q gave %v %q", c.in, parts, rest)
		}
	}
}

func TestWhichCommandAHeadingLevelUses(t *testing.T) {
	if sectionName(1) != `\section` || sectionName(4) != `\paragraph` || sectionName(9) != `\paragraph` {
		t.Error("sectionName is wrong")
	}
	e := &emitter{levels: []float64{14, 12}}
	if e.rank(14) != 1 || e.rank(12) != 2 || e.rank(9) != 2 {
		t.Errorf("rank gave %d %d %d", e.rank(14), e.rank(12), e.rank(9))
	}
}

func TestTakingAnEquationNumberOff(t *testing.T) {
	mathSeg := segment{atoms: []atom{at("x", 0, 0, 5, 10, shape{math: mathLetter})}, math: true}
	num := segment{atoms: []atom{at("(1)", 90, 0, 15, 10, shape{})}}
	if got := trimNumber([]segment{mathSeg, num}); len(got) != 1 {
		t.Error("the number was kept")
	}
	if got := trimNumber([]segment{mathSeg}); len(got) != 1 {
		t.Error("a lone equation lost something")
	}
	if got := trimNumber([]segment{mathSeg, mathSeg}); len(got) != 2 {
		t.Error("an equation was taken for a number")
	}
	two := segment{atoms: []atom{at("(", 0, 0, 5, 10, shape{}), at("1)", 5, 0, 5, 10, shape{})}}
	if got := trimNumber([]segment{mathSeg, two}); len(got) != 2 {
		t.Error("a two-piece tail was taken for a number")
	}
	word := segment{atoms: []atom{at("(x", 90, 0, 15, 10, shape{})}}
	if got := trimNumber([]segment{mathSeg, word}); len(got) != 2 {
		t.Error("a bracket with no closing one was taken for a number")
	}
	if got := trimNumber([]segment{mathSeg, {atoms: []atom{at("()", 0, 0, 5, 10, shape{})}}}); len(got) != 2 {
		t.Error("an empty pair of brackets was taken for a number")
	}
}

func TestTheSmallJudgementsOfALine(t *testing.T) {
	if allBold(line{}) {
		t.Error("a line with nothing on it is bold")
	}
	if allMath(nil) {
		t.Error("a line with nothing on it is mathematics")
	}
	if centred(line{left: 10, right: 10}) {
		t.Error("a column with no width has a centre")
	}
	if startsLower("") || startsLower("A") || !startsLower("a") {
		t.Error("startsLower is wrong")
	}
	if brokenWord("a--") || !brokenWord("a-") {
		t.Error("brokenWord is wrong")
	}
	// Two pieces with no idea how wide a space is have no space between
	// them: both are mathematics.
	m := shape{math: mathLetter}
	if gapBetween(segment{atoms: []atom{at("x", 0, 0, 5, 10, m)}},
		segment{atoms: []atom{at("y", 50, 0, 5, 10, m)}}) {
		t.Error("two math pieces were given a word space")
	}
}

func TestAnEmptyParagraphIsNotWritten(t *testing.T) {
	e := &emitter{}
	e.paragraph("   ")
	e.flush()
	if e.out.String() != "" {
		t.Errorf("got %q", e.out.String())
	}
}

func TestALineWithNothingOnIt(t *testing.T) {
	e := &emitter{body: 10}
	e.line(line{}, nil)
	if e.out.String() != "" || len(e.para) != 0 {
		t.Error("an empty line wrote something")
	}
	if segments(line{}) != nil {
		t.Error("an empty line has segments")
	}
}

// cyclic builds a document with an indirect reference that points at itself
// through another, which is the one thing that makes resolving one fail.
func cyclic(t *testing.T) (*reader.Document, reader.Ref) {
	t.Helper()
	w := reader.NewWriter("1.7")
	a, b := w.Reserve(), w.Reserve()
	w.Put(a, b)
	w.Put(b, a)
	pagesRef := w.Reserve()
	page := w.Add(reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef, "MediaBox": a})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1)})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	d, err := reader.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	return d, a
}

func TestAReferenceThatCannotBeFollowed(t *testing.T) {
	d, ref := cyclic(t)
	if _, ok := rectangle(d, ref); ok {
		t.Error("a reference chain that never ends read as a rectangle")
	}
	if _, ok := rectangle(d, reader.Array{ref, reader.Integer(0), reader.Integer(1), reader.Integer(1)}); ok {
		t.Error("a rectangle with an endless element read")
	}
	if got := fontName(d, reader.Dict{"BaseFont": ref}); got != "" {
		t.Errorf("got %q", got)
	}
	// The page falls back to letter paper, and reconstructs as nothing.
	doc, err := Reconstruct(d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.Preamble, "paperwidth=612.0pt") {
		t.Errorf("got %s", doc.Preamble)
	}
}

func TestSourceOfADocumentWithNoSuchPage(t *testing.T) {
	d := pageWith(t, "", nil)
	if _, err := Reconstruct(d, Options{First: 9, Last: 9}); err == nil {
		t.Error("no error")
	}
}

func TestAnEquationReachingLeftwards(t *testing.T) {
	// The growth goes both ways: the 2 of 2x is roman, and belongs to the
	// equation the x seeds.
	l := line{atoms: []atom{
		at("2", 20, 100, 5, 10, shape{}),
		at("x", 25, 100, 5, 10, mi),
	}, y: 100, size: 10}
	segs := segments(l)
	if len(segs) != 1 || !segs[0].math {
		t.Errorf("got %+v", segs)
	}
}

func TestACharacterThatIsNeitherLetterNorArithmetic(t *testing.T) {
	if mathLike(at("#", 0, 0, 5, 10, shape{})) {
		t.Error("a hash is math-like")
	}
}

func TestAnItalicLetterWithItalicAfterIt(t *testing.T) {
	atoms := []atom{
		at("x", 0, 0, 5, 10, shape{italic: true}),
		at("word", 5, 0, 20, 10, shape{italic: true}),
	}
	if italicVariable(atoms, 0) {
		t.Error("a letter with italic after it is a variable")
	}
}

func TestAFullWidthLineOnATwoColumnPage(t *testing.T) {
	mk := func(x0, x1, y float64) line { return line{x0: x0, x1: x1, y: y, size: 10} }
	var lines []line
	lines = append(lines, mk(10, 190, 300)) // a heading across both columns
	for i := 0; i < 6; i++ {
		y := float64(280 - i*12)
		lines = append(lines, mk(10, 90, y), mk(110, 190, y))
	}
	got, two := order(lines, 200)
	if !two {
		t.Fatal("the page was not read as two columns")
	}
	if got[0].x0 != 10 || got[0].x1 != 190 {
		t.Errorf("the full-width line did not come first: %+v", got[0])
	}
	// The full-width line is measured against the page, the others against
	// their own column.
	if got[0].right != 190 || got[1].right != 90 {
		t.Errorf("margins are %v and %v", got[0].right, got[1].right)
	}
	// Every left line comes before every right one.
	for i := 1; i < 7; i++ {
		if got[i].x0 != 10 {
			t.Errorf("line %d is at %v", i, got[i].x0)
		}
	}
}

func TestParagraphsFromASpaceAndFromAShortLine(t *testing.T) {
	e := &emitter{body: 10, justified: true, hasPrev: true, para: []string{"x"}}
	e.prev = line{y: 200, x0: 20, x1: 200, left: 20, right: 200}
	e.paraRight = 200
	// A gap much wider than the leading ends a paragraph.
	if !e.breaks(line{y: 178, x0: 20, left: 20, right: 200}) {
		t.Error("a wide gap did not end the paragraph")
	}
	// So does a line that stopped well short of the right margin.
	e.prev.x1 = 150
	if !e.breaks(line{y: 188, x0: 20, left: 20, right: 200}) {
		t.Error("a short line did not end the paragraph")
	}
	// A centred block is neither indented nor short in any useful sense.
	e.centred = true
	if e.breaks(line{y: 188, x0: 60, left: 20, right: 200}) {
		t.Error("a centred line ended the paragraph")
	}
}
