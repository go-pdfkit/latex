package latex

import "testing"

func TestReadingWhatAFontSays(t *testing.T) {
	cases := []struct {
		base string
		want shape
	}{
		{"FKPJSN+CMR10", shape{}},
		{"CMBX12", shape{bold: true}},
		{"CMTI10", shape{italic: true}},
		{"CMBXTI10", shape{bold: true, italic: true}},
		{"CMTT10", shape{mono: true}},
		{"CMSS10", shape{sans: true}},
		{"CMCSC10", shape{smallCaps: true}},
		{"CMSL10", shape{italic: true}},
		{"LMRoman10-Bold-Identity-H", shape{bold: true}},
		{"LMRoman10-Italic-Identity-H", shape{italic: true}},
		{"LMMono10-Regular", shape{mono: true}},
		{"NimbusRomNo9L-Medi", shape{bold: true}},
		{"Helvetica-Oblique", shape{sans: true, italic: true}},
		{"Courier-Bold", shape{mono: true, bold: true}},
		{"CMMI10", shape{math: mathLetter}},
		{"CMSY7", shape{math: mathSymbol}},
		{"CMEX10", shape{math: mathExt}},
		{"MSBM10", shape{math: mathSymbol}},
		{"Fourier-Math-Letters-Italic", shape{math: mathLetter}},
		{"Fourier-Math-Symbols", shape{math: mathSymbol}},
		{"Fourier-Math-Extension", shape{math: mathExt}},
		{"LMMathItalic10-Regular", shape{math: mathLetter}},
		{"EUEX10", shape{math: mathExt}},
		{"EURM10", shape{math: mathLetter}},
		{"RSFS10", shape{math: mathSymbol}},
		// A general-purpose face with Math in its name is not a math family:
		// one producer sets whole documents in it, running text included.
		{"STIXTwoMath-Regular", shape{}},
		{"", shape{}},
		// A subset tag is only a tag when it is six letters and a plus.
		{"AB+CMBX10", shape{bold: true}},
	}
	for _, c := range cases {
		if got := classify(c.base); got != c.want {
			t.Errorf("%s reads as %+v, want %+v", c.base, got, c.want)
		}
	}
}

func TestAMathFontIsNeitherBoldNorItalic(t *testing.T) {
	// CMMI is a slanted face, but a variable is not emphasis; saying so would
	// put \emph round every letter of every equation.
	s := classify("CMMI10")
	if s.italic || s.bold || !s.isMath() {
		t.Errorf("CMMI10 reads as %+v", s)
	}
	if s.textual().math != mathNone {
		t.Error("textual() kept the math role")
	}
}

func TestStrippingAFontsDecoration(t *testing.T) {
	for _, c := range [][2]string{
		{"ABCDEF+CMR10", "CMR10"},
		{"CMR10", "CMR10"},
		{"ABC+CMR10", "ABC+CMR10"},
		{"X-Identity-V", "X"},
		{"Y-UniGB-UCS2-H", "Y"},
		{"Z-UniJIS-UCS2-H", "Z"},
	} {
		if got := baseName(c[0]); got != c[1] {
			t.Errorf("baseName(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}
