# latex — go-pdfkit

[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![coverage](https://img.shields.io/badge/coverage-100%25-brightgreen)](#the-gate)

**Reconstruct LaTeX source from a PDF.** Pure Go, no cgo, no dependencies
outside the fleet.

```go
d, _ := reader.Open(pdfBytes)
doc, _ := latex.Reconstruct(d, latex.Options{})
os.WriteFile("out.tex", []byte(doc.String()), 0o644)
doc.WriteFiles(".") // the pictures the \includegraphics commands name
```

A PDF that came from TeX does not contain a document. It contains the marks TeX
made putting one on paper: glyphs at absolute positions, from fonts whose names
are the only surviving record of what the author asked for, and a few thin
rectangles. Everything else — that this was a paragraph, that this word was
`\emph`, that these eleven glyphs and one rule were `\frac{a+b}{c}` — was
consumed by the typesetting and has to be worked out again from the geometry.

This package does that work. It is a **reconstruction and not a recovery**: two
different sources typeset to the same page, and where the geometry cannot tell
them apart this makes a choice and says which one.

## What it reconstructs

**Paragraphs and lines.** Baselines into lines, lines into paragraphs. A
paragraph is found from three things at once — the first line of one is
indented, the last line of one is short, and a document that uses vertical
space instead of an indent leaves a wider gap — with the short-line test only
trusted after checking, over the whole document, that its lines reach the right
margin at all.

**Two columns.** Found from the gutter: a strip down the middle of the page that
no glyph enters. This has to be found *before* the lines are built, because two
columns set to the same grid put their lines on the same baselines, and a page
read by baseline alone comes back with every left-hand line joined to the
right-hand line beside it.

**Font changes.** `\textbf`, `\emph`, `\texttt`, `\textsf`, `\textsc`, read from
the names of the fonts the text was set in — CMBX10, CMTI10, LMRoman10-Bold,
NimbusRomNo9L-Medi. Nothing looks inside the font program: a font's own idea of
its weight is often wrong, and a subsetted face carries no useful flags at all.

**Section headings and their levels.** From the number the author gave them
where there is one — "3.2.1" is a `\subsubsection` and nothing else — and from
the ranking of heading sizes across the document where there is not. An
unnumbered heading comes back starred. The title of a paper becomes `\title`
and `\maketitle`.

**Mathematics.** The part that matters, and the part that is difficult.

- **Super- and subscripts**, recursively: a smaller glyph raised or dropped.
- **Fractions**: a horizontal rule with material above and below it. Both
  producers are read — pdfTeX fills a rectangle, xdvipdfmx (what XeTeX and
  tectonic write through) strokes a segment — because a reader that knows only
  one of them silently loses every fraction in half the world's PDFs.
- **Radicals**: the hook glyph plus the bar drawn over the radicand, which is
  what says how far the radicand reaches.
- **Big operators with their limits**, recognised by the limits sitting *over*
  the operator rather than after it.
- **`\left(` … `\right)`** from the extension family's grown delimiters, when
  they pair up in the equation; when they do not, the plain character.
- **Greek and about 250 symbols**, and the operator names — `\sin`, `\log`,
  `\max` — that TeX sets upright.
- **Numbered displays** come back as an `equation` environment with the number
  taken off, since writing it back would give the equation two numbers.

**Where an equation starts and stops** is itself a reconstruction: TeX sets the
`$` and the text around it in the same roman font, so the boundary leaves no
trace. The three math families are the seeds; around each one the equation is
grown over the characters the roman font also drew — the digits, the `+`, the
parentheses — recognised by being the sort of character an equation contains
*and* by being closer than a word space. Both halves matter. There is also a
rule for one widely used font package, Fourier, which sets its math *letters* in
the text italic face, so that "(L, P)" is drawn with roman parentheses and
italic letters and nothing in any font name says it is an equation: a letter set
in italic, alone, with roman on both sides of it, is a variable, because
emphasis applies to words and a word is more than one letter.

**Pictures**, as `\includegraphics` with the file written out beside the source.
A JPEG or a JPEG 2000 is written as it stands; plain samples in DeviceGray,
DeviceRGB or a stencil mask become a PNG; anything else becomes a `\framebox` of
the right size rather than a command pointing at a file that is not there.

**Output that compiles.** The last thing done to an equation is to read it back
the way TeX will and repair what TeX would refuse: a script with nothing to be
the script of, two scripts of one kind on one letter, a brace left open, a
`\left` whose `\right` was on the next line. This is not a nicety — an aborted
compile loses the whole document rather than the one equation.

## What it does not

- **Bibliographies, citations, cross-references, labels.** A `\cite` that
  typeset to "[14]" comes back as the characters `[14]`; there is nothing on the
  page that says otherwise. Same for `\ref` and `\label`.
- **Tables.** The rules are read (a fraction needs them) but nothing is made of
  a grid of them. A table comes back as the lines of text it is made of.
- **Footnotes**, which come back as text at the foot of the page.
- **Colour.**
- **Accents.** A PDF draws `\hat{L}` as an L and a circumflex placed over it,
  two glyphs, and there is no way to write that back without knowing which glyph
  the accent belongs to. The accent is dropped and the letter kept, rather than
  a character being written into the source that no engine will set.
- **The abstract environment**, which comes back as a centred paragraph.
- **The preamble.** It is reconstructed from what the body needs — amsmath,
  amssymb, graphicx, and the page geometry measured off the first page — rather
  than from what the author wrote, which no PDF records.
- **Whitespace fidelity.**
- **A page whose producer scales text with the text matrix rather than with the
  font size.** `github.com/go-pdfkit/extract` reports the font size, and a
  producer that sets `/F1 1 Tf` and scales by twelve reports a size of one for
  everything on the page. TeX-produced PDFs — which is what this is for — set
  the real size; drawing programs often do not, and on such a page the size
  tests that find headings and scripts have nothing to work with.

## The gate

`go vet` clean, `gofmt` clean, `CGO_ENABLED=0`, **exact 100% statement
coverage**, and a build for linux amd64/arm64/riscv64/loong64/ppc64le/s390x,
js/wasm, darwin/arm64 and windows/amd64. Nothing outside the standard library
and `github.com/go-pdfkit/{reader,extract,pdffont}`.

## Licence

BSD-3-Clause.
