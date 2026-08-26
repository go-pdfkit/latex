// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

// A matrix is the six numbers a PDF transform is written as. Only three things
// are ever asked of one here — compose two, move a point, and say how much a
// length is stretched — so this carries its own rather than depending on a
// drawing library for three lines of arithmetic.
type matrix struct{ a, b, c, d, e, f float64 }

// mul is the transform that applies m first and then n, which is the order a
// PDF composes them in.
func (m matrix) mul(n matrix) matrix {
	return matrix{
		a: m.a*n.a + m.b*n.c,
		b: m.a*n.b + m.b*n.d,
		c: m.c*n.a + m.d*n.c,
		d: m.c*n.b + m.d*n.d,
		e: m.e*n.a + m.f*n.c + n.e,
		f: m.e*n.b + m.f*n.d + n.f,
	}
}

// apply moves a point.
func (m matrix) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// scale is how much the transform stretches a length, taken as the geometric
// mean of its two axes so that a rotation counts as no stretch at all.
func (m matrix) scale() float64 {
	x := m.a*m.a + m.b*m.b
	y := m.c*m.c + m.d*m.d
	return sqrt(sqrt(x * y))
}

// sqrt is Newton's method, which is here rather than math.Sqrt only to keep the
// arithmetic in one place with the rest of the geometry.
func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 24; i++ {
		x = (x + v/x) / 2
	}
	return x
}
