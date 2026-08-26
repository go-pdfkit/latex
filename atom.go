// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"sort"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
)

// An atom is one piece of text on the page together with what its font says
// about it. It is what github.com/go-pdfkit/extract gives back, with the
// font's name looked up in the page's resources and read (see font.go), since
// the resource name a run carries — F10, TT0 — says nothing on its own.
type atom struct {
	text string
	// x and y are where the piece starts and the baseline it sits on, in
	// points up from the bottom left of the visible page.
	x, y float64
	// width is how far the pen moved drawing it and size how tall it is.
	width, size float64
	// space is how wide a space would be in this font at this size, which is
	// what decides whether a gap between two pieces is a word break.
	space float64
	sh    shape
}

// right is where the piece ends.
func (a atom) right() float64 { return a.x + a.width }

// midX is its horizontal centre.
func (a atom) midX() float64 { return a.x + a.width/2 }

// page is everything one page of a document says, in the form the rest of this
// package works on.
type page struct {
	atoms  []atom
	rules  []rule
	images []extract.Image
	// width and height are the visible page's own size in points.
	width, height float64
}

// readPage reads the i'th page, counting from one.
func readPage(d *reader.Document, i int) (page, error) {
	runs, err := extract.Runs(d, i)
	if err != nil {
		return page{}, err
	}
	// The page dictionary cannot fail to be read once extract has read the
	// page itself: both go through the same page tree, and it has just been
	// walked.
	dict, _ := d.Page(i)
	box := visibleBox(d, dict)
	shapes := fontShapes(d, dict)
	p := page{
		width:  box[2] - box[0],
		height: box[3] - box[1],
		rules:  rules(d, i, point{box[0], box[1]}),
	}
	p.images, _ = extract.Images(d, i)
	for _, r := range runs {
		// Text drawn in the invisible mode is kept. It is how a scanner
		// puts what it read underneath the picture it read it from, and on
		// such a page it is the only text there is.
		if r.Text == "" {
			continue
		}
		p.atoms = append(p.atoms, atom{
			text:  r.Text,
			x:     r.X,
			y:     r.Y,
			width: r.Width,
			size:  r.Size,
			space: r.Space,
			sh:    shapes[r.Font],
		})
	}
	sort.SliceStable(p.atoms, func(i, j int) bool {
		if p.atoms[i].y != p.atoms[j].y {
			return p.atoms[i].y > p.atoms[j].y
		}
		return p.atoms[i].x < p.atoms[j].x
	})
	return p, nil
}

// fontShapes reads every font the page names, so that a run's resource name can
// be turned into what the font says about the text. A form XObject the page
// draws has resources of its own, whose names may collide with the page's; the
// page's win, which is the wrong answer only for a document that reuses one
// name for two different faces.
func fontShapes(d *reader.Document, pageDict reader.Dict) map[string]shape {
	out := map[string]shape{}
	collectFonts(d, pageDict, out, 0)
	return out
}

// maxResourceDepth is how deeply this follows a form's own resources before
// taking the page to be drawing itself.
const maxResourceDepth = 8

// collectFonts adds the fonts of one resource-carrying dictionary, and of the
// forms it names, without overwriting a name already found.
func collectFonts(d *reader.Document, dict reader.Dict, out map[string]shape, depth int) {
	if depth > maxResourceDepth {
		return
	}
	res, ok := d.GetDict(dict, "Resources")
	if !ok {
		return
	}
	if fonts, ok := d.GetDict(res, "Font"); ok {
		for name, ref := range fonts {
			if _, seen := out[string(name)]; seen {
				continue
			}
			f, ok := d.GetDict(reader.Dict{"f": ref}, "f")
			if !ok {
				continue
			}
			out[string(name)] = classify(fontName(d, f))
		}
	}
	if xobj, ok := d.GetDict(res, "XObject"); ok {
		for _, ref := range xobj {
			form, ok := d.GetDict(reader.Dict{"f": ref}, "f")
			if !ok {
				continue
			}
			collectFonts(d, form, out, depth+1)
		}
	}
}

// fontName is the name a font dictionary gives itself. A Type0 font keeps the
// interesting part in its descendant, whose /BaseFont says the same thing, so
// the outer one is enough; a font with no name at all comes back empty and is
// classified as plain roman, which is what an unnamed font usually is.
func fontName(d *reader.Document, f reader.Dict) string {
	o, err := d.Resolve(f.Get("BaseFont"))
	if err != nil {
		return ""
	}
	if n, ok := o.(reader.Name); ok {
		return string(n)
	}
	return ""
}

// visibleBox is the area of the page that is shown, which is what extract
// reports its coordinates against.
func visibleBox(d *reader.Document, pageDict reader.Dict) [4]float64 {
	for _, key := range []reader.Name{"CropBox", "MediaBox"} {
		if b, ok := rectangle(d, pageDict.Get(key)); ok {
			return b
		}
	}
	return [4]float64{0, 0, 612, 792}
}

// rectangle reads a PDF rectangle, put the right way round.
func rectangle(d *reader.Document, o reader.Object) ([4]float64, bool) {
	var out [4]float64
	r, err := d.Resolve(o)
	if err != nil {
		return out, false
	}
	arr, ok := reader.ToArray(r)
	if !ok || len(arr) < 4 {
		return out, false
	}
	for i := 0; i < 4; i++ {
		v, err := d.Resolve(arr[i])
		if err != nil {
			return out, false
		}
		f, ok := reader.ToFloat(v)
		if !ok {
			return out, false
		}
		out[i] = f
	}
	if out[0] > out[2] {
		out[0], out[2] = out[2], out[0]
	}
	if out[1] > out[3] {
		out[1], out[3] = out[3], out[1]
	}
	return out, true
}

// The bounds a word space is believed within, as a fraction of the font's size.
// Every text face ever cut sets a space somewhere between a fifth and a half of
// its size; a figure outside that did not come from the font.
const (
	minSpace = 0.22
	maxSpace = 0.55
	// fallbackSpace is what a space is taken to be when the font's own
	// figure is not believable. A third of the size is what the roman faces
	// TeX ships use.
	fallbackSpace = 0.33
)

// wordSpace is how wide a word space is here, which is what says whether a gap
// between two pieces of text is a word break or only kerning.
//
// It is not simply what the font says, because what the font says is sometimes
// not a space at all. A composite font is addressed by two-byte codes and has
// no character at 32; asking it how wide a space is returns whatever its
// fallback width happens to be, and one real producer in this corpus answers
// with the font size itself. Taking that at face value makes every gap on the
// page look like kerning, and a whole document comes back as one long word. So
// an answer outside what a space can be is replaced by what a space usually is.
//
// A math font has no word spaces at all, and says so by returning nothing:
// there are no word breaks inside an equation.
func (a atom) wordSpace() float64 {
	if a.sh.isMath() {
		return 0
	}
	if a.space < minSpace*a.size || a.space > maxSpace*a.size {
		return fallbackSpace * a.size
	}
	return a.space
}
