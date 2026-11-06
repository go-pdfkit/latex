// Copyright (c) 2026, the go-pdfkit authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdfkit/reader"
)

// drive runs the command and returns its exit code and both outputs.
func drive(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	c := RootCmd()
	c.SetArgs(args)
	c.SetOut(&out)
	c.SetErr(&errOut)
	code := 0
	if err := c.Execute(); err != nil {
		if e, ok := err.(exitCode); ok {
			code = int(e)
		} else {
			code = 2
		}
	}
	return code, out.String(), errOut.String()
}

// paper writes a PDF of n pages, each carrying a line of text in a font whose
// widths the reader can read.
func paper(t *testing.T, n int) string {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
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
	kids := make(reader.Array, 0, n)
	for i := 1; i <= n; i++ {
		content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(
			"BT /F1 11 Tf 72 700 Td (A line of prose on a page.) Tj ET")})
		kids = append(kids, w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pagesRef, "Contents": content,
			"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
		}))
	}
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"), "Kids": kids,
		"Count":    reader.Integer(n),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(612), reader.Integer(792)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ⛔⛔ THE WHOLE POINT, RUN. This command exists because the library had no
// caller, and a caller that never converted anything would be the same
// absence with a binary in front of it.
func TestAPDFBecomesSourceBesideIt(t *testing.T) {
	in := paper(t, 3)
	code, out, errOut := drive(t, in)
	if code != 0 {
		t.Fatalf("code %d: %s%s", code, out, errOut)
	}
	tex := filepath.Join(filepath.Dir(in), "paper.tex")
	body, err := os.ReadFile(tex)
	if err != nil {
		t.Fatalf("no source was written: %v", err)
	}
	src := string(body)
	for _, want := range []string{`\documentclass`, `\begin{document}`, `\end{document}`, "A line of prose"} {
		if !strings.Contains(src, want) {
			t.Errorf("the source does not hold %q:\n%.400s", want, src)
		}
	}
	// ⛔ The counts are what tell four pages of a paper from the whole of it.
	if !strings.Contains(out, "3 page(s)") {
		t.Errorf("it did not say how many pages it read: %q", out)
	}
}

// ⛔ What comes out is a RECONSTRUCTION, and the source says so. A PDF holds
// glyphs at places, not sections and citations; source presented as the
// author's would be a quiet lie, and the one place a reader will look is the
// top of the file.
func TestTheSourceSaysItWasReconstructed(t *testing.T) {
	in := paper(t, 1)
	if code, _, errOut := drive(t, in); code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(in), "paper.tex"))
	if err != nil {
		t.Fatal(err)
	}
	head := string(body)
	if i := strings.Index(head, `\begin{document}`); i > 0 {
		head = head[:i]
	}
	if !strings.Contains(strings.ToLower(head), "reconstruct") {
		t.Errorf("the preamble does not say the source was reconstructed:\n%s", head)
	}
}

func TestAPageRangeIsReadAndRefused(t *testing.T) {
	in := paper(t, 5)

	code, out, errOut := drive(t, "--pages", "2-3", in)
	if code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if !strings.Contains(out, "2 page(s)") {
		t.Errorf("a range of two pages read %q", out)
	}

	// ⛔ A range that names a page the document has not is an ERROR, not an
	// empty file: somebody asking for page forty of a five-page paper has made
	// a mistake and is better told.
	if code, _, _ := drive(t, "--pages", "40-41", in); code == 0 {
		t.Error("a range past the end was accepted")
	}
	for _, spec := range []string{"0-2", "x", "7-3", "-"} {
		if code, _, _ := drive(t, "--pages", spec, in); code != 2 {
			t.Errorf("%q gave code %d, want 2", spec, code)
		}
	}
	// An open end is a range: "4-" is page four to the last.
	//
	// ⛔ -f, because the subtests above have already written paper.tex beside
	// the fixture. Without it this read code 1 and looked like a defect in the
	// range parser; it was the "already there" refusal doing its job on a file
	// an earlier assertion in this same test had created.
	if code, out, _ := drive(t, "-f", "--pages", "4-", in); code != 0 || !strings.Contains(out, "2 page(s)") {
		t.Errorf("an open-ended range gave code %d, %q", code, out)
	}
}

// ⛔ Standard output takes the SOURCE and nothing else, so that a caller
// piping it into a file does not find a progress line in their preamble. The
// pictures have nowhere to go in a pipe, and that is SAID rather than
// silently dropped.
func TestDashWritesTheSourceToStandardOutputAlone(t *testing.T) {
	code, out, _ := drive(t, "-o", "-", paper(t, 1))
	if code != 0 {
		t.Fatalf("code %d", code)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), `%`) && !strings.Contains(out, `\documentclass`) {
		t.Errorf("standard output does not look like source:\n%.200s", out)
	}
	if strings.Contains(out, "page(s),") {
		t.Errorf("a progress line reached standard output:\n%.200s", out)
	}
}

func TestAnExistingSourceIsRefusedUnlessForced(t *testing.T) {
	in := paper(t, 1)
	tex := filepath.Join(filepath.Dir(in), "paper.tex")
	if err := os.WriteFile(tex, []byte("do not lose me"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := drive(t, in)
	if code != 1 || !strings.Contains(errOut, "already there") {
		t.Fatalf("code %d, said %q", code, errOut)
	}
	if b, _ := os.ReadFile(tex); string(b) != "do not lose me" {
		t.Error("it overwrote the file it refused")
	}
	if code, _, errOut := drive(t, "-f", in); code != 0 {
		t.Fatalf("-f did not overwrite: code %d, %s", code, errOut)
	}
}

func TestADirectoryToWriteInto(t *testing.T) {
	in := paper(t, 1)
	dir := filepath.Join(t.TempDir(), "out")
	if code, _, errOut := drive(t, in, dir); code != 0 {
		t.Fatalf("code %d: %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "paper.tex")); err != nil {
		t.Errorf("the source is not in the directory asked for: %v", err)
	}
}

func TestWhatItRefuses(t *testing.T) {
	in := paper(t, 1)
	notPDF := filepath.Join(t.TempDir(), "a.pdf")
	if err := os.WriteFile(notPDF, []byte("this is not a PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		args []string
		want int
	}{
		{"nothing to read", nil, 2},
		{"three arguments", []string{in, "a", "b"}, 2},
		{"a file that is not there", []string{filepath.Join(t.TempDir(), "absent.pdf")}, 1},
		{"something that is not a PDF", []string{notPDF}, 1},
	} {
		if code, _, _ := drive(t, c.args...); code != c.want {
			t.Errorf("%s: code %d, want %d", c.name, code, c.want)
		}
	}
}
