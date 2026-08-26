package latex

import (
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// The fonts a test page draws in. The names are the ones TeX has used since
// 1980, which is what the classifier reads; the maps are what a real math font
// carries so that a reader can tell code 11 from alpha.
var (
	roman  = face{base: "ABCDEF+CMR10"}
	bold   = face{base: "ABCDEF+CMBX10"}
	italic = face{base: "ABCDEF+CMTI10"}
	mono   = face{base: "ABCDEF+CMTT10"}
	sansf  = face{base: "ABCDEF+CMSS10"}
	caps   = face{base: "ABCDEF+CMCSC10"}
	mathit = face{base: "ABCDEF+CMMI10", uni: map[byte]rune{'a': 'α', 'b': 'β'}}
	symbol = face{base: "ABCDEF+CMSY10", uni: map[byte]rune{'r': '√', 'i': '∈', 'm': '−'}}
	extend = face{base: "ABCDEF+CMEX10", uni: map[byte]rune{'s': '∑', '(': '(', ')': ')'}}
)

func allFaces() map[string]face {
	return map[string]face{
		"R": roman, "B": bold, "I": italic, "T": mono, "S": sansf, "C": caps,
		"M": mathit, "Y": symbol, "X": extend,
	}
}

// body is the reconstructed body of a one-page document.
func body(t *testing.T, content string) string {
	t.Helper()
	d := pageWith(t, content, allFaces())
	doc, err := Reconstruct(d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(doc.Body)
}

func TestProseAndItsMarkup(t *testing.T) {
	got := body(t, show("R", 10, 20, 150, "This is")+
		show("B", 10, 60, 150, "bold")+
		show("R", 10, 85, 150, "and")+
		show("I", 10, 105, 150, "slanted")+
		show("R", 10, 145, 150, "and")+
		show("T", 10, 165, 150, "fixed"))
	want := `This is \textbf{bold} and \emph{slanted} and \texttt{fixed}`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestSansAndSmallCaps(t *testing.T) {
	got := body(t, show("R", 10, 20, 150, "a")+
		show("S", 10, 30, 150, "bb")+
		show("R", 10, 45, 150, "c")+
		show("C", 10, 55, 150, "dd"))
	if !strings.Contains(got, `\textsf{bb}`) || !strings.Contains(got, `\textsc{dd}`) {
		t.Errorf("got %s", got)
	}
}

func TestWordsAndKerning(t *testing.T) {
	// Two pieces a space apart are two words; two pieces touching are one.
	got := body(t, show("R", 10, 20, 150, "on")+show("R", 10, 30, 150, "e")+
		show("R", 10, 45, 150, "two"))
	if got != "one two" {
		t.Errorf("got %q", got)
	}
}

func TestSuperscriptsAndSubscripts(t *testing.T) {
	got := body(t, show("M", 10, 20, 100, "x")+
		show("R", 7, 25, 104, "2")+
		show("R", 10, 30, 100, "+")+
		show("M", 10, 37, 100, "y")+
		show("R", 7, 42, 97, "1"))
	if got != `$x^{2}+y_{1}$` {
		t.Errorf("got %q", got)
	}
}

func TestGreekAndSymbols(t *testing.T) {
	got := body(t, show("M", 10, 20, 100, "a")+
		show("Y", 10, 26, 100, "i")+
		show("M", 10, 34, 100, "b"))
	if got != `$\alpha\in\beta$` {
		t.Errorf("got %q", got)
	}
}

func TestAFractionIsARuleWithMaterialEitherSide(t *testing.T) {
	got := body(t, show("M", 7, 20, 104, "a")+
		show("M", 7, 20, 96, "b")+
		bar(19.5, 100, 4.5, 0.4))
	if got != `$\frac{\alpha}{\beta}$` {
		t.Errorf("got %q", got)
	}
}

func TestARadicalReachesAsFarAsItsBar(t *testing.T) {
	got := body(t, show("M", 10, 28, 100, "ab")+
		show("Y", 10, 20, 104, "r")+
		bar(25, 107, 14, 0.4))
	if got != `$\sqrt{\alpha\beta}$` {
		t.Errorf("got %q", got)
	}
}

func TestABigOperatorTakesLimits(t *testing.T) {
	got := body(t, show("X", 10, 20, 105, "s")+
		show("M", 7, 21, 111, "b")+
		show("M", 7, 21, 99, "a")+
		show("M", 10, 34, 100, "ab"))
	if got != `$\sum_{\alpha}^{\beta}\alpha\beta$` {
		t.Errorf("got %q", got)
	}
}

func TestGrownDelimitersBecomeLeftAndRight(t *testing.T) {
	got := body(t, show("X", 10, 20, 102, "(")+
		show("M", 10, 26, 100, "ab")+
		show("X", 10, 37, 102, ")"))
	if got != `$\left(\alpha\beta\right)$` {
		t.Errorf("got %q", got)
	}
	// An opening delimiter with no closing one is written plainly: \left
	// with no \right does not compile.
	got = body(t, show("X", 10, 20, 102, "(")+show("M", 10, 26, 100, "ab"))
	if strings.Contains(got, `\left`) {
		t.Errorf("an unbalanced delimiter was written as \\left: %q", got)
	}
}

func TestAnEquationStopsAtTheProseAroundIt(t *testing.T) {
	// "and" is a word, not a variable, so the equation does not reach it;
	// the "+" is arithmetic set in the roman font, so it does.
	got := body(t, show("R", 10, 20, 150, "and")+
		show("M", 10, 40, 150, "x")+
		show("R", 10, 46, 150, "+")+
		show("M", 10, 53, 150, "a")+
		show("R", 10, 59, 150, ".")+
		show("R", 10, 66, 150, "text"))
	if !strings.Contains(got, `and $x+\alpha$. text`) {
		t.Errorf("got %q", got)
	}
}

func TestAnOperatorNameIsSetUpright(t *testing.T) {
	got := body(t, show("M", 10, 20, 100, "a")+
		show("R", 10, 27, 100, "sin")+
		show("M", 10, 43, 100, "b"))
	if !strings.Contains(got, `\sin`) {
		t.Errorf("got %q", got)
	}
	got = body(t, show("M", 10, 20, 100, "a")+
		show("R", 10, 27, 100, "det")+
		show("M", 10, 43, 100, "b"))
	if !strings.Contains(got, `\det`) {
		t.Errorf("got %q", got)
	}
}

func TestSectionsFromTheirNumbers(t *testing.T) {
	// Enough body text for the body size to be the body's, then headings.
	content := show("B", 14, 20, 180, "1 One") +
		show("R", 10, 20, 160, "Some running text on a line of its own here.") +
		show("B", 12, 20, 140, "1.1 Two") +
		show("R", 10, 20, 120, "More running text on a second line of prose.") +
		show("B", 11, 20, 100, "1.1.1 Three") +
		show("R", 10, 20, 80, "Yet more running text to weigh the body size.")
	got := body(t, content)
	for _, want := range []string{`\section{One}`, `\subsection{Two}`, `\subsubsection{Three}`} {
		if !strings.Contains(got, want) {
			t.Errorf("%s missing from\n%s", want, got)
		}
	}
}

func TestAnUnnumberedHeadingIsStarred(t *testing.T) {
	content := show("B", 14, 20, 180, "Introduction") +
		show("R", 10, 20, 160, "Some running text on a line of its own here.") +
		show("R", 10, 20, 145, "More running text to make the body size clear.")
	if got := body(t, content); !strings.Contains(got, `\section*{Introduction}`) {
		t.Errorf("got %s", got)
	}
}

func TestADisplayedEquationAndItsNumber(t *testing.T) {
	content := show("R", 10, 10, 180, "Body text that sets the margins of the column.") +
		show("M", 10, 80, 150, "ab") +
		show("R", 10, 160, 150, "(1)") +
		show("R", 10, 10, 120, "More body text that sets the margins here too.")
	got := body(t, content)
	if !strings.Contains(got, "\\begin{equation}\n\\alpha\\beta\n\\end{equation}") {
		t.Errorf("got %s", got)
	}
	// Without a number it is an unnumbered display.
	content = show("R", 10, 10, 180, "Body text that sets the margins of the column.") +
		show("M", 10, 80, 150, "ab") +
		show("R", 10, 10, 120, "More body text that sets the margins here too.")
	if got := body(t, content); !strings.Contains(got, "\\[\n\\alpha\\beta\n\\]") {
		t.Errorf("got %s", got)
	}
}

func TestParagraphsFromIndentation(t *testing.T) {
	content := show("R", 10, 20, 180, "The first paragraph starts here and runs on") +
		show("R", 10, 10, 168, "to a second line that reaches the margin too.") +
		show("R", 10, 20, 156, "The second paragraph is indented like this one") +
		show("R", 10, 10, 144, "and also runs to a second line at the margin.")
	got := body(t, content)
	if n := strings.Count(got, "\n\n"); n != 1 {
		t.Errorf("%d paragraph breaks in\n%s", n, got)
	}
}

func TestAHyphenAtTheEndOfALineIsPutBack(t *testing.T) {
	content := show("R", 10, 10, 180, "a word that has been hyphen-") +
		show("R", 10, 10, 168, "ated across two lines of text here")
	if got := body(t, content); !strings.Contains(got, "hyphenated") {
		t.Errorf("got %s", got)
	}
	// A hyphen followed by a capital was written by the author.
	content = show("R", 10, 10, 180, "the well-known Levi-") +
		show("R", 10, 10, 168, "Civita symbol appears in this line of text")
	if got := body(t, content); !strings.Contains(got, "Levi-Civita") {
		t.Errorf("got %s", got)
	}
}

func TestACentredBlockComesBackAsOne(t *testing.T) {
	content := show("R", 10, 20, 300, "Body text that reaches right across the column here.") +
		show("R", 10, 135, 280, "middle") +
		show("R", 10, 137, 268, "again") +
		show("R", 10, 20, 250, "Body text that reaches right across again here too.")
	got := strings.TrimSpace(wide(t, content).Body)
	if strings.Count(got, `\begin{center}`) != 1 {
		t.Errorf("got %s", got)
	}
}

func TestReadingTwoColumns(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 6; i++ {
		y := float64(300 - i*12)
		b.WriteString(show("R", 10, 20, y, "left column line here"))
		b.WriteString(show("R", 10, 320, y, "right column line here"))
	}
	doc := wide(t, b.String())
	if !strings.Contains(doc.Preamble, "twocolumn") {
		t.Errorf("the page was not read as two columns:\n%s", doc.Preamble)
	}
	first := strings.Index(doc.Body, "right column line here")
	if strings.LastIndex(doc.Body, "left column line here") > first {
		t.Errorf("the columns were interleaved:\n%s", doc.Body)
	}
}
func TestAPageNumberIsNotText(t *testing.T) {
	content := show("R", 10, 10, 180, "Body text on the first line of the page here.") +
		show("R", 10, 10, 168, "Body text on the second line of the page too.") +
		show("R", 10, 95, 20, "7")
	if got := body(t, content); strings.Contains(got, "7") {
		t.Errorf("the page number survived: %s", got)
	}
}

func TestTheWholeSource(t *testing.T) {
	d := pageWith(t, show("R", 10, 20, 150, "hello"), allFaces())
	s, err := Source(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`\documentclass{article}`, `\begin{document}`, "hello", `\end{document}`} {
		if !strings.Contains(s, want) {
			t.Errorf("%s missing from\n%s", want, s)
		}
	}
}

func TestChoosingPagesAndClass(t *testing.T) {
	d := pagesWith(t, []string{
		show("R", 10, 20, 150, "first"),
		show("R", 10, 20, 150, "second"),
		show("R", 10, 20, 150, "third"),
	}, allFaces(), [4]int{0, 0, 200, 200})
	doc, err := Reconstruct(d, Options{First: 2, Last: 2, Class: "report"})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Pages != 1 || !strings.Contains(doc.Body, "second") || strings.Contains(doc.Body, "first") {
		t.Errorf("got %d pages:\n%s", doc.Pages, doc.Body)
	}
	if !strings.Contains(doc.Preamble, `\documentclass{report}`) {
		t.Errorf("got %s", doc.Preamble)
	}
	// An unset range is the whole document.
	doc, err = Reconstruct(d, Options{})
	if err != nil || doc.Pages != 3 {
		t.Errorf("got %d pages, %v", doc.Pages, err)
	}
}

func TestADocumentThatCannotBeRead(t *testing.T) {
	// A page whose contents are missing still opens, and reads as nothing.
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	page := w.Add(reader.Dict{"Type": reader.Name("Page"), "Parent": pagesRef})
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
	doc, err := Reconstruct(d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(doc.Body) != "" {
		t.Errorf("an empty page says %q", doc.Body)
	}
}

// wide is the reconstructed body of a one-page document on a page big enough
// for a layout to be laid out on.
func wide(t *testing.T, content string) *Document {
	t.Helper()
	d := pagesWith(t, []string{content}, allFaces(), [4]int{0, 0, 600, 400})
	doc, err := Reconstruct(d, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
