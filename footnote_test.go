package latex

import (
	"strings"
	"testing"
)

// ⛔⛔ A FOOTNOTE IS NOT PART OF THE LINE ABOVE IT. Reported from the first run
// of cmd/pdf2latex against a real page (go-pdfkit/latex#18): the last line of
// the body and the footnote under it came out interleaved character by
// character, so the paragraph read
//
//	comme dans1u. nRunnativmige awtaesumr.pur-Go, sans sous-processus.
//
// where the page holds "dans un navigateur." on one line and "1. Runtime wasm
// pur-Go, sans sous-processus." in a smaller face below it.
//
// The geometry here is the real page's, read off the PDF with extract.Runs:
// the body sits on y=95.89 at 10pt from x=72, and the footnote on y=73.76 at
// 8pt from x=92. Both are given as one atom per glyph, which is what extract
// returns.
func TestAFootnoteIsNotPartOfTheLineAboveIt(t *testing.T) {
	var atoms []atom
	// ⛔ THE ENDS ARE GIVEN, NOT GUESSED FROM A WIDTH PER LETTER. A first
	// version of this test spread the glyphs at half the font size, which made
	// the footnote reach x=268 — past the rule's right end at 259.90 — and the
	// band was then rejected for OVERSHOOTING THE BAR rather than accepted as
	// half a fraction. The test passed against the broken code. Where a line
	// ENDS relative to the rule is one of the things being tested, so it is
	// read off the page rather than modelled.
	add := func(s string, x0, x1, y, size float64) {
		w := (x1 - x0) / float64(len([]rune(s)))
		for i, r := range s {
			atoms = append(atoms, at(string(r), x0+float64(i)*w, y, w, size, shape{}))
		}
	}
	// Sorted from the top of the page down, which is what groupLines requires.
	add("dans un navigateur.", 72, 156.45, 95.89, 10)
	add("1. Runtime wasm pur-Go, sans sous-processus.", 92, 253.96, 73.76, 8)

	// ⛔⛔ AND THE SEPARATOR RULE IS THE WHOLE CASE, AT ITS REAL MEASUREMENTS.
	// A footnote is set under a short horizontal rule — the one mark on a page
	// that looks like a fraction bar with material above and material below.
	//
	// The first version of this test left the rule out and PASSED while the
	// real page was still wrong; the second invented a rule at x 72..216 and
	// passed too, because the width decides it. These are the numbers read off
	// the page: x 72.00..259.90, y 83.14..83.54.
	rules := []rule{{x0: 72, y0: 83.14, x1: 259.90, y1: 83.54}}

	lines := groupLines(atoms, rules, 612)
	if len(lines) != 2 {
		var said []string
		for _, l := range lines {
			var b strings.Builder
			for _, a := range l.atoms {
				b.WriteString(a.text)
			}
			said = append(said, b.String())
		}
		t.Fatalf("the body and the footnote came back as %d line(s): %q", len(lines), said)
	}
}
