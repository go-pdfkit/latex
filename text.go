// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import "strings"

// This file turns text back into source: the characters a document drew, put
// back into the form that would draw them again, and the font changes along a
// line put back into the commands that caused them.
//
// The escaping is the ordinary LaTeX list. The interesting part is the other
// direction — a run of characters set in CMBX10 was written \textbf{…}, and to
// say so the runs of one style have to be found and wrapped. That is done at
// the line level rather than the word level, because a style that changes for
// one word and back is one \textbf and not three.

// specials are the characters LaTeX reads as instructions, and what has to be
// written to get the character itself.
var specials = map[rune]string{
	'#': `\#`, '$': `\$`, '%': `\%`, '&': `\&`, '_': `\_`,
	'{': `\{`, '}': `\}`,
	'~': `\textasciitilde{}`, '^': `\textasciicircum{}`,
	'\\': `\textbackslash{}`,
	// The characters a text font draws for a ligature or a dash, put back
	// into what an author types for them.
	'ﬀ': `ff`, 'ﬁ': `fi`, 'ﬂ': `fl`, 'ﬃ': `ffi`, 'ﬄ': `ffl`,
	'–': `--`, '—': `---`, '‘': "`", '’': `'`,
	'“': "``", '”': `''`, '−': `-`,
	'…': `\ldots{}`, '¡': `!`, '¿': `?`,
	'°': `\textdegree{}`, '©': `\textcopyright{}`, '§': `\S{}`,
	'¶': `\P{}`, '†': `\dag{}`, '‡': `\ddag{}`, '•': `\textbullet{}`,
	'£': `\pounds{}`, '€': `\texteuro{}`, '×': `\texttimes{}`,
}

// escapeText writes text so that LaTeX draws it rather than obeying it.
//
// A character that only mathematics has a name for is written as
// mathematics. This happens more often than it should: a symbol the
// reconstruction did not manage to bring inside an equation still has to
// come out as something that compiles, and \ensuremath{\partial} typesets
// where a bare U+2202 does not - not in a document whose preamble this
// package wrote, which does not load an input encoding for it.
func escapeText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if rep, ok := specials[r]; ok {
			b.WriteString(rep)
			continue
		}
		if r > 0x7F {
			if cmd, ok := mathCommand[r]; ok {
				b.WriteString(`\ensuremath{` + cmd + `}`)
				continue
			}
			if decoration(r) {
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

// wrappers are the commands that put a style back, in the order they are
// applied when a piece of text has more than one. A face that is both bold and
// italic becomes \textbf{\emph{…}}.
type wrapper struct {
	on   func(shape) bool
	name string
}

var wrappers = []wrapper{
	{func(s shape) bool { return s.mono }, `\texttt`},
	{func(s shape) bool { return s.sans }, `\textsf`},
	{func(s shape) bool { return s.smallCaps }, `\textsc`},
	{func(s shape) bool { return s.bold }, `\textbf`},
	{func(s shape) bool { return s.italic }, `\emph`},
}

// markup wraps text in whatever commands its font calls for. Plain roman gets
// nothing, which is most of a document.
func markup(s shape, text string) string {
	for i := len(wrappers) - 1; i >= 0; i-- {
		if wrappers[i].on(s) {
			text = wrappers[i].name + `{` + text + `}`
		}
	}
	return text
}
