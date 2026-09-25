// Package ui holds dongle's terminal presentation helpers: aligned tables,
// TTY-aware color and status lines, a spinner, and a yes/no prompt.
//
// The rules everything here follows: command results go to stdout;
// status, diagnostics, prompts and spinners go to stderr; and anything
// fancy (color, symbols, animation) happens only when the stream it's
// written to is a terminal — piped or CI output is always plain text.
package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Gaps between aligned columns: wider after the first (name) column,
// narrower between the value columns that follow it.
const (
	firstGap = 4
	colGap   = 2
)

// Table accumulates rows and prints them with every column aligned across
// all rows, including rows at different indents (so a grouped listing's
// values line up with its top-level ones). Heading lines are printed as-is
// and don't take part in alignment.
type Table struct {
	lines  []tableLine
	styles map[int]func(string) string
}

// Style sets a styling function (e.g. a Palette's Bold) for column col.
// It's applied after alignment, so escape codes never skew the padding.
func (t *Table) Style(col int, fn func(string) string) {
	if t.styles == nil {
		t.styles = map[int]func(string) string{}
	}
	t.styles[col] = fn
}

type tableLine struct {
	heading string
	indent  int
	cols    []string
}

// Heading adds an unaligned line (e.g. a group title like "plugins:").
func (t *Table) Heading(s string) { t.lines = append(t.lines, tableLine{heading: s}) }

// Row adds an aligned row indented by indent spaces.
func (t *Table) Row(indent int, cols ...string) {
	t.lines = append(t.lines, tableLine{indent: indent, cols: cols})
}

// Write prints the table to w.
func (t *Table) Write(w io.Writer) {
	var widths []int
	for _, l := range t.lines {
		for i, c := range l.cols {
			n := utf8.RuneCountInString(c)
			if i == 0 {
				n += l.indent
			}
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			if n > widths[i] {
				widths[i] = n
			}
		}
	}
	for _, l := range t.lines {
		if l.cols == nil {
			fmt.Fprintln(w, l.heading)
			continue
		}
		// Trailing empty cells print nothing, not even padding.
		cols := l.cols
		for len(cols) > 1 && cols[len(cols)-1] == "" {
			cols = cols[:len(cols)-1]
		}
		var b strings.Builder
		b.WriteString(strings.Repeat(" ", l.indent))
		for i, c := range cols {
			if st := t.styles[i]; st != nil {
				b.WriteString(st(c))
			} else {
				b.WriteString(c)
			}
			if i == len(cols)-1 {
				break
			}
			used, gap := utf8.RuneCountInString(c), colGap
			if i == 0 {
				used, gap = used+l.indent, firstGap
			}
			b.WriteString(strings.Repeat(" ", widths[i]-used+gap))
		}
		fmt.Fprintln(w, b.String())
	}
}
