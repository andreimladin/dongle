package ui

import (
	"fmt"
	"os"
)

// Palette styles text for one output stream. Styling is on only when that
// stream is a terminal (and NO_COLOR / TERM=dumb don't say otherwise), so
// piped or redirected output is always plain text.
type Palette struct{ on bool }

// Out styles text written to stdout; Err styles text written to stderr.
var (
	Out = Palette{on: colorEnabled(os.Stdout)}
	Err = Palette{on: colorEnabled(os.Stderr)}
)

func colorEnabled(f *os.File) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return IsTerminal(f)
}

// Enabled reports whether this palette emits styling (i.e. its stream is
// an interactive terminal).
func (p Palette) Enabled() bool { return p.on }

func (p Palette) wrap(code, s string) string {
	if !p.on || s == "" {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func (p Palette) Bold(s string) string   { return p.wrap("1", s) }
func (p Palette) Dim(s string) string    { return p.wrap("2", s) }
func (p Palette) Red(s string) string    { return p.wrap("31", s) }
func (p Palette) Green(s string) string  { return p.wrap("32", s) }
func (p Palette) Yellow(s string) string { return p.wrap("33", s) }
func (p Palette) Cyan(s string) string   { return p.wrap("36", s) }

// Arrow is "→" on a terminal and "->" otherwise (plain ASCII for logs).
func (p Palette) Arrow() string {
	if p.on {
		return "→"
	}
	return "->"
}

// mark prefixes s with symbol (styled by style) on a terminal; plain
// output gets s alone.
func (p Palette) mark(symbol string, style func(string) string, s string) string {
	if !p.on {
		return s
	}
	return style(symbol) + " " + s
}

// --- status lines (stderr) ----------------------------------------------------
//
// Status and diagnostics always go to stderr so stdout carries nothing but
// a command's results.

// Errorf prints "error: <msg>" to stderr — the host's standard error line.
func Errorf(format string, a ...any) {
	fmt.Fprintln(os.Stderr, Err.Bold(Err.Red("error:")), fmt.Sprintf(format, a...))
}

// Warnf prints "warning: <msg>" to stderr.
func Warnf(format string, a ...any) {
	fmt.Fprintln(os.Stderr, Err.Bold(Err.Yellow("warning:")), fmt.Sprintf(format, a...))
}

// Notef prints "note: <msg>" to stderr, for something the user may want
// to act on but that isn't a problem.
func Notef(format string, a ...any) {
	fmt.Fprintln(os.Stderr, Err.Bold(Err.Cyan("note:")), fmt.Sprintf(format, a...))
}

// Infof prints a neutral status line to stderr.
func Infof(format string, a ...any) {
	fmt.Fprintln(os.Stderr, Err.mark("•", Err.Dim, fmt.Sprintf(format, a...)))
}

// Successf prints a completed-step status line to stderr.
func Successf(format string, a ...any) {
	fmt.Fprintln(os.Stderr, Err.mark("✓", Err.Green, fmt.Sprintf(format, a...)))
}

// --- results (stdout) -----------------------------------------------------------

// Resultf prints a command's success result to stdout, check-marked on a
// terminal.
func Resultf(format string, a ...any) {
	fmt.Fprintln(os.Stdout, Out.mark("✓", Out.Green, fmt.Sprintf(format, a...)))
}
