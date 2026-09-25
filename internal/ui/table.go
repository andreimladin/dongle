// Package ui holds dongle's terminal presentation helpers: aligned tables
// for listings. Everything here writes to an io.Writer the caller picks —
// command results go to stdout, status and diagnostics to stderr.
package ui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// colGap is the space between aligned columns.
const colGap = 4

// Table accumulates rows and prints them with every column aligned across
// all rows, including rows at different indents (so a grouped listing's
// values line up with its top-level ones). Heading lines are printed as-is
// and don't take part in alignment.
type Table struct {
	lines []tableLine
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
		var b strings.Builder
		b.WriteString(strings.Repeat(" ", l.indent))
		for i, c := range l.cols {
			b.WriteString(c)
			if i == len(l.cols)-1 {
				break
			}
			used := utf8.RuneCountInString(c)
			if i == 0 {
				used += l.indent
			}
			b.WriteString(strings.Repeat(" ", widths[i]-used+colGap))
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}
