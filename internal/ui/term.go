package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// IsTerminal reports whether f is an interactive terminal (not a pipe,
// file, or /dev/null).
func IsTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// StdinIsTerminal reports whether the user can be prompted: stdin must be
// a terminal, or a prompt would hang (or silently read piped input) in
// scripts and CI.
func StdinIsTerminal() bool { return IsTerminal(os.Stdin) }

// Confirm asks a yes/no question on stderr and reads the answer from
// stdin. Anything but y/yes (including EOF) is "no". Callers must check
// StdinIsTerminal first.
func Confirm(question string) bool {
	fmt.Fprintf(os.Stderr, "%s [y/N] ", question)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
