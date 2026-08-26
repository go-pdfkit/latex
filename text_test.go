package latex

import "testing"

func TestWritingTextBackAsSource(t *testing.T) {
	cases := [][2]string{
		{"plain", "plain"},
		{"100%", `100\%`},
		{"a_b", `a\_b`},
		{"{x}", `\{x\}`},
		{"#$&", `\#\$\&`},
		{"~^", `\textasciitilde{}\textasciicircum{}`},
		{`\`, `\textbackslash{}`},
		{"ﬁne", "fine"},
		{"ﬀﬂﬃﬄ", "ffflffiffl"},
		{"– —", `-- ---`},
		{"‘a’", "`a'"},
		{"“a”", "``a''"},
		// A symbol only mathematics has a name for is written as mathematics
		// so that it compiles at all.
		{"∂", `\ensuremath{\partial}`},
		// A free-standing accent is dropped rather than written out.
		{"Lˆ", "L"},
		// A character with no special meaning is left alone.
		{"café", "café"},
	}
	for _, c := range cases {
		if got := escapeText(c[0]); got != c[1] {
			t.Errorf("escapeText(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestPuttingTheMarkupBack(t *testing.T) {
	cases := []struct {
		s    shape
		want string
	}{
		{shape{}, "x"},
		{shape{bold: true}, `\textbf{x}`},
		{shape{italic: true}, `\emph{x}`},
		{shape{mono: true}, `\texttt{x}`},
		{shape{sans: true}, `\textsf{x}`},
		{shape{smallCaps: true}, `\textsc{x}`},
		{shape{bold: true, italic: true}, `\textbf{\emph{x}}`},
		{shape{sans: true, bold: true}, `\textsf{\textbf{x}}`},
	}
	for _, c := range cases {
		if got := markup(c.s, "x"); got != c.want {
			t.Errorf("markup(%+v) = %q, want %q", c.s, got, c.want)
		}
	}
}
