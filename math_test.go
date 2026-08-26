package latex

import "testing"

var (
	mi = shape{math: mathLetter}
	sy = shape{math: mathSymbol}
	ex = shape{math: mathExt}
)

func TestMaterialEitherSideOfAFraction(t *testing.T) {
	items := []atom{
		at("k", 10, 100, 5, 10, mi),
		at("a", 20, 104, 4, 7, mi),
		at("b", 20, 96, 4, 7, mi),
		at("m", 30, 100, 5, 10, mi),
	}
	got := mathSource(items, []rule{{19, 99.8, 24, 100.2}})
	if got != `k\frac{a}{b}m` {
		t.Errorf("got %q", got)
	}
}

func TestWhatIsNotAFractionBar(t *testing.T) {
	items := []atom{at("a", 20, 104, 4, 7, mi), at("b", 20, 96, 4, 7, mi)}
	// An upright rule divides columns, not a fraction.
	if got := mathSource(items, []rule{{21, 90, 21.4, 110}}); got != "ab" {
		t.Errorf("got %q", got)
	}
	// A rule with everything on one side of it is not a bar.
	if got := mathSource(items, []rule{{19, 90, 24, 90.4}}); got != "ab" {
		t.Errorf("got %q", got)
	}
	// A rule the glyphs sit well to one side of has nothing under it.
	side := []atom{at("a", 200, 104, 4, 7, mi), at("b", 200, 96, 4, 7, mi)}
	if got := mathSource(side, []rule{{19, 99.8, 24, 100.2}}); got != "ab" {
		t.Errorf("got %q", got)
	}
}

func TestARadicalWithNothingUnderIt(t *testing.T) {
	// A radical sign with no bar over it is a sign and nothing more.
	items := []atom{at("√", 20, 104, 8, 10, sy), at("x", 30, 100, 5, 10, mi)}
	if got := mathSource(items, nil); got != `\surd x` {
		t.Errorf("got %q", got)
	}
	// A bar that starts nowhere near the sign is not its bar.
	if got := mathSource(items, []rule{{90, 108, 120, 108.4}}); got != `\surd x` {
		t.Errorf("got %q", got)
	}
	// A bar below the sign is not its bar either.
	if got := mathSource(items, []rule{{28, 96, 40, 96.4}}); got != `\surd x` {
		t.Errorf("got %q", got)
	}
	// A bar with nothing under it covers no radicand.
	if got := mathSource(items, []rule{{28, 108, 29, 108.4}}); got != `\surd x` {
		t.Errorf("got %q", got)
	}
	// An upright rule beside the sign is not a bar.
	if got := mathSource(items, []rule{{28, 108, 28.4, 130}}); got != `\surd x` {
		t.Errorf("got %q", got)
	}
}

func TestAGroupOfNothingButExtensionGlyphs(t *testing.T) {
	// Every glyph is raised by construction, so the baseline has to be
	// taken from them after all rather than from anything standing on it.
	items := []atom{at("∑", 20, 105, 10, 10, ex), at("∏", 32, 105, 10, 10, ex)}
	if got := mathSource(items, nil); got != `\sum\prod` {
		t.Errorf("got %q", got)
	}
}

func TestWordsInsideAnEquation(t *testing.T) {
	// A run of upright letters TeX has a command for.
	items := []atom{at("log", 20, 100, 15, 10, shape{}), at("x", 36, 100, 5, 10, mi)}
	if got := mathSource(items, nil); got != `\log x` {
		t.Errorf("got %q", got)
	}
	// One it has not was set with \mathrm.
	items[0] = at("diag", 20, 100, 20, 10, shape{})
	if got := mathSource(items, nil); got != `\mathrm{diag}x` {
		t.Errorf("got %q", got)
	}
	// A run with something in it that is not a letter is not a name.
	items[0] = at("x2", 20, 100, 10, 10, shape{})
	if got := mathSource(items, nil); got != `x2x` {
		t.Errorf("got %q", got)
	}
}

func TestCharactersAnEquationCannotName(t *testing.T) {
	// A free-standing accent is left out: there is no way to say which
	// glyph it belonged to.
	if got := mathSource([]atom{at("x", 20, 100, 5, 10, mi), at("ˆ", 25, 100, 3, 10, sy)}, nil); got != "x" {
		t.Errorf("got %q", got)
	}
	// A character with no meaning in mathematics is escaped so that it
	// compiles rather than being written raw.
	if got := mathSource([]atom{at("%", 20, 100, 5, 10, sy)}, nil); got != `\%` {
		t.Errorf("got %q", got)
	}
}

func TestWhichRulesAnEquationMayUse(t *testing.T) {
	items := []atom{at("a", 20, 104, 4, 7, mi), at("b", 20, 96, 4, 7, mi)}
	in := []rule{
		{19, 99.8, 24, 100.2},
		{19.5, 99.9, 23, 100.1},
		// Far off to the right: another equation's.
		{300, 99.8, 340, 100.2},
	}
	got := mathRules(items, in)
	if len(got) != 2 || got[0].x0 != 19 {
		t.Errorf("got %+v", got)
	}
}

func TestAnEquationWithNothingLeftInIt(t *testing.T) {
	// Every glyph was an accent that could not be placed. Writing this as
	// $$ would not be an empty equation but the start of a displayed one.
	if got := mathSource([]atom{at("ˆ", 20, 100, 3, 10, sy)}, nil); got != "" {
		t.Errorf("got %q", got)
	}
	seg := segment{atoms: []atom{at("ˆ", 20, 100, 3, 10, sy)}, math: true}
	if got := lineText(line{}, []segment{seg, seg}, nil, false); got != "" {
		t.Errorf("got %q", got)
	}
	if got := displayBody([]segment{seg}, nil); got != "" {
		t.Errorf("got %q", got)
	}
	e := &emitter{}
	e.display(line{}, []segment{seg}, nil)
	if e.out.String() != "" {
		t.Errorf("got %q", e.out.String())
	}
}

func TestAWordInsideADisplayedEquation(t *testing.T) {
	segs := []segment{
		{atoms: []atom{at("x", 20, 100, 5, 10, mi)}, math: true},
		{atoms: []atom{at("for", 30, 100, 15, 10, shape{})}},
	}
	if got := displayBody(segs, nil); got != `x \text{for}` {
		t.Errorf("got %q", got)
	}
}
