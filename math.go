// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"sort"
	"strings"
	"unicode"
)

// This file rebuilds an equation from the marks TeX left behind, which is the
// part of this package that is actually difficult.
//
// TeX does not write mathematics into a PDF. It writes glyphs, each at an
// absolute place, from the three families it sets maths in: the math italic
// family for variables and the lowercase Greek, the symbol family for
// relations and operators, and the extension family for the big operators and
// for delimiters grown to fit. Everything the source said about structure is
// gone, and what is left is geometry:
//
//   - a superscript is a smaller glyph raised above the baseline, a subscript a
//     smaller glyph dropped below it;
//   - a fraction is a horizontal rule with material above it and material below
//     it, both set smaller than what surrounds them;
//   - a radical is a hook from the symbol family followed by a rule drawn over
//     everything it covers, which is what says where the radicand ends;
//   - a big operator's limits are the same shift, but centred on the operator
//     rather than following it;
//   - \left( and \right) are the same characters as ( and ), drawn from the
//     extension family at whatever size fits, and raised so that their middle
//     lands on the maths axis.
//
// So the reconstruction is a recursive decomposition of a set of placed glyphs.
// Take the widest rule that has material above and below it: that is the
// outermost fraction, and the material either side of it is what comes before
// and after. Recurse into each part with that rule removed. When no rule is
// left, walk what remains from left to right, and after each glyph take the
// smaller shifted ones that follow as its scripts. Recurse into those too.
//
// The decomposition is honest about what it cannot see. Nothing in the geometry
// distinguishes \alpha from a variable named by the same character, or \cdot
// from \bullet drawn small, or an author's \mathrm{Re} from the \Re symbol.
// Where a choice has to be made it is made once, in mathnames.go, and written
// down there.

// A mathScope is the rules an equation may still use. Rules are consumed as the
// decomposition goes down, so that a fraction inside a numerator is found by the
// recursion rather than by the same rule matching twice.
type mathScope struct {
	rules []rule
	// pairs says whether grown delimiters may be written as \left and
	// \right. They may only when the equation has as many of one as of the
	// other; an equation cut in half by a line break has not, and \left with
	// no \right does not compile.
	pairs bool
}

// scriptDrop is how far a piece must leave the baseline, as a fraction of its
// own size, before it counts as a script rather than as rounding.
const scriptDrop = 0.12

// mathSource writes an equation. items are its glyphs in order across the page,
// rs the rules that fall within it.
func mathSource(items []atom, rs []rule) string {
	m := mathScope{rules: rs, pairs: balanced(items)}
	return repair(strings.TrimSpace(m.render(items)))
}

// balanced reports whether the grown delimiters in an equation pair up.
func balanced(items []atom) bool {
	open, closed := 0, 0
	for _, a := range items {
		if a.sh.math != mathExt {
			continue
		}
		for _, r := range a.text {
			if _, ok := openDelimiter[r]; ok {
				open++
				continue
			}
			if _, ok := closeDelimiter[r]; ok {
				closed++
			}
		}
	}
	return open > 0 && open == closed
}

// render writes a group of glyphs, taking the outermost fraction first.
func (m mathScope) render(items []atom) string {
	if len(items) == 0 {
		return ""
	}
	if s, ok := m.fraction(items); ok {
		return s
	}
	return m.sequence(items)
}

// fraction finds the widest rule with material both above and below it, and
// writes what it finds as \frac, with whatever lies to either side of the rule
// written around it.
func (m mathScope) fraction(items []atom) (string, bool) {
	best, found := -1, 0.0
	for i, r := range m.rules {
		if !m.bar(r, items) {
			continue
		}
		if w := r.x1 - r.x0; w > found {
			best, found = i, w
		}
	}
	if best < 0 {
		return "", false
	}
	r := m.rules[best]
	inner := m.without(best)
	var before, above, below, after []atom
	for _, a := range items {
		switch {
		case !r.spans(a.midX(), barSlack):
			if a.midX() < r.x0 {
				before = append(before, a)
			} else {
				after = append(after, a)
			}
		case a.y > r.midY():
			above = append(above, a)
		default:
			below = append(below, a)
		}
	}
	return inner.render(before) + `\frac{` + inner.render(above) + `}{` +
		inner.render(below) + `}` + inner.render(after), true
}

// barSlack is how far outside a rule a glyph's centre may sit and still count
// as under it. A fraction bar is drawn a shade wider than its material, but a
// glyph beside the fraction starts a good deal further away than this.
const barSlack = 1.0

// bar reports whether a rule is a fraction bar for these glyphs: wide, with
// material both above and below it, and not the bar of a radical â which is
// drawn hard against the radical sign's right edge and has nothing under it
// that is not also under the sign.
func (m mathScope) bar(r rule, items []atom) bool {
	if !r.wide() {
		return false
	}
	var up, down bool
	for _, a := range items {
		if radicalOf(a) && abs(a.right()-r.x0) < radicalReach {
			return false
		}
		if !r.spans(a.midX(), barSlack) {
			continue
		}
		if a.y > r.midY() {
			up = true
		} else {
			down = true
		}
	}
	return up && down
}

// without is the scope with one rule taken out.
func (m mathScope) without(i int) mathScope {
	rest := make([]rule, 0, len(m.rules)-1)
	rest = append(rest, m.rules[:i]...)
	rest = append(rest, m.rules[i+1:]...)
	return mathScope{rules: rest, pairs: m.pairs}
}

// radicalReach is how far a radical's bar may start from the sign's right edge.
const radicalReach = 2.5

// radicalSign is the character a radical is drawn with, U+221A.
const radicalSign = 0x221A

// radicalOf reports whether a glyph is a radical sign.
func radicalOf(a atom) bool { return strings.ContainsRune(a.text, radicalSign) }

// sequence writes a group with no fraction left in it: left to right, each
// glyph followed by the smaller shifted ones that hang from it.
func (m mathScope) sequence(items []atom) string {
	base, size := level(items)
	var out []string
	for i := 0; i < len(items); {
		a := items[i]
		if radicalOf(a) {
			if s, n, ok := m.radical(items, i); ok {
				out = append(out, s)
				i = n
				continue
			}
		}
		out = append(out, m.glyph(a))
		i++
		j := i
		for j < len(items) && shifted(items[j], base, size) {
			j++
		}
		if j > i {
			out = append(out, m.scripts(items[i:j], base))
			i = j
		}
	}
	return join(out)
}

// radical writes \sqrt. The bar drawn over the radicand says how far it
// reaches; without one there is nothing to say, and the sign is written as a
// bare \surd instead.
func (m mathScope) radical(items []atom, i int) (string, int, bool) {
	a := items[i]
	for k, r := range m.rules {
		if !r.wide() || abs(r.x0-a.right()) >= radicalReach || r.midY() <= a.y {
			continue
		}
		end := i + 1
		for end < len(items) && r.spans(items[end].midX(), barSlack) {
			end++
		}
		if end == i+1 {
			continue
		}
		return `\sqrt{` + m.without(k).render(items[i+1:end]) + `}`, end, true
	}
	return "", 0, false
}

// level is the baseline a group sits on and the size it is set at.
//
// The size is the largest there is. The baseline is the one the most ink sits
// on at that size — not the one under the widest single glyph, because in
// \\sum_{i=1}^n f(x_i) the widest glyph at full size is the \\sum itself, and a
// big operator is drawn RAISED so that it straddles the maths axis. Taking its
// baseline as the group’s puts the upper limit below it and both limits come
// back as subscripts. Glyphs the extension family raises by construction, and
// the radical sign, are left out of the vote for the same reason.
func level(items []atom) (float64, float64) {
	size := 0.0
	for _, a := range items {
		size = max(size, a.size)
	}
	y, ok := vote(items, size, true)
	if !ok {
		y, _ = vote(items, size, false)
	}
	return y, size
}

// vote is the baseline carrying the most width at a group’s own size, and false
// when nothing qualifies. grounded leaves out the glyphs that are raised
// whatever the equation says.
func vote(items []atom, size float64, grounded bool) (float64, bool) {
	weight := map[float64]float64{}
	for _, a := range items {
		if a.size <= scriptSize*size {
			continue
		}
		if grounded && (a.sh.math == mathExt || radicalOf(a)) {
			continue
		}
		weight[a.y] += a.width
	}
	best, found := 0.0, -1.0
	for y, w := range weight {
		if w > found || (w == found && y > best) {
			best, found = y, w
		}
	}
	return best, found >= 0
}

// shifted reports whether a glyph is a script of what precedes it: smaller than
// the level it is being read at, and off its baseline.
func shifted(a atom, base, size float64) bool {
	return a.size < scriptSize*size && abs(a.y-base) > scriptDrop*a.size
}

// scripts writes the superscripts and subscripts that hang from one glyph.
// Both are written even when only one is there, and each is recursed into,
// since a script may hold a fraction of its own.
func (m mathScope) scripts(items []atom, base float64) string {
	var up, down []atom
	for _, a := range items {
		if a.y > base {
			up = append(up, a)
			continue
		}
		down = append(down, a)
	}
	out := ""
	if len(down) > 0 {
		out += `_{` + m.render(down) + `}`
	}
	if len(up) > 0 {
		out += `^{` + m.render(up) + `}`
	}
	return out
}

// glyph writes one piece of text as mathematics.
func (m mathScope) glyph(a atom) string {
	if s, ok := m.operatorName(a); ok {
		return s
	}
	// The characters of one run need the same gaps between them as the runs
	// of one equation. A run holding an epsilon and an e is drawn as two
	// glyphs but arrives as one piece of text, and writing it out without
	// a gap gives \\varepsilone, which is not a command any engine has.
	var parts []string
	for _, r := range a.text {
		parts = append(parts, m.rune(a, r))
	}
	return join(parts)
}

// operatorName catches a run of upright letters, which inside an equation is
// either one of the names TeX has a command for â \sin, \log, \max â or a word
// the author set with \mathrm.
func (m mathScope) operatorName(a atom) (string, bool) {
	if a.sh.isMath() || len([]rune(a.text)) < 2 {
		return "", false
	}
	for _, r := range a.text {
		if !unicode.IsLetter(r) {
			return "", false
		}
	}
	if functionName[a.text] {
		return `\` + a.text, true
	}
	return `\mathrm{` + a.text + `}`, true
}

// rune writes one character as mathematics.
func (m mathScope) rune(a atom, r rune) string {
	if a.sh.math == mathExt && m.pairs {
		if s, ok := openDelimiter[r]; ok {
			return `\left` + s
		}
		if s, ok := closeDelimiter[r]; ok {
			return `\right` + s
		}
	}
	if s, ok := mathCommand[r]; ok {
		return s
	}
	if decoration(r) {
		return ""
	}
	if unicode.IsLetter(r) || mathPlain(r) {
		return string(r)
	}
	return escapeText(string(r))
}

// join puts an equation's pieces together, with a space wherever leaving one
// out would run a command's name into what follows it.
func join(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if needsGap(b.String(), p) {
			b.WriteByte(' ')
		}
		b.WriteString(p)
	}
	return b.String()
}

// needsGap reports whether a control word at the end of what is written would
// swallow the start of what comes next.
func needsGap(done, next string) bool {
	if done == "" || next == "" {
		return false
	}
	last := rune(done[len(done)-1])
	if !unicode.IsLetter(last) || !unicode.IsLetter(rune(next[0])) {
		return false
	}
	return endsInControlWord(done)
}

// endsInControlWord reports whether the text ends inside a \command rather than
// in ordinary letters.
func endsInControlWord(s string) bool {
	i := len(s)
	for i > 0 && isASCIILetter(s[i-1]) {
		i--
	}
	return i > 0 && s[i-1] == '\\'
}

// isASCIILetter reports whether a byte is one of the letters a control word may
// be spelled with.
func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// mathRules is the rules that fall inside an equation's box, which is what the
// decomposition is allowed to use.
func mathRules(items []atom, rs []rule) []rule {
	x0, x1 := items[0].x, items[0].right()
	ylo, yhi := items[0].y, items[0].y
	size := 0.0
	for _, a := range items {
		x0, x1 = min(x0, a.x), max(x1, a.right())
		ylo, yhi = min(ylo, a.y), max(yhi, a.y)
		size = max(size, a.size)
	}
	var out []rule
	for _, r := range rs {
		if r.x0 >= x0-barSlack && r.x1 <= x1+barSlack &&
			r.midY() > ylo-0.5*size && r.midY() < yhi+1.2*size {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].x0 < out[j].x0 })
	return out
}
