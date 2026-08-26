// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

package latex

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
)

// This file turns the pictures a page places back into \includegraphics, and
// writes the pictures out beside the source so that the command has something
// to point at.
//
// A picture that arrives already encoded — a JPEG, a JPEG 2000 — is written
// straight out, because re-encoding it would lose something for nothing and
// graphicx reads both. A picture that arrives as plain samples has to be given
// a container, and PNG is the one the standard library can write and graphicx
// can read. Reading the samples means reading the colour space, and this reads
// the two that account for nearly all of them plus a stencil mask; a picture in
// any other — separation inks, an indexed palette, a Lab space — is not written
// out, and the figure becomes a frame of the right size with a note in it
// rather than a command pointing at a file that is not there.

// A File is a picture pulled out of the document, to be written beside the
// reconstructed source.
type File struct {
	// Name is what the \includegraphics command refers to.
	Name string
	Data []byte
}

// minFigure is how large a picture must be drawn, in points, to be worth a
// figure. Below that it is a rule, a bullet or a logo fragment, and a document
// full of \includegraphics of three-point images is worse than one without.
const minFigure = 12

// figure writes a picture as a graphic, and the file it needs.
func figure(im extract.Image, n int) (string, *File) {
	if im.DrawnWidth < minFigure || im.DrawnHeight < minFigure {
		return "", nil
	}
	box := fmt.Sprintf("[width=%.1fpt,height=%.1fpt]", im.DrawnWidth, im.DrawnHeight)
	name, data, ok := picture(im, n)
	if !ok {
		return fmt.Sprintf(`\framebox[%.1fpt]{\rule{0pt}{%.1fpt}unreadable image}`,
			im.DrawnWidth, im.DrawnHeight), nil
	}
	return `\includegraphics` + box + `{` + name + `}`, &File{Name: name, Data: data}
}

// picture is the file to write for an image, and false for one this cannot put
// into a container graphicx reads.
func picture(im extract.Image, n int) (string, []byte, bool) {
	switch im.Filter {
	case "DCTDecode":
		return fmt.Sprintf("image%03d.jpg", n), im.Data, true
	case "JPXDecode":
		return fmt.Sprintf("image%03d.jp2", n), im.Data, true
	}
	pix, ok := samples(im)
	if !ok {
		return "", nil, false
	}
	var buf bytes.Buffer
	// png.Encode writes an image this function built itself into a buffer
	// that cannot fail, so the only error it could return is one that cannot
	// happen; the encoding of a Gray or an NRGBA is always representable.
	_ = png.Encode(&buf, pix)
	return fmt.Sprintf("image%03d.png", n), buf.Bytes(), true
}

// samples turns unfiltered image data into a picture, for the colour spaces
// that can be read without a full renderer behind them.
func samples(im extract.Image) (image.Image, bool) {
	bits, _ := reader.ToInt(im.Dict.Get("BitsPerComponent"))
	mask, _ := reader.ToBool(im.Dict.Get("ImageMask"))
	w, h := im.Width, im.Height
	if w <= 0 || h <= 0 {
		return nil, false
	}
	if mask {
		return stencil(im.Data, w, h)
	}
	space, _ := im.Dict.Get("ColorSpace").(reader.Name)
	switch {
	case bits == 8 && (space == "DeviceGray" || space == "CalGray" || space == "G"):
		return gray(im.Data, w, h)
	case bits == 8 && (space == "DeviceRGB" || space == "CalRGB" || space == "RGB"):
		return rgb(im.Data, w, h)
	}
	return nil, false
}

// stencil reads a one-bit mask, in which a set bit is a hole and a clear one is
// paint — which is the way round PDF states it when there is no /Decode array.
func stencil(data []byte, w, h int) (image.Image, bool) {
	stride := (w + 7) / 8
	if len(data) < stride*h {
		return nil, false
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			bit := data[y*stride+x/8] >> (7 - uint(x)%8) & 1
			out.SetGray(x, y, color.Gray{Y: bit * 255})
		}
	}
	return out, true
}

// gray reads eight-bit greyscale samples.
func gray(data []byte, w, h int) (image.Image, bool) {
	if len(data) < w*h {
		return nil, false
	}
	out := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		copy(out.Pix[y*out.Stride:], data[y*w:(y+1)*w])
	}
	return out, true
}

// rgb reads eight-bit colour samples.
func rgb(data []byte, w, h int) (image.Image, bool) {
	if len(data) < w*h*3 {
		return nil, false
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			s := (y*w + x) * 3
			d := out.PixOffset(x, y)
			out.Pix[d], out.Pix[d+1], out.Pix[d+2] = data[s], data[s+1], data[s+2]
			out.Pix[d+3] = 255
		}
	}
	return out, true
}
