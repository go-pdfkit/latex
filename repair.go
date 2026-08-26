// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import "strings"

// This file makes sure that what comes out compiles.
//
// A reconstruction is worth nothing if TeX will not read it back, and the
// reconstruction of an equation is put together from geometry rather than from
// grammar, so it can end up saying things that no source ever would: a
// superscript with nothing to be the superscript of, because the glyph it
// belonged to was set in a font this could not read; two subscripts on one
// letter, because a script group was split by a rule; a brace left open,
// because the material a \frac needed ran off the end of the line; a \left with
// its \right on the next line.
//
// Every one of those aborts the compile, and aborting the compile loses the
// whole document rather than the one equation. So the last thing done to an
// equation is to read it back the way TeX will and repair what TeX would refuse.
// The repairs are the smallest ones that keep the meaning: an empty group in
// front of an orphaned script, an empty group between two scripts that would
// otherwise land on the same letter, the missing braces, and - for a delimiter
// that lost its partner - the plain character in place of the grown one.
//
// This was not written from first principles. It was written from a run over
// the corpus: of the first ninety-nine papers reconstructed, twenty-seven
// compiled. The four faults above are what the other seventy-two were.

// repair returns an equation that TeX will read, which is not always the one it
// was given.
func repair(s string) string {
	rs := []rune(s)
	// A \left with no \right does not compile, and there is nothing to pair
	// it with: the equation was cut in half by a line break. The delimiter
	// itself is still right, only its growing is lost.
	//
	// The counting has to be done on whole control words. \rightarrow starts
	// with the six characters of \right, and a pass that works on the text
	// rather than on the tokens turns every limit in the document into the
	// undefined command \thetaarrow.
	strip := count(rs, "left") != count(rs, "right")
	var b strings.Builder
	// levels is the stack of groups, each remembering whether anything in it
	// can carry a script and which scripts it has already been given.
	levels := []group{{}}
	depth := func() *group { return &levels[len(levels)-1] }
	for i := 0; i < len(rs); i++ {
		switch r := rs[i]; {
		case r == '{':
			levels = append(levels, group{})
			b.WriteRune(r)
		case r == '}':
			if len(levels) == 1 {
				// A closing brace with nothing open would end whatever
				// group the surrounding text is in.
				continue
			}
			levels = levels[:len(levels)-1]
			depth().base = true
			b.WriteRune(r)
		case r == '^' || r == '_' || r == '\'':
			// A prime is TeX's own shorthand for a superscript, and lands
			// on the same letter as one.
			kind := r
			if r == '\'' {
				kind = '^'
			}
			g := depth()
			if !g.base || g.taken(kind) {
				b.WriteString("{}")
				g.reset()
			}
			g.mark(kind)
			b.WriteRune(r)
		case r == '\\':
			word, next := controlWord(rs, i)
			if !strip || (word != "left" && word != "right") {
				b.WriteString(string(rs[i:next]))
				depth().start()
			}
			i = next - 1
		default:
			b.WriteRune(r)
			if r != ' ' {
				depth().start()
			}
		}
	}
	// Whatever is still open is closed, in the order it was opened.
	b.WriteString(strings.Repeat("}", len(levels)-1))
	return b.String()
}

// controlWord reads the command starting at the backslash at i: its name, and
// where it ends. A control symbol - a backslash and one character that is not a
// letter - has that character as its name.
func controlWord(rs []rune, i int) (string, int) {
	j := i + 1
	for j < len(rs) && isASCIILetter(byte(rs[j])) {
		j++
	}
	if j == i+1 && j < len(rs) {
		j++
	}
	return string(rs[i+1 : j]), j
}

// count is how many times a command appears, counting whole names only.
func count(rs []rune, name string) int {
	n := 0
	for i := 0; i < len(rs); i++ {
		if rs[i] != '\\' {
			continue
		}
		word, next := controlWord(rs, i)
		if word == name {
			n++
		}
		i = next - 1
	}
	return n
}

// A group is what is known about one level of braces: whether it holds anything
// a script could attach to, and which scripts have already attached.
type group struct {
	base bool
	sub  bool
	sup  bool
}

// taken reports whether this level already has a script of that kind.
func (g *group) taken(r rune) bool {
	if r == '_' {
		return g.sub
	}
	return g.sup
}

// mark records that a script of that kind has been given.
func (g *group) mark(r rune) {
	if r == '_' {
		g.sub = true
		return
	}
	g.sup = true
}

// start records something a script could attach to, which is also the end of
// whatever the last thing's scripts were.
func (g *group) start() {
	g.base, g.sub, g.sup = true, false, false
}

// reset forgets the scripts given so far, for a level that has just been handed
// an empty group to hang the next one from.
func (g *group) reset() {
	g.sub, g.sup = false, false
}
