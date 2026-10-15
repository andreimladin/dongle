package dispatch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/compat"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

// fakeRunner records the plugin process it was asked to run instead of
// running it.
type fakeRunner struct {
	err  error
	cmds []*exec.Cmd
}

func (f *fakeRunner) Run(cmd *exec.Cmd) error {
	f.cmds = append(f.cmds, cmd)
	return f.err
}

func withRunner(t *testing.T, r processRunner) {
	t.Helper()
	orig := runner
	runner = r
	t.Cleanup(func() { runner = orig })
}

func install(t *testing.T, inst state.Installed) {
	t.Helper()
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	st.Plugins[inst.Name] = inst
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
}

func corruptState(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(state.DataDir(), "state.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsInstalled(t *testing.T) {
	testutil.Isolate(t)
	install(t, state.Installed{Name: "deploy", ActiveVersion: "1.0.0"})
	if ok, err := IsInstalled("deploy"); err != nil || !ok {
		t.Fatalf("deploy: %v, %v", ok, err)
	}
	if ok, err := IsInstalled("other"); err != nil || ok {
		t.Fatalf("other: %v, %v", ok, err)
	}
	corruptState(t)
	if _, err := IsInstalled("deploy"); err == nil {
		t.Fatal("want error for unreadable state")
	}
}

func TestRunExecsPluginWithContract(t *testing.T) {
	testutil.Isolate(t)
	t.Setenv("INHERITED_VAR", "kept")
	install(t, state.Installed{Name: "deploy", ActiveVersion: "1.2.0", Entrypoint: "dongle-deploy",
		Requires: compat.Requires{Host: ">=1.0.0", Protocol: "v1"}})
	f := &fakeRunner{}
	withRunner(t, f)

	if code := Run("1.5.0", "v1", "deploy", []string{"--env", "prod", "-x"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(f.cmds) != 1 {
		t.Fatalf("ran %d processes", len(f.cmds))
	}
	cmd := f.cmds[0]
	if want := filepath.Join(state.PluginVersionDir("deploy", "1.2.0"), "dongle-deploy"); cmd.Path != want {
		t.Errorf("Path = %q, want %q", cmd.Path, want)
	}
	if got := strings.Join(cmd.Args[1:], " "); got != "--env prod -x" {
		t.Errorf("args = %q, want them passed through unchanged", got)
	}
	if cmd.Stdin != os.Stdin || cmd.Stdout != os.Stdout || cmd.Stderr != os.Stderr {
		t.Error("plugin must inherit the host's stdin/stdout/stderr")
	}
	env := strings.Join(cmd.Env, "\n") + "\n"
	for _, want := range []string{"DONGLE_VERSION=1.5.0\n", "DONGLE_PROTOCOL=v1\n", "DONGLE_PLUGIN_NAME=deploy\n", "INHERITED_VAR=kept\n"} {
		if !strings.Contains(env, want) {
			t.Errorf("env missing %q", strings.TrimSpace(want))
		}
	}
}

func TestRunRefusesBeforeExec(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T)
		plugin   string
		wantCode int
		wantErr  string
	}{
		{"not installed", func(t *testing.T) {}, "nope", 127, `plugin "nope" is not installed`},
		{"unreadable state", corruptState, "deploy", 1, "error: reading state:"},
		{"host too old", func(t *testing.T) {
			install(t, state.Installed{Name: "deploy", ActiveVersion: "1.0.0", Requires: compat.Requires{Host: ">=2.0.0"}})
		}, "deploy", 1, "deploy requires host >=2.0.0 but this host is 1.5.0 — upgrade dongle"},
		{"protocol mismatch", func(t *testing.T) {
			install(t, state.Installed{Name: "deploy", ActiveVersion: "1.0.0", Requires: compat.Requires{Protocol: "v2"}})
		}, "deploy", 1, `speaks protocol "v2"`},
		{"bad constraint", func(t *testing.T) {
			install(t, state.Installed{Name: "deploy", ActiveVersion: "1.0.0", Requires: compat.Requires{Host: "^1"}})
		}, "deploy", 1, "bad constraint in state"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Isolate(t)
			c.setup(t)
			f := &fakeRunner{}
			withRunner(t, f)
			var code int
			_, stderr := testutil.Capture(t, func() { code = Run("1.5.0", "v1", c.plugin, nil) })
			if code != c.wantCode {
				t.Fatalf("exit %d, want %d", code, c.wantCode)
			}
			if !strings.Contains(stderr, c.wantErr) {
				t.Fatalf("stderr missing %q:\n%s", c.wantErr, stderr)
			}
			if len(f.cmds) != 0 {
				t.Fatal("plugin was executed despite the refusal")
			}
		})
	}
}

func TestRunLaunchFailure(t *testing.T) {
	testutil.Isolate(t)
	install(t, state.Installed{Name: "deploy", ActiveVersion: "1.0.0", Entrypoint: "dongle-deploy"})
	withRunner(t, &fakeRunner{err: errors.New("permission denied")})
	var code int
	_, stderr := testutil.Capture(t, func() { code = Run("1.0.0", "v1", "deploy", nil) })
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr, "launching deploy: permission denied") {
		t.Fatalf("stderr:\n%s", stderr)
	}
}

// TestRunRealProcess runs an actual plugin process (a shell script) to
// check what the fake can't: a plugin's own exit code is passed through
// verbatim, and a missing binary is a launch error, not a plugin exit.
func TestRunRealProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script plugin")
	}
	testutil.Isolate(t)
	dir := state.PluginVersionDir("deploy", "1.0.0")
	os.MkdirAll(dir, 0o755)
	out := filepath.Join(t.TempDir(), "out")
	script := "#!/bin/sh\necho \"$DONGLE_PLUGIN_NAME $*\" > " + out + "\nexit 7\n"
	if err := os.WriteFile(filepath.Join(dir, "dongle-deploy"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	install(t, state.Installed{Name: "deploy", ActiveVersion: "1.0.0", Entrypoint: "dongle-deploy"})

	if code := Run("1.0.0", "v1", "deploy", []string{"a", "b"}); code != 7 {
		t.Fatalf("exit %d, want the plugin's own 7", code)
	}
	if b, _ := os.ReadFile(out); strings.TrimSpace(string(b)) != "deploy a b" {
		t.Fatalf("plugin saw %q", b)
	}

	install(t, state.Installed{Name: "ghost", ActiveVersion: "1.0.0", Entrypoint: "dongle-ghost"})
	var code int
	_, stderr := testutil.Capture(t, func() { code = Run("1.0.0", "v1", "ghost", nil) })
	if code != 1 || !strings.Contains(stderr, "launching ghost") {
		t.Fatalf("missing binary: exit %d, stderr:\n%s", code, stderr)
	}
}
