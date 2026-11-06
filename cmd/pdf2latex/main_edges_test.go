// Copyright (c) 2026, the go-pdfkit authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// ⛔ main ITSELF, not only the command under it. It is three lines, and it is
// the line every real invocation goes through: a command whose Execute returns
// an error must leave a non-zero status, or a script that checks $? is told
// the conversion worked when it did not.
func TestMainReportsAFailureToTheShell(t *testing.T) {
	wasArgs, wasExit := os.Args, osExit
	t.Cleanup(func() { os.Args, osExit = wasArgs, wasExit })

	code := 0
	osExit = func(c int) { code = c }

	// A file that is not there: the command fails, so main must exit 1.
	os.Args = []string{"pdf2latex", filepath.Join(t.TempDir(), "absent.pdf")}
	main()
	if code != 1 {
		t.Errorf("a failed run left the shell status %d", code)
	}

	// And a run that works leaves the status alone.
	code = 0
	in := paper(t, 1)
	os.Args = []string{"pdf2latex", in}
	main()
	if code != 0 {
		t.Errorf("a run that worked exited %d", code)
	}
}

// exitCode carries the status out through cobra's error return, so it has to
// read as an error.
func TestTheExitStatusSaysWhatItIs(t *testing.T) {
	if got := exitCode(2).Error(); !strings.Contains(got, "2") {
		t.Errorf("exit code 2 reads %q", got)
	}
}

// ⛔ WHAT IT DOES WHEN THE FILE SYSTEM REFUSES. Every one of these is a path
// that writes an error and returns 1, and all of them are reachable by someone
// with an awkward directory — the whole point of a status is that it is right
// in the unusual case.
func TestWhatItDoesWhenItCannotWrite(t *testing.T) {
	in := paper(t, 1)

	// A directory it cannot make, because a FILE sits where a parent would go.
	blocked := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := drive(t, in, filepath.Join(blocked, "under")); code != 1 {
		t.Errorf("writing under a file gave code %d, said %q", code, errOut)
	}

	// A source file it cannot write, because a DIRECTORY sits where it goes.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "paper.tex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := drive(t, "-f", in, dir); code != 1 {
		t.Errorf("writing onto a directory gave code %d, said %q", code, errOut)
	}
}

// illustrated is a one-page document carrying a picture big enough to be a
// figure, which is what puts anything in Document.Files.
func illustrated(t *testing.T) string {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	img := w.Add(&reader.Stream{Dict: reader.Dict{
		"Type": reader.Name("XObject"), "Subtype": reader.Name("Image"),
		"Width": reader.Integer(2), "Height": reader.Integer(2),
		"ColorSpace": reader.Name("DeviceGray"), "BitsPerComponent": reader.Integer(8),
	}, Raw: []byte{0, 64, 128, 255}})
	content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(
		"q 200 0 0 100 72 600 cm /Im0 Do Q\n" +
			"BT /F1 11 Tf 72 500 Td (A line under the picture.) Tj ET")})
	widths := make(reader.Array, 0, 224)
	for i := 32; i < 256; i++ {
		widths = append(widths, reader.Integer(500))
	}
	font := w.Add(reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
		"BaseFont": reader.Name("Helvetica"), "FirstChar": reader.Integer(32),
		"LastChar": reader.Integer(255), "Widths": widths,
		"Encoding": reader.Name("WinAnsiEncoding"),
	})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
		"Resources": reader.Dict{
			"Font":    reader.Dict{"F1": font},
			"XObject": reader.Dict{"Im0": img},
		},
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(612), reader.Integer(792)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "illustrated.pdf")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// picturesOf is the names of the files written beside a source, which is how
// this test learns what a picture is called rather than assuming it.
func picturesOf(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".tex") {
			out = append(out, e.Name())
		}
	}
	return out
}

// ⛔ A PIPE HAS NOWHERE TO PUT A PICTURE, AND THAT IS SAID. Dropping them in
// silence would give somebody a source file whose \includegraphics name files
// that were never written, and they would find out when it failed to compile.
func TestPicturesCannotGoIntoAPipeAndItSaysSo(t *testing.T) {
	in := illustrated(t)

	// First establish the document HAS a picture, or this asserts nothing.
	dir := filepath.Join(t.TempDir(), "with")
	if code, _, errOut := drive(t, in, dir); code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	pics := picturesOf(t, dir)
	if len(pics) == 0 {
		t.Fatal("the fixture produced no pictures, so the pipe case proves nothing")
	}

	code, out, errOut := drive(t, "-o", "-", in)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if !strings.Contains(errOut, "picture(s) were not written") {
		t.Errorf("it did not say the pictures were dropped: %q", errOut)
	}
	// And the source still went to standard output, whole.
	if !strings.Contains(out, `\begin{document}`) {
		t.Errorf("standard output does not hold the source:\n%.200s", out)
	}
}

// ⛔ THE SOURCE WRITTEN AND THE PICTURES NOT is its own status, and its own
// sentence: a .tex whose \includegraphics name files that are not there
// compiles to nothing, and the person needs to know which half succeeded.
func TestWhenTheSourceIsWrittenAndThePicturesAreNot(t *testing.T) {
	in := illustrated(t)
	probe := filepath.Join(t.TempDir(), "probe")
	if code, _, errOut := drive(t, in, probe); code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	pics := picturesOf(t, probe)
	if len(pics) == 0 {
		t.Fatal("the fixture produced no pictures")
	}

	// A DIRECTORY standing where the picture must be written. The source is
	// written first and succeeds; the picture cannot be.
	dir := filepath.Join(t.TempDir(), "blocked")
	if err := os.MkdirAll(filepath.Join(dir, pics[0]), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := drive(t, in, dir)
	if code != 1 {
		t.Errorf("code %d when the pictures could not be written", code)
	}
	if !strings.Contains(errOut, "the source was written and its pictures were not") {
		t.Errorf("it said %q, which does not say which half worked", errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "illustrated.tex")); err != nil {
		t.Errorf("it reported a half-success without writing the source: %v", err)
	}
}

// -o names the file to write, and its directory is where the pictures go.
func TestDashONamesTheFile(t *testing.T) {
	in := paper(t, 1)
	out := filepath.Join(t.TempDir(), "named.tex")
	if code, _, errOut := drive(t, "-o", out, in); code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("the source is not where -o named it: %v", err)
	}
}

// A single page is a range of one, and a range whose END is not a number is a
// usage error rather than an open end.
func TestOnePageAndAnUnreadableEnd(t *testing.T) {
	in := paper(t, 4)
	if code, out, _ := drive(t, "--pages", "2", in); code != 0 || !strings.Contains(out, "1 page(s)") {
		t.Errorf("a single page gave code %d, %q", code, out)
	}
	if code, _, _ := drive(t, "--pages", "2-x", in); code != 2 {
		t.Errorf("a range ending in nonsense gave code %d, want 2", code)
	}
}
