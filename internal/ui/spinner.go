package ui

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Spinner shows that a long action is in progress. It always writes to
// stderr. On a terminal it animates in place and erases itself when
// stopped; otherwise (piped, CI) it prints its message once as a plain
// status line and emits no control characters at all.
//
// Nothing else may write to stderr while a spinner is running; stop it
// (Stop/Success/Fail) before printing, prompting, or handing the terminal
// to a child process.
type Spinner struct {
	msg  string
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StartSpinner starts a spinner with msg (e.g. "Downloading deploy...").
func StartSpinner(msg string) *Spinner {
	s := &Spinner{msg: msg}
	if !IsTerminal(os.Stderr) {
		fmt.Fprintln(os.Stderr, msg)
		return s
	}
	s.stop, s.done = make(chan struct{}), make(chan struct{})
	go s.run()
	return s
}

func (s *Spinner) run() {
	defer close(s.done)
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	for i := 0; ; i++ {
		fmt.Fprintf(os.Stderr, "\r\033[K%s %s", Err.Cyan(spinnerFrames[i%len(spinnerFrames)]), s.msg)
		select {
		case <-s.stop:
			fmt.Fprint(os.Stderr, "\r\033[K")
			return
		case <-t.C:
		}
	}
}

// Stop halts the spinner and erases it, printing nothing in its place.
// Safe to call more than once.
func (s *Spinner) Stop() {
	s.once.Do(func() {
		if s.stop != nil {
			close(s.stop)
			<-s.done
		}
	})
}

// Success stops the spinner and prints a completed-step line (Successf).
func (s *Spinner) Success(format string, a ...any) {
	s.Stop()
	Successf(format, a...)
}
