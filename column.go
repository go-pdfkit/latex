// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

// This file puts a page's lines into reading order, which on a two-column paper
// is not the order they appear in from the top of the page down.
//
// The test for two columns is the gutter: a band down the middle of the page
// that lines stop at rather than cross. A one-column page has lines crossing the
// middle constantly; a two-column page has almost none, and the few there are —
// the title, a wide figure, a section heading set across both — are the ones
// that have to stay where they are rather than being sorted into a column.
//
// So the page is cut at every full-width line. Between two of them the lines are
// read left column first and right column second, which is what a reader does.

// gutterBand is how wide the band down the middle of the page is, as a fraction
// of the page's width. A two-column layout leaves about a twentieth of the page
// empty there; this is narrower, so that a line reaching a little into the
// gutter still counts as staying in its column.
const gutterBand = 0.02

// minColumnLines is how many lines each column must have before the page is
// read as two. A page with three lines on the left and three on the right is
// more likely to be a table or a title block than a two-column layout.
const minColumnLines = 5

// A side is where a line sits relative to the gutter.
type side int

const (
	full side = iota
	leftSide
	rightSide
)

// order returns the page's lines in reading order, and reports whether the page
// was read as two columns. Each line comes back knowing the margins of the
// column it is in, which is what an indent is measured against.
func order(lines []line, width float64) ([]line, bool) {
	sides := make([]side, len(lines))
	mid, band := width/2, gutterBand*width
	var nleft, nright, nfull int
	for i, l := range lines {
		switch {
		case l.x0 < mid-band && l.x1 > mid+band:
			sides[i], nfull = full, nfull+1
		case l.x1 <= mid+band:
			sides[i], nleft = leftSide, nleft+1
		default:
			sides[i], nright = rightSide, nright+1
		}
	}
	if nleft < minColumnLines || nright < minColumnLines || nfull*4 > nleft+nright {
		return margins(lines, nil), false
	}
	var out []line
	var outSides []side
	var run []int
	take := func(i int) {
		out = append(out, lines[i])
		outSides = append(outSides, sides[i])
	}
	flush := func() {
		for _, want := range []side{leftSide, rightSide} {
			for _, i := range run {
				if sides[i] == want {
					take(i)
				}
			}
		}
		run = run[:0]
	}
	for i := range lines {
		if sides[i] == full {
			flush()
			take(i)
			continue
		}
		run = append(run, i)
	}
	flush()
	return margins(out, outSides), true
}

// margins gives each line the left and right edge of the column it is in, so
// that an indent, a centred line and an equation number can each be recognised
// by where they sit within it. With no sides given every line shares the page's.
func margins(lines []line, sides []side) []line {
	var bounds [3][2]float64
	for i := range bounds {
		bounds[i] = [2]float64{1e9, -1e9}
	}
	at := func(i int) side {
		if sides == nil {
			return full
		}
		return sides[i]
	}
	for i, l := range lines {
		s := at(i)
		bounds[s][0] = min(bounds[s][0], l.x0)
		bounds[s][1] = max(bounds[s][1], l.x1)
	}
	// A line set across the whole page is measured against the page rather
	// than against either column, so its bounds take in both.
	if sides != nil {
		for _, s := range []side{leftSide, rightSide} {
			bounds[full][0] = min(bounds[full][0], bounds[s][0])
			bounds[full][1] = max(bounds[full][1], bounds[s][1])
		}
	}
	for i := range lines {
		s := at(i)
		lines[i].left, lines[i].right = bounds[s][0], bounds[s][1]
	}
	return lines
}
