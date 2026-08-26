// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import "unicode"

// This file decides where an equation starts and stops, which is a question the
// page does not answer.
//
// TeX sets the $ and the text around it in the same roman font, so the boundary
// leaves no trace. What it does leave is the three math families: a glyph from
// the math italic, symbol or extension family is inside an equation and nothing
// else is. Those are the seeds. Around each one the equation is grown outwards
// over the glyphs that the roman font also drew but that belong to the maths Ã¢ÂÂ
// the digits, the + and the =, the parentheses Ã¢ÂÂ recognised by two things
// together: they are the sort of character an equation contains, and they are
// close enough to their neighbour that TeX did not put a word space there.
//
// Both halves matter. Without the character test, "and" gets eaten whenever it
// happens to sit near an equation; without the spacing test, the "2" of
// "section 2" joins the equation on the same line.

// wordGap is how wide a gap must be, as a fraction of the font's own space, to
// be a word break rather than kerning. TeX kerns by a point or two and spaces by
// three or four, so a third of a space separates them cleanly.
const wordGap = 0.35

// mathGap is how far an equation may reach over a gap to take in a character
// its own font did not draw, as a fraction of the size it is set at. TeX’s
// widest ordinary inter-atom space is five eighteenths of an em on each side of
// a relation, so a shade under half an em covers both.
const mathGap = 0.45

// A segment is a stretch of one line that is either all mathematics or all
// text.
type segment struct {
	atoms []atom
	math  bool
}

// segments splits a line into its equations and the text between them.
func segments(l line) []segment {
	if len(l.atoms) == 0 {
		return nil
	}
	inMath := grow(l)
	var out []segment
	for i := 0; i < len(l.atoms); {
		j := i
		for j < len(l.atoms) && inMath[j] == inMath[i] {
			j++
		}
		out = append(out, segment{atoms: l.atoms[i:j], math: inMath[i]})
		i = j
	}
	return out
}

// grow marks which of a line's glyphs are inside an equation: the ones drawn in
// a math family, the single italic letters standing among roman, and the ones
// the growth reaches from them.
func grow(l line) []bool {
	in := make([]bool, len(l.atoms))
	any := false
	for i, a := range l.atoms {
		if a.sh.isMath() || italicVariable(l.atoms, i) {
			in[i], any = true, true
		}
	}
	if !any {
		return in
	}
	for i := range in {
		if !in[i] {
			continue
		}
		for j := i - 1; j >= 0 && !in[j] && absorbs(l.atoms[j], l.atoms[j+1], l.atoms[j]); j-- {
			in[j] = true
		}
		for j := i + 1; j < len(l.atoms) && !in[j] && absorbs(l.atoms[j-1], l.atoms[j], l.atoms[j]); j++ {
			in[j] = true
		}
	}
	return dropTrailing(l.atoms, in)
}

// dropTrailing takes the punctuation off the end of each equation. TeX leaves no
// space between $x$ and the full stop after it, so the growth reaches over the
// gap every time; a comma inside an equation is never the last thing in it,
// which is what tells the two apart.
func dropTrailing(atoms []atom, in []bool) []bool {
	for i := len(in) - 1; i >= 0; i-- {
		if !in[i] || (i+1 < len(in) && in[i+1]) {
			continue
		}
		for j := i; j >= 0 && in[j] && trailing(atoms[j]); j-- {
			in[j] = false
		}
	}
	return in
}

// absorbs reports whether an equation reaches over the gap between two
// neighbouring glyphs to take in outer, the one that is not yet part of it.
func absorbs(left, right, outer atom) bool {
	if !mathLike(outer) {
		return false
	}
	gap := right.x - left.right()
	// TeX puts a thin, medium or thick space around the operators inside an
	// equation - up to five eighteenths of an em - which is wider than the
	// kerning a word space has to be told from, so the reach here is measured
	// against the font size rather than against the space. What keeps a word
	// of prose out is mathLike, not the distance.
	return gap < mathGap*max(left.size, right.size) ||
		gap < wordGap*max(left.wordSpace(), right.wordSpace())
}

// mathLike reports whether a piece of text drawn in a text font is the sort an
// equation contains. A single letter is a variable set upright; two or more are
// an operator name; digits and the arithmetic characters are themselves. A word
// of ordinary prose is none of those.
func mathLike(a atom) bool {
	letters := 0
	for _, r := range a.text {
		switch {
		case unicode.IsLetter(r):
			letters++
		case mathPlain(r) || r == ' ':
		default:
			return false
		}
	}
	if letters == 0 {
		return true
	}
	return letters == len([]rune(a.text)) &&
		(letters == 1 || functionName[a.text])
}

// trailing reports whether a glyph is the punctuation that follows an equation
// rather than part of it. TeX leaves no space between $x$ and the full stop
// after it, so without this test every equation at the end of a sentence
// swallows the sentence's punctuation.
func trailing(a atom) bool {
	if a.sh.isMath() {
		return false
	}
	for _, r := range a.text {
		if r != '.' && r != ',' && r != ';' && r != ':' {
			return false
		}
	}
	return len(a.text) > 0
}

// italicVariable reports whether a single italic letter standing among roman
// text is a mathematical variable.
//
// It is here because one widely used font package makes the font names useless
// for this. Fourier sets its Greek and its operators in Fourier-Math-Letters and
// Fourier-Math-Symbols, but it sets the ordinary math LETTERS in Utopia-Italic —
// the same face the text italic uses. A paper set in it has "(L, P)" drawn with
// the parentheses roman and the L and the P italic, and nothing in any font name
// says that this is an equation rather than two emphasised letters.
//
// What does say so is the arrangement. A letter set in italic, alone, with roman
// on both sides of it, is a variable: emphasis applies to words, and a word is
// more than one letter. The neighbours have to be checked rather than only the
// letter, because a producer that emits one glyph per showing operator turns a
// whole italic sentence into single italic letters, and every one of them would
// otherwise qualify.
func italicVariable(atoms []atom, i int) bool {
	a := atoms[i]
	if a.sh.isMath() || !a.sh.italic {
		return false
	}
	rs := []rune(a.text)
	if len(rs) != 1 || !unicode.IsLetter(rs[0]) {
		return false
	}
	if i > 0 && atoms[i-1].sh.italic {
		return false
	}
	if i+1 < len(atoms) && atoms[i+1].sh.italic {
		return false
	}
	return true
}
