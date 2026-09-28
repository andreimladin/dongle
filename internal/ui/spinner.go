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

// StartSpinner starts a spinner with a present-tense msg (e.g.
// "Installing deploy...") and returns once msg is on screen, so call it
// immediately before the slow work it describes, then finish with
// Success, Warn, Fail or Stop.
func StartSpinner(msg string) *Spinner {
	s := &Spinner{msg: msg}
	if !IsTerminal(os.Stderr) {
		fmt.Fprintln(os.Stderr, msg)
		return s
	}
	// The first frame is drawn here, synchronously, rather than by the
	// animating goroutine: CPU-bound work started right after this call
	// can otherwise starve that goroutine (notably with GOMAXPROCS=1), and
	// the message would first appear only as the work finishes.
	s.frame(0)
	s.stop, s.done = make(chan struct{}), make(chan struct{})
	go s.run()
	return s
}

func (s *Spinner) frame(i int) {
	fmt.Fprintf(os.Stderr, "\r\033[K%s %s", Err.Cyan(spinnerFrames[i%len(spinnerFrames)]), s.msg)
}

func (s *Spinner) run() {
	defer close(s.done)
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	for i := 1; ; i++ {
		select {
		case <-s.stop:
			fmt.Fprint(os.Stderr, "\r\033[K")
			return
		case <-t.C:
		}
		s.frame(i)
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

// Success stops the spinner and prints a completed-step line (Successf),
// in the past tense (e.g. "Installed deploy 1.2.0").
func (s *Spinner) Success(format string, a ...any) {
	s.Stop()
	Successf(format, a...)
}

// Warn stops the spinner and prints a warning line (Warnf), for a step
// that failed without stopping the command.
func (s *Spinner) Warn(format string, a ...any) {
	s.Stop()
	Warnf(format, a...)
}

// Fail stops the spinner and prints an error line (Errorf).
func (s *Spinner) Fail(format string, a ...any) {
	s.Stop()
	Errorf(format, a...)
}
