// Package plugincmd implements the `dongle plugin ...` builtins.
package plugincmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/andreimladin/dongle/internal/manifest"
	"github.com/andreimladin/dongle/internal/state"
)

// Run handles `dongle plugin <subcommand>`.
func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dongle plugin <list|install|uninstall> ...")
		return 2
	}
	switch args[0] {
	case "list":
		return list()
	case "install":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: dongle plugin install <dir>")
			return 2
		}
		return install(args[1])
	case "uninstall":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: dongle plugin uninstall <name>")
			return 2
		}
		return uninstall(args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown plugin subcommand %q\n", args[0])
		return 2
	}
}

func list() int {
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(st.Plugins) == 0 {
		fmt.Println("no plugins installed")
		return 0
	}
	names := make([]string, 0, len(st.Plugins))
	for n := range st.Plugins {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("%-16s %s\n", n, st.Plugins[n].ActiveVersion)
	}
	return 0
}

// install copies a locally-built plugin (a dir with plugin.json + the
// entrypoint binary) into the versioned install tree and records it.
//
// In production, this instead resolves the name against an index, downloads the
// right per-platform artifact, and verifies its sha256 before unpacking.
func install(srcDir string) int {
	m, err := manifest.Load(filepath.Join(srcDir, "plugin.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	dstDir := state.PluginVersionDir(m.Name, m.Version)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := copyFile(filepath.Join(srcDir, "plugin.json"), filepath.Join(dstDir, "plugin.json"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := copyFile(filepath.Join(srcDir, m.Entrypoint), filepath.Join(dstDir, m.Entrypoint), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	st.Plugins[m.Name] = state.Installed{Name: m.Name, ActiveVersion: m.Version}
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("installed %s %s (command: dongle %s)\n", m.Name, m.Version, m.Name)
	return 0
}

func uninstall(name string) int {
	st, err := state.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if _, ok := st.Plugins[name]; !ok {
		fmt.Fprintf(os.Stderr, "error: %s is not installed\n", name)
		return 1
	}
	if err := os.RemoveAll(filepath.Join(state.PluginsDir(), name)); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	delete(st.Plugins, name)
	if err := st.Save(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("uninstalled %s\n", name)
	return 0
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}
