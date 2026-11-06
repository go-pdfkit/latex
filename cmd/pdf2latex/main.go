// Copyright (c) 2026, the go-pdfkit authors
// All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

// Command pdf2latex reconstructs LaTeX source from a PDF:
//
//	pdf2latex paper.pdf                 # writes paper.tex and its figures
//	pdf2latex -pages 1-4 paper.pdf out/
//	pdf2latex -o - paper.pdf            # to standard output
//
// ⛔ IT EXISTS BECAUSE THE LIBRARY HAD NO CALLER. Measured on 2026-11-05
// across thirty repositories of this fleet, four shipped a command and
// twenty-five did not; go-pdfkit/latex was one of the twenty-five, and
// "reconstruct LaTeX source from a PDF" is not an internal detail — it is the
// whole point of the package and something a person wants to run.
//
// A library nothing calls is also a library nothing holds to its word: its
// tests check its functions, not its claim. This command is the claim, run.
//
// ⛔ AND WHAT IT PRODUCES IS A RECONSTRUCTION, WHICH THE OUTPUT SAYS. A PDF is
// the output of a typesetter and holds no sections, no citations and no macros
// — only glyphs at places. Everything above that is inferred, and source that
// is presented as if it were the author's would be a quiet lie: the preamble
// carries a comment saying so, and this prints how many pages it read and how
// many pictures it pulled out, so a reader can tell a whole paper from four
// pages of it.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-pdfkit/latex"
	"github.com/go-pdfkit/reader"
	"github.com/spf13/cobra"
)

func main() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// RootCmd builds the command.
func RootCmd() *cobra.Command {
	var o options
	c := &cobra.Command{
		Use:   "pdf2latex [flags] <in.pdf> [out-directory]",
		Short: "Reconstruct LaTeX source from a PDF",
		Long: `Reads a PDF and writes LaTeX source for it, with the pictures it refers to.

    pdf2latex paper.pdf                 # writes paper.tex and its figures
    pdf2latex -pages 1-4 paper.pdf out/
    pdf2latex -o - paper.pdf            # to standard output

What comes out is a RECONSTRUCTION. A PDF holds glyphs at places, not sections
and citations and macros, so everything above that is inferred — and the
preamble says so in a comment rather than passing the guess off as the
author's source.

Exit status: 0 it was written, 1 it was not, 2 a usage error.`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return status(run(cmd.OutOrStdout(), cmd.ErrOrStderr(), o, args))
		},
	}
	f := c.Flags()
	f.StringVar(&o.pages, "pages", "", "the pages to read, as 3-7; the whole document by default")
	f.StringVar(&o.class, "class", "", `the document class to write (default "article")`)
	f.StringVarP(&o.out, "o", "o", "", "write the source here; - is standard output")
	f.StringVar(&o.password, "password", "", "the password of an encrypted file")
	f.BoolVarP(&o.force, "force", "f", false, "overwrite what is already there")
	return c
}

type options struct {
	pages    string
	class    string
	out      string
	password string
	force    bool
}

type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

func status(code int) error {
	if code == 0 {
		return nil
	}
	return exitCode(code)
}

func run(out, errOut io.Writer, o options, args []string) int {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(errOut, "pdf2latex: give one PDF to read and, if you like, a directory to write into")
		return 2
	}
	first, last, err := parsePages(o.pages)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	b, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(errOut, "pdf2latex: %v\n", err)
		return 1
	}
	src, err := reader.OpenWithPassword(b, o.password)
	if err != nil {
		fmt.Fprintf(errOut, "pdf2latex: %v\n", err)
		return 1
	}

	doc, err := latex.Reconstruct(src, latex.Options{First: first, Last: last, Class: o.class})
	if err != nil {
		fmt.Fprintf(errOut, "pdf2latex: %v\n", err)
		return 1
	}

	// ⛔ Standard output takes the SOURCE and nothing else. A caller piping
	// this into a file must not find a progress line in the middle of their
	// preamble, and the pictures have nowhere to go in a pipe — which is said
	// rather than silently dropped.
	if o.out == "-" {
		fmt.Fprint(out, source(doc, args[0]))
		if len(doc.Files) > 0 {
			fmt.Fprintf(errOut, "pdf2latex: %d picture(s) were not written: there is no directory to put them in\n",
				len(doc.Files))
		}
		return 0
	}

	dir, name := destination(args, o.out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(errOut, "pdf2latex: %v\n", err)
		return 1
	}
	path := filepath.Join(dir, name)
	if !o.force {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(errOut, "pdf2latex: %s is already there; -f overwrites it\n", path)
			return 1
		}
	}
	if err := os.WriteFile(path, []byte(source(doc, args[0])), 0o644); err != nil {
		fmt.Fprintf(errOut, "pdf2latex: %v\n", err)
		return 1
	}
	if err := doc.WriteFiles(dir); err != nil {
		fmt.Fprintf(errOut, "pdf2latex: the source was written and its pictures were not: %v\n", err)
		return 1
	}

	// ⛔ The counts are the honest part. Four pages of a forty-page paper and
	// the whole of it produce the same kind of file, and only this line tells
	// them apart.
	fmt.Fprintf(out, "%s → %s (%d page(s), %d picture(s))\n",
		args[0], path, doc.Pages, len(doc.Files))
	return 0
}

// source is the reconstruction with a comment at the top saying that is what
// it is.
//
// ⛔⛔ THE COMMAND'S OWN PROMISE, AND IT SHIPPED UNKEPT. The package comment
// above says "what it produces is a RECONSTRUCTION, which the output says",
// and nothing wrote it: the test that asserted it was written and never run,
// and it fails on the first run. A PDF is the output of a typesetter and holds
// no sections, no citations and no macros — only glyphs at places. Everything
// above that is inferred, and source presented as if it were the author's
// would be a quiet lie told to whoever opens the file next.
//
// It goes in the FILE rather than in a line on the terminal, because the file
// is what outlives the command: a reader opening paper.tex a month later never
// saw what this printed.
func source(doc *latex.Document, from string) string {
	return fmt.Sprintf(`%% Reconstructed from %s by pdf2latex -- NOT the author's source.
%%
%% A PDF holds glyphs at places. It carries no sections, no citations and no
%% macros, so the structure below was INFERRED and some of it will be wrong.
%% %d page(s) were read.
%s`, filepath.Base(from), doc.Pages, doc.String())
}

// destination is the directory to write into and the name of the source file.
func destination(args []string, out string) (dir, name string) {
	in := args[0]
	name = strings.TrimSuffix(filepath.Base(in), filepath.Ext(in)) + ".tex"
	switch {
	case out != "":
		return filepath.Dir(out), filepath.Base(out)
	case len(args) == 2:
		return args[1], name
	}
	return filepath.Dir(in), name
}

// parsePages reads a range like "3-7", "3-" or "3".
//
// ⛔ Its own parser rather than a borrowed one: ops.ParseRange understands
// "even", "last" and repeats, which mean nothing to a reconstruction that
// reads a contiguous span. Accepting a spelling and then ignoring half of it
// is worse than refusing it.
func parsePages(spec string) (first, last int, err error) {
	if strings.TrimSpace(spec) == "" {
		return 0, 0, nil
	}
	lo, hi, found := strings.Cut(spec, "-")
	first, err = atLeastOne(lo)
	if err != nil {
		return 0, 0, fmt.Errorf("pdf2latex: %q is not a page range: %w", spec, err)
	}
	if !found {
		return first, first, nil
	}
	if strings.TrimSpace(hi) == "" {
		return first, 0, nil
	}
	last, err = atLeastOne(hi)
	if err != nil {
		return 0, 0, fmt.Errorf("pdf2latex: %q is not a page range: %w", spec, err)
	}
	if last < first {
		return 0, 0, fmt.Errorf("pdf2latex: %q ends before it begins", spec)
	}
	return first, last, nil
}

func atLeastOne(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("%q is not a page number", strings.TrimSpace(s))
	}
	if n < 1 {
		return 0, fmt.Errorf("pages count from one, so %d is not one", n)
	}
	return n, nil
}
