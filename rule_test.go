package latex

import (
	"testing"

	"github.com/go-pdfkit/reader"
)

// marks is what a content stream draws, read back as straight marks.
func marks(t *testing.T, content string) []rule {
	t.Helper()
	d := pageWith(t, content, nil)
	return rules(d, 1, point{0, 0})
}

func TestAFilledRectangleIsAMark(t *testing.T) {
	got := marks(t, "10 20 30 2 re f")
	if len(got) != 1 || got[0] != (rule{10, 20, 40, 22}) {
		t.Fatalf("got %+v", got)
	}
	if !got[0].wide() || got[0].midY() != 21 || got[0].midX() != 25 {
		t.Errorf("got %+v", got[0])
	}
	if !got[0].spans(9.5, 1) || got[0].spans(5, 1) {
		t.Error("spans is wrong")
	}
	// A rectangle written the other way round is the same mark.
	if got := marks(t, "40 22 -30 -2 re f"); len(got) != 1 || got[0] != (rule{10, 20, 40, 22}) {
		t.Errorf("got %+v", got)
	}
	// A tall rectangle is a column divider rather than a rule across.
	if got := marks(t, "10 20 2 30 re f"); len(got) != 1 || got[0].wide() {
		t.Errorf("got %+v", got)
	}
}

func TestAStrokedLineIsAMark(t *testing.T) {
	// xdvipdfmx, which is what XeTeX writes through, draws a rule as a
	// stroked segment whose thickness is the line width.
	got := marks(t, "1 w 10 20 m 40 20 l S")
	if len(got) != 1 || got[0] != (rule{10, 19.5, 40, 20.5}) {
		t.Fatalf("got %+v", got)
	}
	// A vertical one.
	if got := marks(t, "2 w 10 20 m 10 50 l S"); len(got) != 1 || got[0].wide() {
		t.Errorf("got %+v", got)
	}
	// A width of nought is the thinnest the device can draw, not nothing.
	got = marks(t, "0 w 10 20 m 40 20 l S")
	if len(got) != 1 || abs(got[0].y1-got[0].y0-defaultLineWidth) > 1e-9 {
		t.Errorf("got %+v", got)
	}
	// A line at an angle is part of a drawing, not a rule.
	if got := marks(t, "1 w 10 20 m 40 60 l S"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
	// A stroked rectangle is kept as it stands.
	if got := marks(t, "10 20 30 2 re S"); len(got) != 1 {
		t.Errorf("got %+v", got)
	}
	// A path that is only closed and stroked draws its segments too.
	if got := marks(t, "10 20 m 40 20 l s"); len(got) != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestTheTransformMovesAMark(t *testing.T) {
	got := marks(t, "q 2 0 0 2 5 5 cm 10 20 30 2 re f Q 0 0 1 1 re f")
	if len(got) != 2 || got[0] != (rule{25, 45, 85, 49}) {
		t.Fatalf("got %+v", got)
	}
	// The transform was put back by Q, so the second mark is where it says.
	if got[1] != (rule{0, 0, 1, 1}) {
		t.Errorf("got %+v", got[1])
	}
	// A Q with nothing pushed is ignored rather than fatal.
	if got := marks(t, "Q 0 0 1 1 re f"); len(got) != 1 {
		t.Errorf("got %+v", got)
	}
	// The page's own origin is taken off.
	d := pageWith(t, "10 20 30 2 re f", nil)
	if got := rules(d, 1, point{5, 5}); got[0] != (rule{5, 15, 35, 17}) {
		t.Errorf("got %+v", got)
	}
}

func TestAPathThatIsNotStraightIsDropped(t *testing.T) {
	for _, c := range []string{
		"10 20 m 15 25 20 30 25 20 c f",
		"10 20 m 15 25 25 20 v f",
		"10 20 m 15 25 25 20 y f",
	} {
		if got := marks(t, c); len(got) != 0 {
			t.Errorf("%s gave %+v", c, got)
		}
	}
	// A path used only for clipping paints nothing.
	if got := marks(t, "10 20 30 2 re W n"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}

func TestEveryWayOfPainting(t *testing.T) {
	for _, op := range []string{"f", "F", "f*", "B", "B*", "b", "b*"} {
		if got := marks(t, "10 20 30 2 re "+op); len(got) != 1 {
			t.Errorf("%s painted %d marks", op, len(got))
		}
	}
	// An operator with too few operands does nothing rather than panicking.
	for _, c := range []string{"1 2 3 re f", "1 cm 0 0 1 1 re f", "w 0 0 1 1 re f",
		"1 m 0 0 1 1 re f", "1 l 0 0 1 1 re f", "/G gs 0 0 1 1 re f"} {
		marks(t, c)
	}
}

func TestReadingTheNumbersOfAnOperation(t *testing.T) {
	// The numbers of an operation come first; reading stops at the first
	// operand that is not one.
	got := numbers([]reader.Object{reader.Integer(1), reader.Real(2.5), reader.Name("x"), reader.Integer(3)})
	if len(got) != 2 || got[0] != 1 || got[1] != 2.5 {
		t.Errorf("got %v", got)
	}
}

func TestAPageWithNoContentHasNoMarks(t *testing.T) {
	d := pageWith(t, "10 20 30 2 re f", nil)
	if got := rules(d, 2, point{0, 0}); got != nil {
		t.Errorf("page two of a one-page document has marks: %+v", got)
	}
}
