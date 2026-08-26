// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import "strings"

// This file works out what a font was, from the only thing the page reliably
// says about it: its name. A PDF made by TeX does not record that a word was
// bold, or that a letter was a mathematical variable. It records that the word
// was drawn in CMBX10 and the letter in CMMI10 — and those names, which TeX has
// used unchanged since 1980, say exactly that. Every later family kept the
// convention or spelled it out in words (LMRoman10-Bold, NimbusRomNo9L-Medi),
// so the same reading works for them too.
//
// Nothing here looks inside the font program. A font's own idea of its weight
// is often wrong, and a subsetted face carries no useful flags at all; the name
// the document gives it is what the author's \textbf actually turned into.

// A shape is what a font says about the text drawn in it.
type shape struct {
	bold      bool
	italic    bool
	mono      bool
	sans      bool
	smallCaps bool
	// math is the part a math font plays, and mathNone for a text font. A
	// text font still appears inside mathematics — digits and parentheses
	// come from the roman face — which is why this cannot be the only test
	// for whether something is an equation.
	math mathRole
}

// A mathRole is the part a math font plays in an equation.
type mathRole int

const (
	// mathNone is a font that is not one of the math families.
	mathNone mathRole = iota
	// mathLetter is the math italic family (CMMI): variables, and the
	// lowercase Greek letters, which TeX treats as variables too.
	mathLetter
	// mathSymbol is the symbol family (CMSY, and the AMS extensions):
	// relations, operators, arrows, and the calligraphic capitals.
	mathSymbol
	// mathExt is the extension family (CMEX): the big operators, and the
	// delimiters grown to fit what they enclose.
	mathExt
)

// isMath reports whether the font is one of the math families.
func (s shape) isMath() bool { return s.math != mathNone }

// textual reports whether two pieces of text should be wrapped in the same
// markup. Only the text properties count: two math fonts differ in role all the
// time inside one equation and that is not a change of markup.
func (s shape) textual() shape {
	s.math = mathNone
	return s
}

// classify reads a font's name. What comes in is the /BaseFont of the font
// dictionary, which for an embedded subset carries a six-letter tag and a plus
// sign in front (FKPJSN+CMSY10) and for a Type0 font a suffix naming the
// encoding (LMRoman10-Bold-Identity-H). Both are cut away first.
func classify(base string) shape {
	n := strings.ToUpper(baseName(base))
	var s shape
	switch {
	case mathFamily(n, "ITAL", "LETTER", "MI"):
		s.math = mathLetter
	case mathFamily(n, "SYM", "SY"):
		s.math = mathSymbol
	case mathFamily(n, "EXT", "EX"):
		s.math = mathExt
	case hasAny(n, "CMMI", "CMMIB", "MATHITALIC", "MTMI", "RMMI", "TXMI", "PXMI", "EURM", "EUMI"):
		s.math = mathLetter
	case hasAny(n, "CMSY", "CMBSY", "MATHSYMBOL", "MSAM", "MSBM", "MTSY", "RMSY", "TXSY", "PXSY",
		"EUFM", "EUSM", "RSFS", "WASY", "BBOLD", "DSROM", "MNSYMBOL"):
		s.math = mathSymbol
	case hasAny(n, "CMEX", "MATHEXTENSION", "MTEX", "RMEX", "TXEX", "PXEX", "ESINT", "STMARY", "EUEX"):
		s.math = mathExt
	}
	if s.isMath() {
		// A math font is drawn slanted or upright as its family decides;
		// saying it is italic would put \emph round every variable.
		return s
	}
	s.mono = hasAny(n, "CMTT", "CMITT", "CMSLTT", "MONO", "TYPEWRITER", "COURIER", "NIMBUSMON")
	s.sans = !s.mono && hasAny(n, "CMSS", "SANS", "HELVETIC", "ARIAL", "NIMBUSSAN")
	s.smallCaps = hasAny(n, "CMCSC", "CAPS", "SMALLCAPS", "-SC", "SC10")
	s.bold = hasAny(n, "CMBX", "CMB10", "CMBXTI", "CMBXSL", "CMSSBX", "CMFIB", "BOLD", "-MEDI", "SEMIBOLD", "BLACK", "HEAVY")
	s.italic = hasAny(n, "CMTI", "CMSL", "CMITT", "CMSLTT", "CMBXTI", "CMBXSL", "CMFI", "ITALIC", "OBLIQUE", "SLANT")
	return s
}

// baseName strips a subset tag and a Type0 encoding suffix from a /BaseFont.
func baseName(base string) string {
	if i := strings.IndexByte(base, '+'); i >= 0 && i == 6 {
		base = base[i+1:]
	}
	for _, suffix := range []string{"-Identity-H", "-Identity-V", "-UniGB-UCS2-H", "-UniJIS-UCS2-H"} {
		base = strings.TrimSuffix(base, suffix)
	}
	return base
}

// hasAny reports whether the name contains any of the marks.
func hasAny(name string, marks ...string) bool {
	for _, m := range marks {
		if strings.Contains(name, m) {
			return true
		}
	}
	return false
}

// mathFamily reports whether a font's name says it is a math font of a
// particular kind: the word MATH together with a word naming the part it plays.
//
// This is what catches the families that are not descended from Computer
// Modern's naming. Fourier calls its three fonts Fourier-Math-Letters-Italic,
// Fourier-Math-Symbols and Fourier-Math-Extension, and a reader that knows only
// CMMI and CMSY sees a paper set in it as prose with a great many one-letter
// italic words in it. The word MATH on its own is not enough: a font called
// STIXTwoMath is a general-purpose face that one producer in this corpus sets
// entire documents in, running text included.
func mathFamily(name string, roles ...string) bool {
	return strings.Contains(name, "MATH") && hasAny(name, roles...)
}
