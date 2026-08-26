package latex

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
)

func TestAPictureBecomesAGraphic(t *testing.T) {
	im := extract.Image{
		Width: 2, Height: 2, DrawnWidth: 100, DrawnHeight: 50,
		Filter: "DCTDecode", Data: []byte("jpeg bytes"),
	}
	s, f := figure(im, 3)
	if !strings.Contains(s, `\includegraphics[width=100.0pt,height=50.0pt]{image003.jpg}`) {
		t.Errorf("got %q", s)
	}
	if f == nil || f.Name != "image003.jpg" || string(f.Data) != "jpeg bytes" {
		t.Errorf("got %+v", f)
	}
	im.Filter = "JPXDecode"
	if _, f := figure(im, 0); f == nil || f.Name != "image000.jp2" {
		t.Errorf("got %+v", f)
	}
}

func TestASmallPictureIsNotAFigure(t *testing.T) {
	// A three-point image is a bullet or a rule, and a document full of
	// \includegraphics of them is worse than one without.
	if s, f := figure(extract.Image{DrawnWidth: 3, DrawnHeight: 3}, 0); s != "" || f != nil {
		t.Errorf("got %q %+v", s, f)
	}
	if s, _ := figure(extract.Image{DrawnWidth: 100, DrawnHeight: 3}, 0); s != "" {
		t.Errorf("got %q", s)
	}
}

func TestPlainSamplesBecomeAPNG(t *testing.T) {
	grey := extract.Image{
		Width: 2, Height: 2, DrawnWidth: 40, DrawnHeight: 40,
		Data: []byte{0, 64, 128, 255},
		Dict: reader.Dict{"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("DeviceGray")},
	}
	s, f := figure(grey, 1)
	if f == nil || f.Name != "image001.png" || !strings.Contains(s, "image001.png") {
		t.Fatalf("got %q %+v", s, f)
	}
	pix, err := png.Decode(bytes.NewReader(f.Data))
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := pix.At(1, 0).RGBA(); r>>8 != 64 {
		t.Errorf("the second sample came out %d", r>>8)
	}
	colour := extract.Image{
		Width: 1, Height: 2, DrawnWidth: 40, DrawnHeight: 40,
		Data: []byte{255, 0, 0, 0, 0, 255},
		Dict: reader.Dict{"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("DeviceRGB")},
	}
	_, f = figure(colour, 2)
	if f == nil {
		t.Fatal("no file")
	}
	pix, _ = png.Decode(bytes.NewReader(f.Data))
	if r, _, b, _ := pix.At(0, 1).RGBA(); r != 0 || b>>8 != 255 {
		t.Errorf("the second sample came out %v", pix.At(0, 1))
	}
}

func TestAStencilMaskBecomesAPNG(t *testing.T) {
	mask := extract.Image{
		Width: 8, Height: 1, DrawnWidth: 40, DrawnHeight: 40,
		Data: []byte{0b10000000},
		Dict: reader.Dict{"ImageMask": reader.Bool(true)},
	}
	_, f := figure(mask, 0)
	if f == nil {
		t.Fatal("no file")
	}
	pix, err := png.Decode(bytes.NewReader(f.Data))
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := pix.At(0, 0).RGBA(); r>>8 != 255 {
		t.Errorf("the set bit came out %d", r>>8)
	}
	if r, _, _, _ := pix.At(1, 0).RGBA(); r>>8 != 0 {
		t.Errorf("the clear bit came out %d", r>>8)
	}
}

func TestAPictureThatCannotBeWrittenBecomesAFrame(t *testing.T) {
	for _, im := range []extract.Image{
		// A colour space this does not read.
		{Width: 1, Height: 1, DrawnWidth: 40, DrawnHeight: 40, Data: []byte{0},
			Dict: reader.Dict{"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("DeviceCMYK")}},
		// Four bits a sample.
		{Width: 1, Height: 1, DrawnWidth: 40, DrawnHeight: 40, Data: []byte{0},
			Dict: reader.Dict{"BitsPerComponent": reader.Integer(4), "ColorSpace": reader.Name("DeviceGray")}},
		// A picture with no samples in it at all.
		{Width: 4, Height: 4, DrawnWidth: 40, DrawnHeight: 40, Data: nil,
			Dict: reader.Dict{"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("DeviceGray")}},
		{Width: 4, Height: 4, DrawnWidth: 40, DrawnHeight: 40, Data: nil,
			Dict: reader.Dict{"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("DeviceRGB")}},
		{Width: 4, Height: 4, DrawnWidth: 40, DrawnHeight: 40, Data: nil,
			Dict: reader.Dict{"ImageMask": reader.Bool(true)}},
		// A picture with no size.
		{Width: 0, Height: 0, DrawnWidth: 40, DrawnHeight: 40,
			Dict: reader.Dict{"BitsPerComponent": reader.Integer(8)}},
	} {
		s, f := figure(im, 0)
		if f != nil || !strings.Contains(s, `\framebox`) {
			t.Errorf("%v gave %q %+v", im.Dict, s, f)
		}
	}
}

func TestAPictureTooLargeToEncode(t *testing.T) {
	// png.Encode refuses a picture whose bounds it cannot write.
	if _, _, ok := picture(extract.Image{Width: 1, Height: 1,
		Dict: reader.Dict{"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("DeviceGray")},
		Data: []byte{0}}, 0); !ok {
		t.Error("a one-pixel greyscale picture could not be written")
	}
	if _, ok := samples(extract.Image{Width: 1, Height: 1, Data: []byte{0},
		Dict: reader.Dict{"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("CalGray")}}); !ok {
		t.Error("CalGray was not read")
	}
	if _, ok := samples(extract.Image{Width: 1, Height: 1, Data: []byte{0, 0, 0},
		Dict: reader.Dict{"BitsPerComponent": reader.Integer(8), "ColorSpace": reader.Name("CalRGB")}}); !ok {
		t.Error("CalRGB was not read")
	}
}

func TestWritingThePicturesOut(t *testing.T) {
	dir := t.TempDir()
	doc := &Document{Files: []File{{Name: "a.png", Data: []byte("x")}}}
	if err := doc.WriteFiles(dir); err != nil {
		t.Fatal(err)
	}
	if err := doc.WriteFiles(dir + "/nowhere"); err == nil {
		t.Error("writing into a directory that does not exist succeeded")
	}
}
