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


## Measured

On arXiv source packages held locally: 282 papers, of which 194 were typeset by
**tectonic** (a real TeX engine) into the PDFs this then read back — **5 107
pages**. The other 88 would not compile at all: they want packages, classes or
figures that tectonic could not supply. Everything below is the distribution
over those 194, not one example of one.

### Does it compile?

**193 of 194 (99%)** of the reconstructions are accepted by tectonic.

That number is the whole point of the repair pass, and it was not free. Over the
same set of papers, as the faults were found and fixed, it went

| | reconstructions that compile |
|---|---|
| before any repair | **27%** |
| after the first repair pass | **55%** |
| after three more faults were fixed | **97–99%** |

The faults were: a script with nothing to be the script of; two scripts of one
kind on one letter; a brace left open; a `\left` whose `\right` was on the next
line or inside a different group; a prime, which TeX reads as a superscript, so
a following subscript lands on a letter that already has one; a bare `\sqrt`
with no radicand; and — the one that no amount of reading the code would have
found — that `\rightarrow` begins with the six characters of `\right`, so the
first version of the pass that strips an unmatched `\left` turned every limit in
every document into the undefined command `\thetaarrow`.

### How close is it to what the author wrote?

The reconstruction is compared with the paper's own `.tex` files, with comments
stripped, commands and braces removed, and mathematics compared separately.

| measure | q1 | **median** | q3 |
|---|---|---|---|
| words, F1 of the bag | 0.55 | **0.66** | 0.75 |
| word bigrams, F1 (order-sensitive) | 0.37 | **0.49** | 0.56 |
| mathematics, F1 of the token multiset | 0.38 | **0.62** | 0.75 |

The word figure is a floor rather than a score: the denominator is the author's
whole source, which contains a preamble, macro definitions, commented-out
paragraphs and a bibliography, none of which ever reach the page and none of
which this could recover.

### Round trip: typeset the reconstruction and compare the pixels

The reconstruction is set again — once by the fleet's own `go-tex/engine`, once
by tectonic — and each rendering is compared with the original page by
`go-pdfkit/render`.

The comparison is **where the ink lands**, not the mean pixel difference: a dark
pixel in one page counts as matched when there is a dark pixel within two pixels
of it in the other, scored as an F1. Mean absolute difference has a blind spot
on a page that is mostly white — drawing the right thing one pixel off scores
worse than drawing nothing — and this corpus shows it plainly, below.

The engine's own fidelity has to be separated from the reconstruction's, so the
same engine also sets the paper's **true source**, and the two are compared:

| page one of 194 papers, ink F1 against the original | q1 | **median** | q3 |
|---|---|---|---|
| go-tex sets **the reconstruction** | 0.31 | **0.40** | 0.47 |
| go-tex sets **the author's own source** | 0.18 | **0.34** | 0.44 |
| tectonic sets **the reconstruction** | 0.25 | **0.32** | 0.41 |

**The reconstruction lands closer to the original page than the true source
does, in 193 of 194 papers**, when both are set by the fleet's engine. That is
not a claim that the reconstruction is better than the source. It is a statement
about what the engine can read: the reconstruction is plain `article` LaTeX with
amsmath and graphicx, which go-tex sets in full, while a real arXiv paper pulls
in classes and packages it drops. A reconstruction faithful enough to stand in
for the source under an engine that cannot read the source is the useful thing
being measured here.

### The blind spot, demonstrated

The same 194 comparisons, scored by **mean absolute pixel difference** instead:

| | q1 | **median** | q3 |
|---|---|---|---|
| go-tex sets the reconstruction | 0.074 | **0.091** | 0.106 |
| go-tex sets the author's own source | 0.054 | **0.076** | 0.095 |

By that measure the reconstruction is *worse* in **194 of 194** — the exact
opposite verdict. The reason is that go-tex sets less of the true source than of
the reconstruction, so its page is emptier, and on a page that is 95% white an
emptier page is nearer the original by mean difference however much of the
document it has lost. Both numbers are reported because only one of them is
answering the question.

### What it costs

Median 18 seconds per paper end to end, which is dominated by the two TeX
compiles; the reconstruction itself is a fraction of a second per page.
## The gate

`go vet` clean, `gofmt` clean, `CGO_ENABLED=0`, **exact 100% statement
coverage**, and a build for linux amd64/arm64/riscv64/loong64/ppc64le/s390x,
js/wasm, darwin/arm64 and windows/amd64. Nothing outside the standard library
and `github.com/go-pdfkit/{reader,extract,pdffont}`.

## Licence

BSD-3-Clause.
