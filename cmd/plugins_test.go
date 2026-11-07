package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/host"
	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/state"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestSearchSyncFlagsReachTheIndex(t *testing.T) {
	cases := []struct {
		flag   string
		stderr string // what that sync mode makes index.Prepare say
	}{
		{"", ""}, // fresh cache, default mode: silent
		{"--no-sync", "Using cached plugin index 2024.1 (--no-sync)."},
		// --sync checks the feed despite the fresh cache; with no az on
		// PATH that check fails and the cache is used.
		{"--sync", "could not check the feed for a newer plugin index"},
	}
	for _, c := range cases {
		t.Run("flag="+c.flag, func(t *testing.T) {
			testutil.Isolate(t)
			cacheIndex(t)
			args := []string{"search"}
			if c.flag != "" {
				args = append(args, c.flag)
			}
			code, stdout, stderr := run(t, args...)
			if code != 0 {
				t.Fatalf("exit %d; stderr:\n%s", code, stderr)
			}
			mustContain(t, "stdout", stdout, "deploy", "Ship it")
			if c.stderr != "" {
				mustContain(t, "stderr", stderr, c.stderr)
			} else if strings.Contains(stderr, "plugin index") {
				t.Errorf("default mode with a fresh cache should not mention the index:\n%s", stderr)
			}
		})
	}
}

func TestSyncMode(t *testing.T) {
	cases := []struct {
		args []string
		want index.SyncMode
	}{
		{nil, index.SyncAsk},
		{[]string{"--sync"}, index.SyncAlways},
		{[]string{"--no-sync"}, index.SyncNever},
		{[]string{"--sync=false"}, index.SyncAsk},
	}
	for _, c := range cases {
		cmd := &cobra.Command{}
		addSyncFlags(cmd)
		if err := cmd.ParseFlags(c.args); err != nil {
			t.Fatal(err)
		}
		if got := syncMode(cmd); got != c.want {
			t.Errorf("syncMode(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestSyncFlagsOnlyOnIndexReadingCommands(t *testing.T) {
	// search/install/upgrade consult the index and take --sync/--no-sync;
	// remove doesn't, so the flags are a usage error there.
	for _, name := range []string{"search", "install", "upgrade"} {
		c, _, err := rootCmd.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"sync", "no-sync"} {
			if c.Flags().Lookup(flag) == nil {
				t.Errorf("%s has no --%s flag", name, flag)
			}
		}
	}
	testutil.Isolate(t)
	code, _, stderr := run(t, "remove", "deploy", "--sync")
	if code != 2 {
		t.Fatalf("remove --sync: exit %d, want 2", code)
	}
	mustContain(t, "stderr", stderr, "unknown flag: --sync")
}

func TestPluginCommandArgCounts(t *testing.T) {
	for _, args := range [][]string{
		{"install"}, {"install", "a", "b"},
		{"remove"}, {"remove", "a", "b"},
		{"upgrade", "a", "b"},
	} {
		testutil.Isolate(t)
		if code, _, stderr := run(t, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2 (usage error); stderr:\n%s", args, code, stderr)
		}
	}
}

func TestUpgradeNameIsOptional(t *testing.T) {
	testutil.Isolate(t)
	// No name: upgrade everything installed (here, nothing).
	code, _, stderr := run(t, "upgrade")
	if code != 0 {
		t.Fatalf("upgrade: exit %d; stderr:\n%s", code, stderr)
	}
	mustContain(t, "stderr", stderr, "No plugins installed; nothing to upgrade.")

	// A name: upgrade just that plugin, which must be installed.
	code, _, stderr = run(t, "upgrade", "deploy")
	if code != 1 {
		t.Fatalf("upgrade deploy: exit %d, want 1", code)
	}
	mustContain(t, "stderr", stderr, "deploy is not installed; use `"+host.Name+" install deploy`")
}

func TestInstallAndRemove(t *testing.T) {
	testutil.Isolate(t)
	st := &state.State{Plugins: map[string]state.Installed{
		"deploy": {Name: "deploy", ActiveVersion: "1.0.0", Entrypoint: "dongle-deploy"}}}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	// Already installed: reported and left alone, no feed access needed.
	code, _, stderr := run(t, "install", "deploy")
	if code != 0 {
		t.Fatalf("install: exit %d; stderr:\n%s", code, stderr)
	}
	mustContain(t, "stderr", stderr, "deploy is already installed (version 1.0.0)")

	code, stdout, _ := run(t, "remove", "deploy")
	if code != 0 {
		t.Fatalf("remove: exit %d", code)
	}
	mustContain(t, "stdout", stdout, "Removed deploy")
	if st, _ := state.Load(); len(st.Plugins) != 0 {
		t.Fatalf("still installed: %+v", st.Plugins)
	}
}
