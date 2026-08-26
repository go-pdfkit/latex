// Copyright (c) the go-pdfkit/latex authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package latex reconstructs LaTeX source from a PDF.
//
// A PDF that came from TeX does not contain a document. It contains the marks
// TeX made putting one on paper: glyphs at absolute positions, from fonts whose
// names are the only surviving record of what the author asked for, and a few
// thin rectangles. Everything else — that this was a paragraph, that this word
// was \emph, that these eleven glyphs and one rule were \frac{a+b}{c} — was
// consumed by the typesetting and has to be worked out again from the geometry.
//
// This package does that work and writes out source that, put back through a
// TeX engine, sets something close to the page it was read from. It is a
// reconstruction and not a recovery: two different sources typeset to the same
// page, and where the geometry cannot tell them apart this makes a choice and
// says in its own documentation which one.
//
// # What it reconstructs
//
//   - Paragraphs and lines, including two-column layouts, indentation, centred
//     lines, and the hyphens TeX put in that the author did not.
//   - Bold, italic, typewriter, sans and small-capital text, from the names of
//     the fonts it was set in.
//   - Section headings and their levels, from the numbers the author gave them
//     where there are numbers and from the ranking of their sizes where there
//     are not.
//   - Inline and displayed mathematics: superscripts and subscripts, fractions,
//     radicals, big operators with their limits, grown delimiters as \left and
//     \right, the Greek alphabet, and about two hundred and fifty symbols.
//     Numbered displays come back as an equation environment.
//   - Pictures, as \includegraphics with the file written out beside the source.
//
// # What it does not
//
// Bibliographies, citations, cross-references, labels, footnotes, tables,
// colour, and whitespace fidelity. A \cite that typeset to "[14]" comes back as
// the characters [14]; there is nothing on the page that says otherwise. The
// preamble is reconstructed from what the body needs rather than from what the
// author wrote, which no PDF records.
package latex

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-pdfkit/reader"
)

// Options say how a document is to be reconstructed. The zero value asks for
// every page of an article.
type Options struct {
	// First and Last are the range of pages to read, counting from one. Zero
	// means the first page and the last respectively.
	First, Last int
	// Class is the document class to write. Empty means article.
	Class string
}

// A Document is the reconstructed source, and the files it refers to.
type Document struct {
	// Preamble is everything before \begin{document}.
	Preamble string
	// Body is everything between \begin{document} and \end{document}.
	Body string
	// Files are the pictures the body's \includegraphics commands name, to
	// be written in the same directory as the source.
	Files []File
	// Pages is how many pages were read.
	Pages int
}

// String is the whole source, ready to be typeset.
func (d *Document) String() string {
	return d.Preamble + "\n\\begin{document}\n\n" + d.Body + "\n\\end{document}\n"
}

// WriteFiles writes the pictures the body refers to into a directory, which
// must already exist.
func (d *Document) WriteFiles(dir string) error {
	for _, f := range d.Files {
		if err := os.WriteFile(filepath.Join(dir, f.Name), f.Data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Reconstruct reads a document and writes LaTeX source for it.
func Reconstruct(d *reader.Document, opt Options) (*Document, error) {
	first, last := pageRange(opt, d.PageCount())
	var frames []frame
	width, height := 612.0, 792.0
	for i := first; i <= last; i++ {
		p, err := readPage(d, i)
		if err != nil {
			return nil, err
		}
		lines, two := order(groupLines(p.atoms, p.rules, p.width), p.width)
		frames = append(frames, frame{
			lines: lines, rules: p.rules, images: p.images, twoColumn: two,
		})
		width, height = p.width, p.height
	}
	e := &emitter{opt: opt}
	e.measure(frames)
	for i := range frames {
		frames[i].lines = stripRunningHeads(frames[i].lines, e.body)
	}
	for i, f := range frames {
		e.opening = i == 0
		e.write(f)
	}
	e.flush()
	out := &Document{
		Body:  strings.TrimRight(e.out.String(), "\n") + "\n",
		Files: e.files,
		Pages: len(frames),
	}
	if len(frames) > 0 {
		out.Preamble = preamble(opt, frames[0], width, height, e.body, e.leading, e.title)
	}
	if e.title != "" {
		out.Body = "\\maketitle\n\n" + out.Body
	}
	return out, nil
}

// Source is the reconstructed source of a whole document, as one string.
func Source(d *reader.Document) (string, error) {
	doc, err := Reconstruct(d, Options{})
	if err != nil {
		return "", err
	}
	return doc.String(), nil
}

// pageRange puts the requested range inside the document.
func pageRange(opt Options, count int) (int, int) {
	first, last := opt.First, opt.Last
	if first < 1 {
		first = 1
	}
	// Only an unset Last means "to the end". A caller that names a page the
	// document has not is told so rather than handed an empty document.
	if last < 1 {
		last = count
	}
	return first, last
}
