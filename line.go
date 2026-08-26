// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import "sort"

// This file puts the page's text back into lines, which is harder than it
// sounds for exactly one reason: mathematics does not sit on the baseline.
//
// A page of prose is easy. TeX gives every piece of a line the same baseline, so
// grouping by that one number is enough. But the moment an equation appears, the
// same line carries a superscript four points up, a subscript three points down,
// the two halves of a fraction on either side of a rule, the limits of a big
// operator further out still, a radical sign raised so its bar clears the
// radicand, and a \left( grown from the extension family and lifted so that its
// middle lands on the maths axis. Every one of those has to end up on the line
// it belongs to, and none of them may drag in the line above.
//
// The obvious way â walk down the page attaching each baseline to the one before
// â does not work, and fails in a way that is worth writing down because the
// output looks plausible. A superscript sits ABOVE the line it belongs to, so
// walking downwards meets it first and has to decide what it is before the line
// exists; a radical sign raised seven points sits nearer the line above it than
// the one it is part of. The \sqrt{x} on one line ends up in the sentence on the
// line before, and the page still reads as prose.
//
// So this goes the other way round. The biggest pieces are placed first, because
// a page's structure is carried by its body text and its headings, and they are
// never displaced. Everything smaller is then offered to the lines that already
// exist and goes to the nearest one that will have it â which for a superscript
// caught between two lines is the one four points away rather than the one eight
// points away, whichever came first down the page.

// bandTolerance is how far apart two baselines may be and still be the same
// one. TeX writes a baseline as an exact number, but a rounded one, and two
// pieces set by different routes can differ in the last place.
const bandTolerance = 0.35

// The reaches a displaced piece is allowed, each a fraction of the size of the
// line it is offered to.
const (
	// sameBaseline is the slack for two pieces meant to be level.
	sameBaseline = 0.25
	// scriptSpan is how far a smaller piece may sit from the line it hangs
	// from. A superscript rises about four tenths of the size and the upper
	// limit of a big operator three quarters of it.
	scriptSpan = 0.85
	// mathReach is how far a piece drawn in a symbol or extension font may be
	// lifted. A radical sign and a grown delimiter are raised by design, at
	// the same nominal size as their surroundings, so the size test cannot
	// catch them â but they are only ever raised, never dropped, and that is
	// what keeps this from reaching down into the next line.
	mathReach = 0.85
	// sideReach is how far to either side of a line a piece may sit and
	// still belong to it, which stops a script in one column being claimed
	// by a line in the other.
	sideReach = 2.0
	// scriptSize is how much smaller a piece must be than the line it hangs
	// from. TeX sets a first-level script at seven tenths of the size.
	scriptSize = 0.92
)

// A line is everything on one baseline, plus the parts of it that were shifted
// off that baseline.
type line struct {
	atoms []atom
	// y is the baseline the line's main material sits on and size how tall
	// that material is.
	y, size float64
	// x0 and x1 are the line's left and right ends.
	x0, x1 float64
	// left and right are the margins of the column the line is in, which is
	// what an indent and a centred line are measured against.
	left, right float64
}

// groupLines puts the page's atoms into lines. The atoms must already be sorted
// from the top of the page down, which readPage does.
func groupLines(atoms []atom, rules []rule, width float64) []line {
	bs := bands(atoms)
	if x, ok := gutter(atoms, width); ok {
		bs = split(bs, x)
	}
	order := make([]int, len(bs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		x, y := bs[order[a]], bs[order[b]]
		if x.size != y.size {
			return x.size > y.size
		}
		return x.width > y.width
	})
	var lines []line
	for _, i := range order {
		if k := nearest(lines, bs[i], rules); k >= 0 {
			lines[k] = attach(lines[k], bs[i])
			continue
		}
		lines = append(lines, seed(bs[i]))
	}
	sort.SliceStable(lines, func(a, b int) bool { return lines[a].y > lines[b].y })
	for i := range lines {
		sort.SliceStable(lines[i].atoms, func(a, b int) bool {
			return lines[i].atoms[a].x < lines[i].atoms[b].x
		})
	}
	return lines
}

// A band is the atoms that share one baseline.
type band struct {
	atoms   []atom
	y       float64
	size    float64
	width   float64
	x0, x1  float64
	allMath bool
}

// bands groups atoms that share a baseline.
func bands(atoms []atom) []band {
	var out []band
	for _, a := range atoms {
		if n := len(out); n > 0 && abs(out[n-1].y-a.y) <= bandTolerance {
			out[n-1].atoms = append(out[n-1].atoms, a)
			continue
		}
		out = append(out, band{atoms: []atom{a}, y: a.y, x0: a.x, x1: a.right()})
	}
	for i := range out {
		out[i] = measured(out[i].atoms)
	}
	return out
}

// nearest is the line a band belongs to, or -1 when it starts one of its own.
// Where more than one line would take it, the closest wins: a superscript
// caught between two lines belongs to the one it is nearer.
func nearest(lines []line, b band, rules []rule) int {
	best, found := -1, 0.0
	for i, l := range lines {
		if !accepts(l, b, rules) {
			continue
		}
		if d := abs(b.y - l.y); best < 0 || d < found {
			best, found = i, d
		}
	}
	return best
}

// accepts reports whether a line will take a band. Bands are offered largest
// first, so a band is never larger than the line it is offered to by more than
// rounding.
func accepts(l line, b band, rules []rule) bool {
	if b.size > l.size*1.02 {
		return false
	}
	if b.x0 > l.x1+sideReach*l.size || b.x1 < l.x0-sideReach*l.size {
		return false
	}
	dy := b.y - l.y
	switch {
	case abs(dy) < sameBaseline*l.size:
		return true
	case b.size < scriptSize*l.size && abs(dy) < scriptSpan*l.size:
		return true
	case b.size < scriptSize*l.size && abs(dy) < limitSpan*l.size && overOperator(l, b):
		return true
	case b.allMath && dy > 0 && dy < mathReach*l.size:
		return true
	case sharesBar(l, b, rules):
		return true
	}
	return false
}

// seed starts a line from a band.
func seed(b band) line {
	return line{atoms: b.atoms, y: b.y, size: b.size, x0: b.x0, x1: b.x1}
}

// attach adds a band to a line, which keeps the baseline and size it was
// seeded with: those belong to the line's own material, not to what hangs off
// it.
func attach(l line, b band) line {
	l.atoms = append(l.atoms, b.atoms...)
	l.x0, l.x1 = min(l.x0, b.x0), max(l.x1, b.x1)
	return l
}

// headFoot is how far from the rest of the page a short line must sit, in body
// sizes, to be a running head or a page number rather than part of the text.
const headFoot = 2.2

// headFootWidth is how much of the column such a line may fill.
const headFootWidth = 0.4

// stripRunningHeads drops the page number and the running head. Neither was in
// the source: LaTeX put them there, and writing them back would put them on the
// page twice â once as text in the middle of a paragraph and once again by the
// page style.
func stripRunningHeads(lines []line, body float64) []line {
	for len(lines) > 1 && isolated(lines[0], lines[1], body) {
		lines = lines[1:]
	}
	for len(lines) > 1 && isolated(lines[len(lines)-1], lines[len(lines)-2], body) {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// isolated reports whether an edge line is short, set well away from its
// neighbour, and no larger than the body text. The size test is what keeps a
// section heading at the top of a page — which is also short, and also set off
// from what follows it — from being thrown away as a running head.
func isolated(edge, next line, body float64) bool {
	return edge.size <= body*1.05 &&
		edge.x1-edge.x0 < headFootWidth*(edge.right-edge.left) &&
		abs(edge.y-next.y) > headFoot*body
}

// fracReach is how far apart the two halves of a displayed fraction may sit,
// as a multiple of the size they are set at. A display sets both halves at full
// size, one above the bar and one below, and TeX leaves room for the tallest
// thing in each.
const fracReach = 2.4

// sharesBar reports whether a rule between a line and a band is a fraction bar
// with one of them above it and the other below.
//
// This is the case the size and reach tests above cannot cover. A superscript
// is recognised by being smaller; the two halves of a DISPLAYED fraction are
// both set at full size, three points above and three points below a bar, and
// nothing about either of them says they are not two ordinary lines of text one
// after the other. The bar says it. Without this, \frac{d}{dt} at the head of a
// displayed equation comes out as three separate lines - a d, the rest of the
// equation, and a dt - which compiles, and is wrong.
func sharesBar(l line, b band, rules []rule) bool {
	if abs(b.y-l.y) > fracReach*l.size {
		return false
	}
	for _, r := range rules {
		// The bar sits between the two baselines, give or take: a fraction
		// is set about the maths axis, which is a quarter of the size above
		// the baseline of the equation it is in, so a bar with the whole
		// fraction below it still lies a little above the line.
		if !r.wide() || r.midY() < min(l.y, b.y)-0.2*l.size || r.midY() > max(l.y, b.y)+0.8*l.size {
			continue
		}
		// The band has to be one half of the fraction: entirely within the
		// bar's reach, and not much narrower than it. A bar is drawn a
		// little wider than the wider of the two halves it separates, so a
		// rule many times the width of what sits under it is a table's line
		// and not a fraction's - which is what keeps a row of a table from
		// being folded into the row above.
		if b.x0 < r.x0-barSlack || b.x1 > r.x1+barSlack {
			continue
		}
		if r.x1-r.x0 > 2*(b.x1-b.x0)+4 {
			continue
		}
		// And the line has to be the equation the fraction belongs to: its
		// other half, or the material either side of it.
		if overlaps(r, l.x0-l.size, l.x1+l.size) {
			return true
		}
	}
	return false
}

// overlaps reports whether a stretch of the page falls within a rule's reach.
func overlaps(r rule, x0, x1 float64) bool {
	return x1 > r.x0-barSlack && x0 < r.x1+barSlack
}

// limitSpan is how far the limits of a big operator may sit from the baseline
// of the equation they are part of, as a multiple of its size. A displayed
// \sum sets its limits further out than a superscript by half again, which is
// most of the way to the next line - so this reach is only offered to a band
// that is actually sitting over such an operator.
const limitSpan = 1.3

// overOperator reports whether a band is centred on a big operator the line
// already holds, which is what a limit is and a superscript is not: TeX puts a
// superscript AFTER what it belongs to and a limit ABOVE it.
func overOperator(l line, b band) bool {
	mid := (b.x0 + b.x1) / 2
	for _, a := range l.atoms {
		if a.sh.math != mathExt {
			continue
		}
		if mid > a.x-l.size && mid < a.right()+l.size {
			return true
		}
	}
	return false
}

// The gutter of a two-column page: how wide the empty strip down the middle
// must be, in points, and how far from the centre it may sit.
const (
	gutterWidth = 8.0
	gutterZone  = 0.15
)

// gutter is where a page's columns are divided, and false for a page with one
// column.
//
// This has to be found before the lines are built rather than after. Two columns
// set to the same grid put their lines on the SAME baselines, so a page read by
// baseline alone comes back with every line of the left column joined to the
// line of the right column beside it - and the join happens before anything has
// had a chance to notice there are two columns. What gives it away is a strip
// down the middle of the page that no glyph enters, which is what a gutter is.
func gutter(atoms []atom, width float64) (float64, bool) {
	if width <= 0 || len(atoms) == 0 {
		return 0, false
	}
	lo, hi := int(width*(0.5-gutterZone)), int(width*(0.5+gutterZone))
	covered := make([]bool, hi-lo+1)
	var left, right bool
	for _, a := range atoms {
		if a.x < float64(lo) {
			left = true
		}
		if a.right() > float64(hi) {
			right = true
		}
		for i := max(int(a.x)-lo, 0); i <= min(int(a.right())-lo, len(covered)-1); i++ {
			covered[i] = true
		}
	}
	if !left || !right {
		return 0, false
	}
	best, run, end := 0, 0, 0
	for i, c := range covered {
		if c {
			run = 0
			continue
		}
		run++
		if run > best {
			best, end = run, i
		}
	}
	if float64(best) < gutterWidth {
		return 0, false
	}
	return float64(lo+end) - float64(best)/2, true
}

// split cuts every band at the gutter, so that the left column's line and the
// right column's are two lines rather than one.
func split(bs []band, x float64) []band {
	var out []band
	for _, b := range bs {
		var lhs, rhs []atom
		for _, a := range b.atoms {
			if a.midX() < x {
				lhs = append(lhs, a)
				continue
			}
			rhs = append(rhs, a)
		}
		if len(lhs) == 0 || len(rhs) == 0 {
			out = append(out, b)
			continue
		}
		out = append(out, measured(lhs), measured(rhs))
	}
	return out
}

// measured is a band built from atoms that already share a baseline.
func measured(atoms []atom) band {
	b := band{atoms: atoms, y: atoms[0].y, x0: atoms[0].x, x1: atoms[0].right(), allMath: true}
	for _, a := range atoms {
		b.size = max(b.size, a.size)
		b.width += a.width
		b.x0, b.x1 = min(b.x0, a.x), max(b.x1, a.right())
		if !a.sh.isMath() {
			b.allMath = false
		}
	}
	return b
}
