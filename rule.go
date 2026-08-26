// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import "github.com/go-pdfkit/reader"

// This file finds the straight lines on a page, because two of the things this
// package has to reconstruct are not text at all.
//
// A fraction is a rule with its numerator above and its denominator below: TeX
// draws \frac{a+b}{c} as five glyphs and one thin horizontal bar, and without
// the bar there is nothing to say the a+b and the c belong together rather than
// being two lines of an equation. A radical is the same — a hook glyph followed
// by a bar over the radicand, and the bar is what says how far the radicand
// reaches. Ruled tables are made of the same material.
//
// So this walks the content stream a second time, ignoring the text and keeping
// only the marks. Two producers draw a rule two different ways and both have to
// be read: pdfTeX emits a filled rectangle (re … f), while xdvipdfmx — what
// XeTeX and tectonic write through — emits a stroked segment (m … l … S) whose
// thickness is the line width. A reader that knows only one of them silently
// loses every fraction in half the world's PDFs.

// A rule is a straight mark on the page, given as the box it covers in the same
// coordinates the text comes back in: up from the bottom left of what is
// visible, in points.
type rule struct {
	x0, y0, x1, y1 float64
}

// wide reports whether the mark is longer across than it is tall, which is what
// separates a fraction bar or a table's horizontal line from a column divider.
func (r rule) wide() bool { return r.x1-r.x0 >= r.y1-r.y0 }

// midY is the height of the mark's centre.
func (r rule) midY() float64 { return (r.y0 + r.y1) / 2 }

// midX is the horizontal centre of the mark.
func (r rule) midX() float64 { return (r.x0 + r.x1) / 2 }

// spans reports whether x falls within the mark's horizontal reach, with a
// little slack at each end: a fraction bar is drawn a shade wider than the
// material it covers, but not always wider than a glyph's own side bearings.
func (r rule) spans(x, slack float64) bool {
	return x >= r.x0-slack && x <= r.x1+slack
}

// defaultLineWidth is the thickness of a stroke drawn with a width of zero,
// which PDF defines as the thinnest line the device can manage. A fraction bar
// is never drawn that way, but a hairline table rule is, and giving it no
// thickness at all would make it a mark with no height.
const defaultLineWidth = 0.4

// pathState is the part of the graphics state that says where a mark lands.
type pathState struct {
	ctm   matrix
	width float64
}

// A pathScan walks a content stream keeping the marks and dropping the text.
type pathScan struct {
	g     pathState
	stack []pathState
	// rects are the rectangles the current path has collected, in page
	// coordinates, waiting to learn whether the path will be filled.
	rects []rule
	// segs are the straight straight pieces of the current path, waiting to learn
	// whether it will be stroked.
	segs []stroke
	// curved records that the path contains something this cannot reduce to
	// a straight mark, so the whole path is dropped rather than half kept.
	curved bool
	// cur is where the current subpath is, for a lineto to draw from.
	cur point
	out []rule
}

// A point is a place on the page.
type point struct{ x, y float64 }

// A stroke is a straight piece of a path, in page coordinates.
type stroke struct{ a, b point }

// rules is every straight mark on the i'th page, counting from one. Marks drawn
// inside a form XObject are not reached: a form has its own content stream and
// its own transform, and following them is the renderer's work rather than
// this package's. A fraction TeX put in a form — which it does not do — would
// be missed.
func rules(d *reader.Document, i int, origin point) []rule {
	ops, err := d.PageOperations(i)
	if err != nil {
		return nil
	}
	s := &pathScan{g: pathState{
		ctm:   matrix{1, 0, 0, 1, -origin.x, -origin.y},
		width: 1,
	}}
	for _, op := range ops {
		s.step(op)
	}
	return s.out
}

// step reads one operation.
func (s *pathScan) step(op reader.Operation) {
	n := numbers(op.Operands)
	switch op.Operator {
	case "q":
		s.stack = append(s.stack, s.g)
	case "Q":
		if len(s.stack) > 0 {
			s.g = s.stack[len(s.stack)-1]
			s.stack = s.stack[:len(s.stack)-1]
		}
	case "cm":
		if len(n) >= 6 {
			s.g.ctm = matrix{n[0], n[1], n[2], n[3], n[4], n[5]}.mul(s.g.ctm)
		}
	case "w":
		if len(n) >= 1 {
			s.g.width = n[0]
		}
	case "gs":
		// An external graphics state can carry a line width, but reading it
		// means resolving a resource; a rule whose width comes from one is
		// rare enough to leave at the default rather than guess.
	case "m":
		if len(n) >= 2 {
			s.cur = s.place(n[0], n[1])
		}
	case "l":
		if len(n) >= 2 {
			p := s.place(n[0], n[1])
			s.segs = append(s.segs, stroke{s.cur, p})
			s.cur = p
		}
	case "c", "v", "y":
		s.curved = true
	case "re":
		if len(n) >= 4 {
			s.rect(n[0], n[1], n[2], n[3])
		}
	case "f", "F", "f*", "B", "B*", "b", "b*":
		s.paint(true, op.Operator != "f" && op.Operator != "F" && op.Operator != "f*")
	case "S", "s":
		s.paint(false, true)
	case "n":
		s.reset()
	}
}

// place moves a point into page coordinates.
func (s *pathScan) place(x, y float64) point {
	px, py := s.g.ctm.apply(x, y)
	return point{px, py}
}

// rect records a rectangle, put the right way round.
func (s *pathScan) rect(x, y, w, h float64) {
	a := s.place(x, y)
	b := s.place(x+w, y+h)
	if a.x > b.x {
		a.x, b.x = b.x, a.x
	}
	if a.y > b.y {
		a.y, b.y = b.y, a.y
	}
	s.rects = append(s.rects, rule{a.x, a.y, b.x, b.y})
}

// paint ends a path, keeping what it drew. A filled rectangle is a mark as it
// stands; a stroked segment becomes one by giving it the line's thickness.
func (s *pathScan) paint(fill, stroke bool) {
	if s.curved {
		s.reset()
		return
	}
	// A rectangle is one mark whether it is filled, stroked, or both: an
	// operator that does both must not put it in twice.
	if fill || stroke {
		s.out = append(s.out, s.rects...)
	}
	if stroke {
		half := s.strokeHalfWidth()
		for _, g := range s.segs {
			if r, ok := g.mark(half); ok {
				s.out = append(s.out, r)
			}
		}
	}
	s.reset()
}

// strokeHalfWidth is half the thickness a stroke gets on the page.
func (s *pathScan) strokeHalfWidth() float64 {
	w := s.g.width
	if w <= 0 {
		w = defaultLineWidth
	}
	return w * s.g.ctm.scale() / 2
}

// mark turns a straight segment into the box it covers, and reports false for
// one that runs at an angle: a rule is horizontal or vertical, and a diagonal
// line is part of a drawing.
func (g stroke) mark(half float64) (rule, bool) {
	dx, dy := abs(g.b.x-g.a.x), abs(g.b.y-g.a.y)
	switch {
	case dy <= half && dx > 0:
		y := (g.a.y + g.b.y) / 2
		return rule{min(g.a.x, g.b.x), y - half, max(g.a.x, g.b.x), y + half}, true
	case dx <= half && dy > 0:
		x := (g.a.x + g.b.x) / 2
		return rule{x - half, min(g.a.y, g.b.y), x + half, max(g.a.y, g.b.y)}, true
	}
	return rule{}, false
}

// reset clears the current path.
func (s *pathScan) reset() {
	s.rects, s.segs, s.curved = nil, nil, false
}

// numbers reads the numeric operands of an operation, stopping at the first
// that is not a number since an operator's numbers come first.
func numbers(ops []reader.Object) []float64 {
	out := make([]float64, 0, len(ops))
	for _, o := range ops {
		v, ok := reader.ToFloat(o)
		if !ok {
			break
		}
		out = append(out, v)
	}
	return out
}

// abs is the size of a number without its sign.
func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
