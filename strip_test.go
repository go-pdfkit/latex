package latex

import (
	"strings"
	"testing"
)

// ⛔⛔ A FOOTNOTE IS NOT A RUNNING FOOT. Freeing the footnote from the line
// above it (#18) made it the last line of the page, on its own — and
// stripRunningHeads then threw it away, because a footnote is short, set below
// the text, and no larger than the body, which is every test it makes.
//
// One defect was traded for a worse one: a garbled footnote is visible, a
// dropped one is not. The reconstruction simply lost a sentence the page
// carries, and the README promises footnotes "come back as text at the foot of
// the page".
//
// The geometry is the real page's.
func TestAFootnoteIsNotThrownAwayAsARunningFoot(t *testing.T) {
	var atoms []atom
	add := func(s string, x0, x1, y, size float64) {
		w := (x1 - x0) / float64(len([]rune(s)))
		for i, r := range s {
			atoms = append(atoms, at(string(r), x0+float64(i)*w, y, w, size, shape{}))
		}
	}
	add("1 Preuve wazero", 72, 177.20, 122.89, 14)
	add("Le moteur gotex tourne ici dans wazero, composant ligatures et comme", 92, 541.39, 107.89, 10)
	add("dans un navigateur.", 72, 156.45, 95.89, 10)
	add("1. Runtime wasm pur-Go, sans sous-processus.", 92, 253.96, 73.76, 8)
	rules := []rule{{x0: 72, y0: 83.14, x1: 259.90, y1: 83.54}}

	lines, _ := order(groupLines(atoms, rules, 613.8), 613.8)
	kept := stripRunningHeads(lines, rules, 10)

	var said []string
	for _, l := range kept {
		var b strings.Builder
		for _, a := range l.atoms {
			b.WriteString(a.text)
		}
		said = append(said, b.String())
	}
	for _, s := range said {
		if strings.Contains(s, "Runtime") {
			return
		}
	}
	t.Errorf("the footnote was dropped; what is left is %q", said)
}
