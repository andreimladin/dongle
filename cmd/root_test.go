package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}} {
		testutil.Isolate(t)
		code, stdout, _ := run(t, args...)
		if code != 0 {
			t.Fatalf("%v: exit %d", args, code)
		}
		mustContain(t, "help", stdout, "dongle <command> [flags]", "dongle <plugin> [args...]", "search", "install")
		if strings.Contains(stdout, "completion") {
			t.Errorf("%v: completion command should be disabled", args)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	testutil.Isolate(t)
	code, stdout, _ := run(t, "--version")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stdout", stdout, "dongle", hostVersion, "no plugins installed")
}

func TestUnknownCommands(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		stderr []string
	}{
		{"unknown flag", []string{"--bogus"}, []string{`error: unknown flag "--bogus"`, "Usage:"}},
		{"unknown command", []string{"frobnicate"}, []string{`error: command "frobnicate" is not supported`, "Usage:"}},
		// Only prefix matches are suggested today: cobra leaves
		// SuggestionsMinimumDistance at 0 when SuggestionsFor is called
		// directly, so a typo like "serch" gets no suggestion.
		{"prefix gets a suggestion", []string{"inst"}, []string{`command "inst" is not supported`, "Did you mean: install?"}},
		// `help` is not a builtin: --help/-h is.
		{"help is not a command", []string{"help"}, []string{`command "help" is not supported`}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Isolate(t)
			code, stdout, stderr := run(t, c.args...)
			if code != 2 {
				t.Fatalf("exit %d, want 2 (usage error)", code)
			}
			if stdout != "" {
				t.Errorf("usage errors go to stderr; stdout = %q", stdout)
			}
			mustContain(t, "stderr", stderr, c.stderr...)
		})
	}
}

func TestCobraUsageErrorsExitTwo(t *testing.T) {
	cases := []struct {
		args   []string
		stderr []string
	}{
		{[]string{"install"}, []string{"error: accepts 1 arg(s), received 0", "Run 'dongle install --help' for usage."}},
		{[]string{"search", "extra"}, []string{"error: unknown command \"extra\"", "Run 'dongle search --help' for usage."}},
		{[]string{"search", "--bogus"}, []string{"error: unknown flag: --bogus", "Run 'dongle search --help' for usage."}},
		{[]string{"search", "--sync", "--no-sync"}, []string{"error:", "sync", "no-sync"}},
	}
	for _, c := range cases {
		testutil.Isolate(t)
		code, _, stderr := run(t, c.args...)
		if code != 2 {
			t.Errorf("%v: exit %d, want 2", c.args, code)
		}
		mustContain(t, strings.Join(c.args, " "), stderr, c.stderr...)
	}
}

func TestBuiltinExitCodePropagates(t *testing.T) {
	testutil.Isolate(t)
	code, _, stderr := run(t, "remove", "deploy")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	mustContain(t, "stderr", stderr, "error: deploy is not installed")
	// exitError carries no message of its own; cobra must not add one.
	if strings.Count(stderr, "error:") != 1 {
		t.Fatalf("want exactly one error line, got:\n%s", stderr)
	}
}

func TestExitCode(t *testing.T) {
	if exitCode(0) != nil {
		t.Fatal("0 must be success")
	}
	err := exitCode(3)
	ee, ok := err.(*exitError)
	if !ok || ee.code != 3 || err.Error() != "" {
		t.Fatalf("exitCode(3) = %#v", err)
	}
}

func TestInstalledPluginIsDispatched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script plugin")
	}
	testutil.Isolate(t)
	out := filepath.Join(t.TempDir(), "args")
	dir := state.PluginVersionDir("deploy", "1.0.0")
	os.MkdirAll(dir, 0o755)
	script := "#!/bin/sh\necho \"$DONGLE_VERSION $DONGLE_PROTOCOL $*\" > " + out + "\nexit 5\n"
	if err := os.WriteFile(filepath.Join(dir, "dongle-deploy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	st := &state.State{Plugins: map[string]state.Installed{
		"deploy": {Name: "deploy", ActiveVersion: "1.0.0", Entrypoint: "dongle-deploy"}}}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	// Flags after the plugin name — even --help and --version — belong to
	// the plugin and arrive unparsed.
	code, _, stderr := run(t, "deploy", "--help", "--version", "-x", "prod")
	if code != 5 {
		t.Fatalf("exit %d, want the plugin's 5; stderr:\n%s", code, stderr)
	}
	b, _ := os.ReadFile(out)
	if got := strings.TrimSpace(string(b)); got != hostVersion+" "+protocol+" --help --version -x prod" {
		t.Fatalf("plugin saw %q", got)
	}
}

func TestUnreadableStateWhenResolvingPlugin(t *testing.T) {
	testutil.Isolate(t)
	os.WriteFile(filepath.Join(state.DataDir(), "state.json"), []byte("{bad"), 0o644)
	code, _, stderr := run(t, "deploy")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	mustContain(t, "stderr", stderr, "error: reading state:")
}
