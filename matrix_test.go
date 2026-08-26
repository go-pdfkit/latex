package latex

import "testing"

func TestComposingTransforms(t *testing.T) {
	move := matrix{1, 0, 0, 1, 5, 7}
	twice := matrix{2, 0, 0, 2, 0, 0}
	// Moving then doubling doubles the move.
	x, y := move.mul(twice).apply(1, 1)
	if x != 12 || y != 16 {
		t.Errorf("got (%v,%v), want (12,16)", x, y)
	}
	if s := twice.scale(); s != 2 {
		t.Errorf("a doubling stretches by %v", s)
	}
	// A rotation stretches nothing.
	if s := (matrix{0, 1, -1, 0, 0, 0}).scale(); s < 0.999 || s > 1.001 {
		t.Errorf("a quarter turn stretches by %v", s)
	}
	// A transform that collapses the page stretches by nothing at all.
	if s := (matrix{0, 0, 0, 0, 0, 0}).scale(); s != 0 {
		t.Errorf("a collapse stretches by %v", s)
	}
}

func TestSquareRoot(t *testing.T) {
	if v := sqrt(9); v < 2.999 || v > 3.001 {
		t.Errorf("sqrt(9) = %v", v)
	}
	if v := sqrt(-1); v != 0 {
		t.Errorf("sqrt(-1) = %v", v)
	}
}

func TestSizeWithoutSign(t *testing.T) {
	if abs(-3) != 3 || abs(3) != 3 {
		t.Error("abs is wrong")
	}
}
