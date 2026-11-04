package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/testutil"
)

func TestTableAlignsAcrossIndentsAndSkipsHeadings(t *testing.T) {
	var tb Table
	tb.Row(0, "index", "2024.01.01")
	tb.Heading("plugins:")
	tb.Row(2, "deploy", "1.2.0", "installed")
	tb.Row(2, "x", "10.0.0", "")
	var buf bytes.Buffer
	tb.Write(&buf)

	want := strings.Join([]string{
		"index       2024.01.01",
		"plugins:",
		"  deploy    1.2.0       installed",
		"  x         10.0.0",
		"",
	}, "\n")
	if buf.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestTableStyleAppliedAfterPadding(t *testing.T) {
	var tb Table
	tb.Style(0, func(s string) string { return "<" + s + ">" })
	tb.Row(0, "a", "1")
	tb.Row(0, "long", "2")
	var buf bytes.Buffer
	tb.Write(&buf)
	// Padding is computed from the raw text, so the styled cell's markers
	// don't shift the second column.
	want := "<a>       1\n<long>    2\n"
	if buf.String() != want {
		t.Fatalf("got %q, want %q", buf.String(), want)
	}
}

func TestPalette(t *testing.T) {
	off, on := Palette{}, Palette{on: true}
	if off.Enabled() || !on.Enabled() {
		t.Fatal("Enabled() disagrees with the palette state")
	}
	if got := off.Red("x"); got != "x" {
		t.Errorf("plain Red = %q", got)
	}
	if got := on.Red("x"); got != "\033[31mx\033[0m" {
		t.Errorf("styled Red = %q", got)
	}
	if got := on.Bold(""); got != "" {
		t.Errorf("styling an empty string should emit nothing, got %q", got)
	}
	for name, fn := range map[string]func(string) string{
		"Bold": on.Bold, "Dim": on.Dim, "Green": on.Green, "Yellow": on.Yellow, "Cyan": on.Cyan,
	} {
		if got := fn("x"); !strings.HasPrefix(got, "\033[") || !strings.Contains(got, "x") {
			t.Errorf("%s(x) = %q, want an escape-wrapped x", name, got)
		}
	}
	if off.Arrow() != "->" || on.Arrow() != "→" {
		t.Errorf("Arrow: plain %q, styled %q", off.Arrow(), on.Arrow())
	}
	if got := off.mark("✓", on.Green, "done"); got != "done" {
		t.Errorf("plain mark = %q", got)
	}
	if got := on.mark("✓", func(s string) string { return s }, "done"); got != "✓ done" {
		t.Errorf("styled mark = %q", got)
	}
}

func TestColorEnabled(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	t.Setenv("TERM", "xterm")
	t.Setenv("NO_COLOR", "") // registers the restore before unsetting
	os.Unsetenv("NO_COLOR")
	if colorEnabled(f) {
		t.Error("a regular file is not a terminal; color must be off")
	}
	os.Setenv("NO_COLOR", "")
	if colorEnabled(os.Stdout) {
		t.Error("NO_COLOR (even empty) must disable color")
	}
	os.Unsetenv("NO_COLOR")
	t.Setenv("TERM", "dumb")
	if colorEnabled(os.Stdout) {
		t.Error("TERM=dumb must disable color")
	}
}

func TestStatusLinesGoToStderrAndResultsToStdout(t *testing.T) {
	stdout, stderr := testutil.Capture(t, func() {
		Errorf("bad %d", 1)
		Warnf("careful %s", "now")
		Notef("fyi")
		Infof("info")
		Successf("ok")
		Resultf("result %s", "here")
	})
	if stdout != "result here\n" {
		t.Errorf("stdout = %q, want only the result", stdout)
	}
	for _, want := range []string{"error: bad 1\n", "warning: careful now\n", "note: fyi\n", "info\n", "ok\n"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr)
		}
	}
}

func TestSpinnerOffTerminalPrintsOncePlain(t *testing.T) {
	_, stderr := testutil.Capture(t, func() {
		sp := StartSpinner("Downloading deploy...")
		sp.Stop()
		sp.Stop() // idempotent
		StartSpinner("Second...").Success("Done %s", "now")
	})
	if stderr != "Downloading deploy...\nSecond...\nDone now\n" {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stderr, "\033") || strings.Contains(stderr, "\r") {
		t.Fatal("non-terminal spinner emitted control characters")
	}
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"YES\n", true},
		{"  yes  \n", true},
		{"n\n", false},
		{"\n", false},
		{"", false}, // EOF
		{"yep\n", false},
	}
	for _, c := range cases {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		w.WriteString(c.input)
		w.Close()
		orig := os.Stdin
		os.Stdin = r
		var got bool
		_, stderr := testutil.Capture(t, func() { got = Confirm("Proceed?") })
		os.Stdin = orig
		r.Close()
		if got != c.want {
			t.Errorf("Confirm with input %q = %v, want %v", c.input, got, c.want)
		}
		if stderr != "Proceed? [y/N] " {
			t.Errorf("prompt = %q", stderr)
		}
	}
}

func TestIsTerminalFalseForPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminal(r) || IsTerminal(w) {
		t.Fatal("a pipe is not a terminal")
	}
	orig := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = orig }()
	if StdinIsTerminal() {
		t.Fatal("piped stdin must not be promptable")
	}
}
