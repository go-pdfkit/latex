// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-pdfkit/extract"
)

// This file decides what each line of the page was, and writes it out.
//
// A PDF has no paragraphs. It has lines, and the paragraph has to be inferred
// from three things TeX leaves behind: the first line of one is indented, the
// last line of one is short, and a document that uses vertical space instead of
// an indent leaves a wider gap. The first two are only usable together with a
// check that the document is justified at all — in ragged-right text every line
// is short and the "last line" test fires on all of them, which is why it is
// measured over the whole document before it is trusted on any one line.
//
// A heading is a line set larger or bolder than the body and short enough not to
// be a line of it. Where the author numbered their sections the number is still
// there at the front of the line, and it says the level exactly — "3.2.1" is a
// \subsubsection and nothing else — which is far better evidence than the size.
// Where they did not, the sizes are ranked across the document and the ranking
// gives the level.

// A frame is one page, ready to be written out.
type frame struct {
	lines     []line
	rules     []rule
	images    []extract.Image
	twoColumn bool
}

// An emitter writes the body of the reconstructed document.
type emitter struct {
	opt   Options
	out   strings.Builder
	files []File
	// body is the size the document's running text is set at, and levels
	// the distinct sizes its headings use, largest first.
	body   float64
	levels []float64
	// justified says whether the document's lines reach the right margin,
	// which is what makes a short line evidence of a paragraph ending.
	justified bool
	// leading is how far apart the document set its lines.
	leading float64
	// para is the paragraph being built and prev the line it last took.
	para    []string
	prev    line
	hasPrev bool
	// paraRight is how far right the paragraph being built has reached,
	// which is what a short line is short against.
	paraRight float64
	// centred says the paragraph being built is set in the middle of its
	// column, so that a block of centred lines comes out as one \begin{center}
	// rather than as one per line.
	centred bool
	images  int
	title   string
	// opening says the emitter is still on the first page, which is the
	// only place a title can be.
	opening bool
}

// indentShare is how far a line must start beyond its column's margin to be the
// first of a paragraph, as a fraction of the body size. LaTeX's \parindent for a
// ten-point article is fifteen points, so half the body size is a wide margin
// for error either way.
const indentShare = 0.5

// paragraphGap is how much more than a line's own leading a vertical gap must
// be before it ends a paragraph on its own.
const paragraphGap = 1.7

// shortLine is how far short of the right margin a line must stop, in body
// sizes, to be the last of a paragraph.
const shortLine = 2.5

// centreShare is how near a column's centre a line's centre must be, as a
// fraction of the column's width, to be centred.
const centreShare = 0.06

// write puts one page into the document.
func (e *emitter) write(f frame) {
	for _, it := range interleave(f) {
		if it.image != nil {
			e.flush()
			e.figure(*it.image)
			continue
		}
		e.line(*it.line, f.rules)
	}
}

// An item is something to be written, from a page read top to bottom.
type item struct {
	line  *line
	image *extract.Image
	y     float64
}

// interleave puts a page's pictures among its lines, so that a figure comes out
// where it sat rather than all of them at the end.
func interleave(f frame) []item {
	items := make([]item, 0, len(f.lines)+len(f.images))
	for i := range f.lines {
		items = append(items, item{line: &f.lines[i], y: f.lines[i].y})
	}
	for i := range f.images {
		items = append(items, item{image: &f.images[i], y: f.images[i].Y + f.images[i].DrawnHeight})
	}
	// The lines are already in reading order, which on a two-column page is
	// not the order they sit in; sorting by height would undo that, so the
	// pictures are put in by position and the lines left where they are.
	sort.SliceStable(items, func(a, b int) bool {
		return items[a].image != nil && items[b].image == nil && items[a].y > items[b].y
	})
	return items
}

// line writes one line, as whatever it turns out to be.
func (e *emitter) line(l line, rs []rule) {
	segs := segments(l)
	if len(segs) == 0 {
		return
	}
	switch {
	case e.heading(l, segs):
		e.flush()
		e.section(l, segs)
	case display(l, segs):
		e.flush()
		e.display(l, segs, rs)
	default:
		e.text(l, segs, rs)
	}
	e.prev, e.hasPrev = l, true
}

// text adds a line to the paragraph being built, starting a new one where the
// evidence says the last ended.
func (e *emitter) text(l line, segs []segment, rs []rule) {
	if c := centred(l); c != e.centred && len(e.para) > 0 {
		e.flush()
		e.centred = c
	} else if len(e.para) == 0 {
		e.centred = c
	}
	if e.breaks(l) {
		e.flush()
	}
	e.paraRight = max(e.paraRight, l.x1)
	e.para = append(e.para, lineText(l, segs, rs, false))
}

// breaks reports whether a line starts a new paragraph.
func (e *emitter) breaks(l line) bool {
	if !e.hasPrev || len(e.para) == 0 {
		return false
	}
	if e.prev.y-l.y > paragraphGap*e.body {
		return true
	}
	// Neither of the horizontal tests means anything in a centred block,
	// where every line starts and stops somewhere different by design.
	if e.centred {
		return false
	}
	switch {
	case l.x0 > e.prev.x0+indentShare*e.body && l.x0 > l.left+indentShare*e.body:
		return true
	case e.justified && e.prev.x1 < e.paraRight-shortLine*e.body:
		return true
	}
	return false
}

// flush ends the paragraph being built.
func (e *emitter) flush() {
	if len(e.para) == 0 {
		return
	}
	s := mend(e.para)
	if e.centred {
		s = "\\begin{center}\n" + s + "\n\\end{center}"
	}
	e.paragraph(s)
	e.para = e.para[:0]
	e.paraRight = 0
}

// paragraph writes a finished paragraph.
func (e *emitter) paragraph(s string) {
	if strings.TrimSpace(s) == "" {
		return
	}
	e.out.WriteString(s)
	e.out.WriteString("\n\n")
}

// mend joins a paragraph's lines. A line that ends in a hyphen and is followed
// by a lowercase letter was broken by TeX rather than by the author, so the
// hyphen goes and the word is put back together; a hyphen followed by anything
// else was written.
func mend(parts []string) string {
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			s := b.String()
			switch {
			case !brokenWord(s):
				b.WriteByte(' ')
			case startsLower(p):
				// TeX put the hyphen in, so it comes out again.
				b.Reset()
				b.WriteString(s[:len(s)-1])
			}
		}
		b.WriteString(p)
	}
	return b.String()
}

// startsLower reports whether text begins with a lowercase letter.
func startsLower(s string) bool {
	for _, r := range s {
		return r >= 'a' && r <= 'z'
	}
	return false
}

// heading reports whether a line is a section heading.
func (e *emitter) heading(l line, segs []segment) bool {
	if allMath(segs) || l.x1-l.x0 > 0.9*(l.right-l.left) {
		return false
	}
	if l.size > e.body*1.08 {
		return true
	}
	return l.size > e.body*0.9 && allBold(l)
}

// section writes a heading at the level the document says it is.
func (e *emitter) section(l line, segs []segment) {
	text := strings.TrimSpace(lineText(l, segs, nil, true))
	number, rest := splitNumber(text)
	level := len(number)
	if level == 0 {
		level = e.rank(l.size)
	}
	if e.opening && e.title == "" && number == nil && centred(l) && l.size >= e.levels[0] {
		e.title = rest
		return
	}
	star := "*"
	if number != nil {
		star = ""
	}
	e.out.WriteString(sectionName(level) + star + "{" + rest + "}\n\n")
}

// sectionNames are the levels a heading may be written at. A document nested
// deeper than this is written at the deepest one rather than being given a level
// LaTeX has no command for.
var sectionNames = []string{`\section`, `\subsection`, `\subsubsection`, `\paragraph`}

// sectionName is the command for a level, counting from one.
func sectionName(level int) string {
	if level > len(sectionNames) {
		level = len(sectionNames)
	}
	return sectionNames[level-1]
}

// rank is the level a heading of a given size sits at, from where that size
// falls among all the heading sizes in the document.
func (e *emitter) rank(size float64) int {
	for i, s := range e.levels {
		if size > s*0.99 {
			return i + 1
		}
	}
	return len(e.levels)
}

// splitNumber takes a section number off the front of a heading and returns its
// parts and what is left. "3.2 Results" is a \subsection because the number has
// two parts, which is better evidence of the level than any measurement.
func splitNumber(s string) ([]string, string) {
	i := 0
	for i < len(s) && (isDigit(s[i]) || s[i] == '.' || (i == 0 && isASCIILetter(s[i]) && len(s) > 1 && s[1] == '.')) {
		i++
	}
	head := strings.TrimSuffix(s[:i], ".")
	rest := strings.TrimSpace(s[i:])
	if head == "" || rest == "" || !isDigit(head[0]) && !isASCIILetter(head[0]) {
		return nil, s
	}
	parts := strings.Split(head, ".")
	for _, p := range parts {
		if p == "" {
			return nil, s
		}
	}
	return parts, rest
}

// isDigit reports whether a byte is a decimal digit.
func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// display reports whether a line is a displayed equation: all mathematics, and
// set apart from the margin the way TeX sets a display.
func display(l line, segs []segment) bool {
	if !allMath(trimNumber(segs)) {
		return false
	}
	width := l.right - l.left
	return centred(l) || l.x0 > l.left+0.06*width || len(trimNumber(segs)) < len(segs)
}

// display writes a displayed equation, in the numbered environment when the
// document numbered it.
func (e *emitter) display(l line, segs []segment, rs []rule) {
	kept := trimNumber(segs)
	body := displayBody(kept, rs)
	if body == "" {
		return
	}
	if len(kept) < len(segs) {
		e.out.WriteString("\\begin{equation}\n" + body + "\n\\end{equation}\n\n")
		return
	}
	e.out.WriteString("\\[\n" + body + "\n\\]\n\n")
}

// trimNumber drops an equation number from the end of a line. TeX sets it at the
// right margin in the roman font, in parentheses, and it is not part of the
// equation — writing it back would give the equation two numbers.
func trimNumber(segs []segment) []segment {
	if len(segs) < 2 {
		return segs
	}
	last := segs[len(segs)-1]
	if last.math || len(last.atoms) != 1 {
		return segs
	}
	a := last.atoms[0]
	l := a.text
	if len(l) < 3 || l[0] != '(' || l[len(l)-1] != ')' {
		return segs
	}
	return segs[:len(segs)-1]
}

// allMath reports whether every piece of a line is mathematics.
func allMath(segs []segment) bool {
	for _, s := range segs {
		if !s.math {
			return false
		}
	}
	return len(segs) > 0
}

// allBold reports whether every piece of a line is set bold.
func allBold(l line) bool {
	for _, a := range l.atoms {
		if !a.sh.bold {
			return false
		}
	}
	return len(l.atoms) > 0
}

// centred reports whether a line sits in the middle of its column with space at
// both ends.
func centred(l line) bool {
	width := l.right - l.left
	if width <= 0 {
		return false
	}
	return abs((l.x0+l.x1)/2-(l.left+l.right)/2) < centreShare*width &&
		l.x0 > l.left+centreShare*width
}

// figure writes a picture, and keeps the file it needs.
func (e *emitter) figure(im extract.Image) {
	s, f := figure(im, e.images)
	if s == "" {
		return
	}
	e.images++
	if f != nil {
		e.files = append(e.files, *f)
	}
	e.paragraph("\\begin{center}\n" + s + "\n\\end{center}")
}

// lineText writes one line: its equations between dollars and its text with the
// markup its fonts call for.
func lineText(l line, segs []segment, rs []rule, bare bool) string {
	var b strings.Builder
	for i, s := range segs {
		text := s.source(rs, bare)
		if text == "" {
			continue
		}
		if i > 0 && gapBetween(segs[i-1], s) {
			b.WriteByte(' ')
		}
		b.WriteString(text)
	}
	return b.String()
}

// source is one piece of a line written out. An equation with nothing left in
// it - every glyph of it was an accent this could not place - is written as
// nothing rather than as an empty pair of dollars, which is not an empty
// equation but the start of a displayed one.
func (s segment) source(rs []rule, bare bool) string {
	if !s.math {
		return runText(s.atoms, bare)
	}
	if m := mathSource(s.atoms, mathRules(s.atoms, rs)); m != "" {
		return "$" + m + "$"
	}
	return ""
}

// displayBody is a displayed equation written without the dollars, since the
// environment around it opens the mathematics itself. A line that is mostly an
// equation but carries a word or two comes back with those words in \text,
// which is what an author writes for them.
func displayBody(segs []segment, rs []rule) string {
	var parts []string
	for _, s := range segs {
		if s.math {
			if m := mathSource(s.atoms, mathRules(s.atoms, rs)); m != "" {
				parts = append(parts, m)
			}
			continue
		}
		if t := strings.TrimSpace(runText(s.atoms, false)); t != "" {
			parts = append(parts, `\text{`+t+`}`)
		}
	}
	return strings.Join(parts, " ")
}

// gapBetween reports whether two neighbouring pieces of a line have a word space
// between them.
func gapBetween(left, right segment) bool {
	a := left.atoms[len(left.atoms)-1]
	b := right.atoms[0]
	space := max(a.wordSpace(), b.wordSpace())
	if space == 0 {
		return false
	}
	return b.x-a.right() > wordGap*space
}

// runText writes a stretch of ordinary text, putting the word spaces back and
// wrapping each run of one style in the command that produced it. The space
// between two words of different styles goes between the two commands rather
// than inside either: \\textbf{bold} words, never \\textbf{bold } words, which
// sets a trailing space in bold and is not what the author wrote.
//
// bare suppresses the markup, for a heading: \\section already sets its argument
// bold, and wrapping it in \\textbf as well would double it.
func runText(atoms []atom, bare bool) string {
	var b, run strings.Builder
	cur := atoms[0].sh.textual()
	flush := func() {
		if bare {
			b.WriteString(run.String())
			run.Reset()
			return
		}
		b.WriteString(markup(cur, run.String()))
		run.Reset()
	}
	for i, a := range atoms {
		if i > 0 {
			prev := atoms[i-1]
			gap := false
			if space := max(prev.wordSpace(), a.wordSpace()); space > 0 {
				gap = a.x-prev.right() > wordGap*space
			}
			if a.sh.textual() != cur {
				flush()
				cur = a.sh.textual()
				if gap {
					b.WriteByte(' ')
					gap = false
				}
			}
			if gap {
				run.WriteByte(' ')
			}
		}
		run.WriteString(escapeText(a.text))
	}
	flush()
	return b.String()
}

// measure works out, over the whole document, the size its running text is set
// at, the sizes its headings use, and whether its lines reach the right margin.
func (e *emitter) measure(frames []frame) {
	weight := map[int]float64{}
	reach, total := 0, 0
	for _, f := range frames {
		for _, l := range f.lines {
			for _, a := range l.atoms {
				weight[int(a.size*20+0.5)] += a.width
			}
		}
	}
	e.body = heaviest(weight)
	for _, f := range frames {
		for _, l := range f.lines {
			if l.size > e.body*1.08 || (l.size > e.body*0.9 && allBold(l)) {
				e.levels = append(e.levels, l.size)
				continue
			}
			total++
			if l.x1 > l.right-0.5*e.body {
				reach++
			}
		}
	}
	e.justified = total > 0 && reach*10 > total*6
	e.levels = distinct(e.levels)
	e.leading = e.leadingOf(frames)
}

// heaviest is the size the most ink was set in, which is the body size: a
// document has more running text than anything else, by a wide margin.
func heaviest(weight map[int]float64) float64 {
	best, found := 0, -1.0
	for k, v := range weight {
		if v > found || (v == found && k < best) {
			best, found = k, v
		}
	}
	return float64(best) / 20
}

// distinct is the heading sizes, largest first, with sizes within a twentieth of
// each other counted as one level.
func distinct(sizes []float64) []float64 {
	sort.Float64s(sizes)
	var out []float64
	for i := len(sizes) - 1; i >= 0; i-- {
		if len(out) == 0 || sizes[i] < out[len(out)-1]*0.95 {
			out = append(out, sizes[i])
		}
	}
	if len(out) == 0 {
		out = []float64{1e9}
	}
	return out
}

// preamble is the document's opening, which says only what the reconstruction
// actually needs: the classes of symbol it may have written, the graphics it may
// have referred to, and the shape of the page it came off.
func preamble(opt Options, f frame, width, height, size, leading float64, title string) string {
	var b strings.Builder
	class := opt.Class
	if class == "" {
		class = "article"
	}
	options := ""
	if f.twoColumn {
		options = "[twocolumn]"
	}
	fmt.Fprintf(&b, "\\documentclass%s{%s}\n", options, class)
	b.WriteString("\\usepackage{amsmath}\n\\usepackage{amssymb}\n\\usepackage{graphicx}\n")
	l, r, t, bot := textBlock(f, width, height)
	fmt.Fprintf(&b, "\\usepackage[paperwidth=%.1fpt,paperheight=%.1fpt,"+
		"left=%.1fpt,right=%.1fpt,top=%.1fpt,bottom=%.1fpt]{geometry}\n",
		width, height, l, r, t, bot)
	if size > 0 && leading > 0 {
		// The size the document was actually set at, rather than the ten
		// point the class would otherwise choose. A paper whose body is nine
		// point set on eleven, put back as ten on twelve, drifts a point and
		// a half a line: by the twentieth line it is a whole line out, and
		// every comparison with the original page is then comparing text
		// against the gap between two other pieces of text.
		fmt.Fprintf(&b, "\\AtBeginDocument{\\fontsize{%.2fpt}{%.2fpt}\\selectfont}\n", size, leading)
	}
	if title != "" {
		fmt.Fprintf(&b, "\\title{%s}\n\\author{}\n\\date{}\n", title)
	}
	return b.String()
}

// textBlock is the four margins the page was set with, taken from where its
// text actually sits: how far in it starts on the left, how far the longest
// line stops from the right, and the same top and bottom. Setting all four is
// worth more than setting one, because a paper is rarely symmetrical and a
// reconstruction with the wrong measure rebreaks every line in it.
//
// A page with no text on it is given an inch all round, which is the default.
func textBlock(f frame, width, height float64) (left, right, top, bottom float64) {
	if len(f.lines) == 0 {
		return 72, 72, 72, 72
	}
	x0, x1 := width, 0.0
	y0, y1 := height, 0.0
	for _, l := range f.lines {
		x0, x1 = min(x0, l.x0), max(x1, l.x1)
		y0, y1 = min(y0, l.y), max(y1, l.y+l.size)
	}
	return x0, max(width-x1, 0), max(height-y1, 0), max(y0, 0)
}

// brokenWord reports whether a line ended in the middle of a word. TeX breaks a
// word across two lines with a single hyphen; a line that ends in a dash the
// author wrote ends in two or three of them, and one that ends in a hyphenated
// compound - Levi-Civita broken after the Levi - ends in one as well, which is
// why the hyphen itself is only removed when what follows is lowercase.
func brokenWord(s string) bool {
	return strings.HasSuffix(s, "-") && !strings.HasSuffix(s, "--")
}

// The bounds a believable leading falls within, as a multiple of the body size.
// TeX sets a ten-point document on twelve-point leading; a paper set tighter
// than the size itself, or looser than double, has a gap between paragraphs or
// a figure in the way rather than a leading.
const (
	minLeading = 0.9
	maxLeading = 2.0
)

// leadingOf is how far apart the document sets its lines, which together with
// the body size is what decides where every line of the reconstruction lands.
//
// It is worth setting. A paper whose body is nine point set on eleven, put back
// as LaTeX's ten on twelve, drifts a point and a half a line: by the twentieth
// line it is a whole line out, and every measurement that compares the
// reconstruction with the original page compares text against the gap between
// two other pieces of text. Zero means the document did not say, and the class
// decides.
func (e *emitter) leadingOf(frames []frame) float64 {
	var gaps []float64
	for _, f := range frames {
		for i := 1; i < len(f.lines); i++ {
			a, b := f.lines[i-1], f.lines[i]
			if a.size < e.body*0.98 || b.size < e.body*0.98 || a.left != b.left {
				continue
			}
			if d := a.y - b.y; d > minLeading*e.body && d < maxLeading*e.body {
				gaps = append(gaps, d)
			}
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	sort.Float64s(gaps)
	return gaps[len(gaps)/2]
}
