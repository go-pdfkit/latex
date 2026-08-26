// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import "unicode"

// This file is the dictionary between what a glyph turned into and what the
// author wrote. A PDF says the character is U+03B1; the source said \alpha.
// There is no way to derive one from the other, so it is written out.
//
// The characters come from the font's /ToUnicode map, or failing that from the
// glyph names in its built-in encoding — either way that work is already done
// by github.com/go-pdfkit/pdffont, and what arrives here is a rune. Some of
// these are unambiguous (U+2211 is \sum and nothing else); some are a choice
// this makes and should say so. U+2212 is emitted as a plain minus because that
// is what TeX's - means in maths, and U+00D7 as \times rather than \x. Where a
// symbol has an AMS spelling and a plain one, the plain one wins, so that the
// output needs amsmath and amssymb but not more.

// mathCommand is what a rune is written as inside mathematics. A rune that is
// not here is emitted as itself, which is right for the letters, the digits and
// the handful of characters TeX takes literally.
var mathCommand = map[rune]string{
	// Lowercase Greek, which TeX sets from the math italic family.
	'α': `\alpha`, 'β': `\beta`, 'γ': `\gamma`, 'δ': `\delta`,
	'ε': `\epsilon`, 'ζ': `\zeta`, 'η': `\eta`, 'θ': `\theta`,
	'ι': `\iota`, 'κ': `\kappa`, 'λ': `\lambda`, 'μ': `\mu`,
	'ν': `\nu`, 'ξ': `\xi`, 'π': `\pi`, 'ρ': `\rho`,
	'σ': `\sigma`, 'τ': `\tau`, 'υ': `\upsilon`, 'φ': `\phi`,
	'χ': `\chi`, 'ψ': `\psi`, 'ω': `\omega`,
	'ϑ': `\vartheta`, 'ϕ': `\varphi`, 'ϖ': `\varpi`, 'ϱ': `\varrho`,
	'ς': `\varsigma`, 'ϵ': `\varepsilon`, 'ϝ': `\digamma`,
	// Uppercase Greek. Only the ones TeX has a command for: the rest are
	// Latin capitals in the roman font and come through as themselves.
	'Γ': `\Gamma`, 'Δ': `\Delta`, 'Θ': `\Theta`, 'Λ': `\Lambda`,
	'Ξ': `\Xi`, 'Π': `\Pi`, 'Σ': `\Sigma`, 'Υ': `\Upsilon`,
	'Φ': `\Phi`, 'Ψ': `\Psi`, 'Ω': `\Omega`,
	// Relations.
	'≤': `\leq`, '≥': `\geq`, '≠': `\neq`, '≈': `\approx`,
	'≡': `\equiv`, '∼': `\sim`, '≃': `\simeq`, '≅': `\cong`,
	'∝': `\propto`, '≪': `\ll`, '≫': `\gg`, '≺': `\prec`, '≻': `\succ`,
	'⪯': `\preceq`, '⪰': `\succeq`, '≍': `\asymp`, '≐': `\doteq`,
	'⊥': `\perp`, '∥': `\parallel`, '∣': `\mid`, '⊢': `\vdash`, '⊣': `\dashv`,
	'⊨': `\models`, '≜': `\triangleq`, '≔': `:=`,
	// Set relations.
	'∈': `\in`, '∉': `\notin`, '∋': `\ni`, '⊂': `\subset`, '⊃': `\supset`,
	'⊆': `\subseteq`, '⊇': `\supseteq`, '∅': `\emptyset`,
	// Binary operators.
	'±': `\pm`, '∓': `\mp`, '×': `\times`, '÷': `\div`, '∗': `\ast`,
	'⋆': `\star`, '∘': `\circ`, '∙': `\bullet`, '·': `\cdot`, '⋅': `\cdot`,
	'∪': `\cup`, '∩': `\cap`, '⊎': `\uplus`, '⊓': `\sqcap`, '⊔': `\sqcup`,
	'∨': `\vee`, '∧': `\wedge`, '∖': `\setminus`, '≀': `\wr`,
	'⊕': `\oplus`, '⊖': `\ominus`, '⊗': `\otimes`, '⊘': `\oslash`,
	'⊙': `\odot`, '†': `\dagger`, '‡': `\ddagger`, '⨿': `\amalg`,
	'△': `\bigtriangleup`, '▽': `\bigtriangledown`, '◁': `\triangleleft`,
	'▷': `\triangleright`,
	// Arrows.
	'←': `\leftarrow`, '→': `\rightarrow`, '↔': `\leftrightarrow`,
	'⇐': `\Leftarrow`, '⇒': `\Rightarrow`, '⇔': `\Leftrightarrow`,
	'↑': `\uparrow`, '↓': `\downarrow`, '↕': `\updownarrow`,
	'⇑': `\Uparrow`, '⇓': `\Downarrow`, '⇕': `\Updownarrow`,
	'↦': `\mapsto`, '⟶': `\longrightarrow`, '⟵': `\longleftarrow`,
	'⟹': `\Longrightarrow`, '⟸': `\Longleftarrow`, '⟷': `\longleftrightarrow`,
	'↩': `\hookleftarrow`, '↪': `\hookrightarrow`, '⇀': `\rightharpoonup`,
	'⇁': `\rightharpoondown`, '↼': `\leftharpoonup`, '↽': `\leftharpoondown`,
	'↗': `\nearrow`, '↘': `\searrow`, '↙': `\swarrow`, '↖': `\nwarrow`,
	// Miscellaneous.
	'∞': `\infty`, '∂': `\partial`, '∇': `\nabla`, '∀': `\forall`,
	'∃': `\exists`, '¬': `\neg`, '∠': `\angle`, '□': `\Box`,
	'ℓ': `\ell`, 'ℏ': `\hbar`, 'ℜ': `\Re`, 'ℑ': `\Im`, '℘': `\wp`,
	'ℵ': `\aleph`, '′': `'`, '″': `''`, '‴': `'''`,
	'…': `\ldots`, '⋯': `\cdots`, '⋮': `\vdots`, '⋱': `\ddots`,
	'♭': `\flat`, '♮': `\natural`, '♯': `\sharp`, '♣': `\clubsuit`,
	'♦': `\diamondsuit`, '♥': `\heartsuit`, '♠': `\spadesuit`,
	'✓': `\checkmark`, '∴': `\therefore`, '∵': `\because`,
	// A degree sign is a superscript, and a superscript needs something to
	// be the superscript of.
	'\u00B0': `{}^\circ`, '\u00AF': `-`, '\u2016': `\|`,
	// The big operators, which take limits.
	'∑': `\sum`, '∏': `\prod`, '∐': `\coprod`, '∫': `\int`,
	'∮': `\oint`, '⋃': `\bigcup`, '⋂': `\bigcap`, '⨁': `\bigoplus`,
	'⨂': `\bigotimes`, '⨀': `\bigodot`, '⋁': `\bigvee`, '⋀': `\bigwedge`,
	'⨆': `\bigsqcup`, '∬': `\iint`, '∭': `\iiint`,
	// Delimiters.
	'⟨': `\langle`, '⟩': `\rangle`, '⌈': `\lceil`, '⌉': `\rceil`,
	'\u230A': `\lfloor`, '\u230B': `\rfloor`,
	// A radical sign this could not find a bar for has no radicand either,
	// and \sqrt with nothing after it does not compile: \surd is the sign
	// on its own, and \sqrt{...} is written out by the radical rule.
	'\u221A': `\surd`,
	// A minus sign is TeX's plain hyphen in maths, and a dash inside an
	// equation is always one.
	'\u2212': `-`, '\u2013': `-`, '\u2014': `-`,
	// Characters Unicode has more than one code for. A font's map answers
	// with whichever its maker chose, and an equation set in one of them
	// would otherwise come back with a raw character in it.
	'\u2126': `\Omega`, '\u00B5': `\mu`, '\u2206': `\Delta`,
	'\u2019': `'`,
}

// bigOperator reports whether a rune is one of the operators whose scripts
// become limits. LaTeX places them itself, so the only thing that matters here
// is knowing that a script sitting under such an operator rather than after it
// is still a subscript.
var bigOperator = map[rune]bool{
	'∑': true, '∏': true, '∐': true, '∫': true, '∮': true,
	'⋃': true, '⋂': true, '⨁': true, '⨂': true, '⨀': true,
	'⋁': true, '⋀': true, '⨆': true, '∬': true, '∭': true,
}

// openDelimiter is what a rune is written as after \left.
var openDelimiter = map[rune]string{
	'(': `(`, '[': `[`, '{': `\{`, '⟨': `\langle`,
	'⌈': `\lceil`, '⌊': `\lfloor`, '|': `|`, '‖': `\|`,
	'↑': `\uparrow`, '⇑': `\Uparrow`, '/': `/`,
}

// closeDelimiter is what a rune is written as after \right.
var closeDelimiter = map[rune]string{
	')': `)`, ']': `]`, '}': `\}`, '⟩': `\rangle`,
	'⌉': `\rceil`, '⌋': `\rfloor`, '|': `|`, '‖': `\|`,
	'↓': `\downarrow`, '⇓': `\Downarrow`, '\\': `\backslash`,
}

// functionName is the set of operator names TeX has a command for. A run of
// roman letters inside an equation is one of these, or it is a word the author
// wrote with \mathrm; both are set upright, and the only way to tell them apart
// is this list.
var functionName = map[string]bool{
	"arccos": true, "arcsin": true, "arctan": true, "arg": true,
	"cos": true, "cosh": true, "cot": true, "coth": true, "csc": true,
	"deg": true, "det": true, "dim": true, "exp": true, "gcd": true,
	"hom": true, "inf": true, "ker": true, "lg": true, "lim": true,
	"liminf": true, "limsup": true, "ln": true, "log": true, "max": true,
	"min": true, "mod": true, "Pr": true, "sec": true, "sin": true,
	"sinh": true, "sup": true, "tan": true, "tanh": true,
}

// mathPlain reports whether a rune drawn in a text font inside an equation may
// be written as itself. Digits, the arithmetic that lives in the roman font,
// and the punctuation TeX takes literally.
func mathPlain(r rune) bool {
	if r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case '+', '=', '<', '>', '(', ')', '[', ']', '/', '!', ':', ';',
		',', '.', '|', '\'', '-', '*', '?', '@':
		return true
	}
	return false
}

// decoration reports whether a character is a free-standing accent rather than
// something to set.
//
// A PDF draws \hat{L} as an L and a circumflex placed over it, two glyphs; the
// font's map calls the second one U+02C6, a modifier letter. There is no way to
// write that back as \hat without knowing which glyph it belongs to and how far
// the accent's own box was shifted, which this does not attempt. What it does
// do is leave the accent out rather than write a character into the source that
// no engine will set - the reconstruction loses the hat and keeps the L.
func decoration(r rune) bool {
	return unicode.In(r, unicode.Sk, unicode.Lm, unicode.Mn)
}
