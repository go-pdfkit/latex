package latex

import "testing"

func TestRepairingAnEquationTeXWouldRefuse(t *testing.T) {
	cases := [][2]string{
		// A script with nothing to be the script of.
		{`^{2}`, `{}^{2}`},
		// A plus is an atom and may carry a script; that is legal TeX.
		{`+^{i}x`, `+^{i}x`},
		// A script at the head of a group has nothing before it.
		{`{^{i}}`, `{{}^{i}}`},
		// Two scripts of the same kind on one letter.
		{`x_{1}_{2}`, `x_{1}{}_{2}`},
		{`x^{1}^{2}`, `x^{1}{}^{2}`},
		// One of each is what a big operator has, and is left alone.
		{`x_{1}^{2}`, `x_{1}^{2}`},
		// A new letter carries its own scripts.
		{`x_{1}y_{2}`, `x_{1}y_{2}`},
		// A group that closes is something a script can attach to.
		{`{ab}^{2}`, `{ab}^{2}`},
		// Braces left open are closed, and braces closed too often dropped.
		{`\frac{a}{b`, `\frac{a}{b}`},
		{`a}b`, `ab`},
		// A command is something a script can attach to.
		{`\alpha^{2}`, `\alpha^{2}`},
		// A \left with no \right loses the growing and keeps the delimiter.
		{`\left(x`, `(x`},
		{`\left(x\right)`, `\left(x\right)`},
		// A control symbol is one character long.
		{`\{x^{2}`, `\{x^{2}`},
		// A space is not something a script can attach to.
		{` ^{2}`, ` {}^{2}`},
		// A backslash at the very end is left as it is.
		{`x\`, `x\`},
		{"", ""},
	}
	for _, c := range cases {
		if got := repair(c[0]); got != c[1] {
			t.Errorf("repair(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestARightArrowIsNotAnUnmatchedRight(t *testing.T) {
	// \rightarrow starts with the six characters of \right. Stripping an
	// unmatched \left by working on the text rather than on the tokens turns
	// every limit in the document into \thetaarrow, which is not a command.
	if got := repair(`\left(\theta\rightarrow0`); got != `(\theta\rightarrow0` {
		t.Errorf("got %q", got)
	}
	if got := repair(`\theta\rightarrow0`); got != `\theta\rightarrow0` {
		t.Errorf("got %q", got)
	}
}

func TestAPrimeIsASuperscript(t *testing.T) {
	// TeX reads x' as x^{\prime}, so a subscript after one lands on a letter
	// that already has a superscript.
	if got := repair(`q_{a}'_{b}`); got != `q_{a}'{}_{b}` {
		t.Errorf("got %q", got)
	}
	if got := repair(`q'`); got != `q'` {
		t.Errorf("got %q", got)
	}
}

func TestDelimitersThatDoNotNest(t *testing.T) {
	// One of each, but the \right is in a group of its own: TeX reads each
	// group on its own and refuses it.
	if got := repair(`\frac{\left(a}{b\right)}`); got != `\frac{(a}{b)}` {
		t.Errorf("got %q", got)
	}
	// A \right before its \left.
	if got := repair(`a\right)b\left(`); got != `a)b(` {
		t.Errorf("got %q", got)
	}
	// A brace that closes with nothing open is dropped before the pairing is
	// judged, so the pairing sees what TeX will see.
	if got := repair(`\left(a\right)`); got != `\left(a\right)` {
		t.Errorf("got %q", got)
	}
	if got := repair(`{\left(a\right)}`); got != `{\left(a\right)}` {
		t.Errorf("got %q", got)
	}
	if got := repair(`}\left(a\right)`); got != `(a)` {
		t.Errorf("got %q", got)
	}
}
