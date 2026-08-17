package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/andreimladin/dongle/internal/index"
	"github.com/andreimladin/dongle/internal/plugincmd"
)

// pluginCmd's own RunE only fires when cobra's Find couldn't match a deeper
// subcommand (no subcommand given, or an unrecognized one) — the known
// subcommands below are matched and handled directly by cobra. Either way
// the args are handed to plugincmd.Run exactly as the old hand-rolled
// switch did, so usage text and exit codes are unchanged.
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "manage plugins",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, args))
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "list installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, []string{"list"}))
	},
}

var pluginSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "list plugins available in the index",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, []string{"search"}))
	},
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "install a plugin from the index",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, append([]string{"install"}, args...)))
	},
}

var pluginUninstallCmd = &cobra.Command{
	Use:   "uninstall <name>",
	Short: "uninstall a plugin",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(plugincmd.Run(hostVersion, protocol, append([]string{"uninstall"}, args...)))
	},
}

// resolveIndexTTL mirrors internal/plugincmd's own index cache TTL — only
// relevant here when --index isn't given and resolve falls back to the
// normal (refresh-if-stale) index cache lookup.
const resolveIndexTTL = 24 * time.Hour

var (
	resolveVersion string
	resolveOS      string
	resolveArch    string
	resolveIndex   string
	resolveFormat  string
)

// pluginResolveCmd is a thin, read-only wrapper over internal/index: it
// resolves a plugin's Azure Artifacts feed coordinates for an arbitrary
// os/arch and prints them, without downloading or installing anything.
// scripts/build-release.sh shells out to it so the release script never has
// to know the manifest's YAML shape (or hardcode feed coordinates) itself.
var pluginResolveCmd = &cobra.Command{
	Use:   "resolve <name>",
	Short: "print a plugin's feed coordinates for a given os/arch (read-only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return exitCode(runPluginResolve(args[0]))
	},
}

func runPluginResolve(name string) int {
	m, err := loadManifestForResolve(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	plat, err := m.PlatformForOSArch(resolveOS, resolveArch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	version := strings.TrimPrefix(resolveVersion, "v") // upack versions are bare semver

	switch resolveFormat {
	case "env":
		fmt.Printf("ORG=%q\n", m.Feed.Organization)
		fmt.Printf("FEED=%q\n", m.Feed.Feed)
		fmt.Printf("PROJECT=%q\n", m.Feed.Project)
		fmt.Printf("PACKAGE=%q\n", plat.Package)
		fmt.Printf("VERSION=%q\n", version)
	case "json":
		b, err := json.MarshalIndent(map[string]string{
			"org":     m.Feed.Organization,
			"feed":    m.Feed.Feed,
			"project": m.Feed.Project,
			"package": plat.Package,
			"version": version,
		}, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println(string(b))
	default:
		fmt.Fprintf(os.Stderr, "error: unknown --format %q (want env or json)\n", resolveFormat)
		return 2
	}
	return 0
}

// loadManifestForResolve reads the plugin's manifest either straight from a
// local index checkout (--index — no clone, no refresh) or, absent that,
// from the normal TTL-refreshed index cache.
func loadManifestForResolve(name string) (*index.Manifest, error) {
	if resolveIndex != "" {
		return index.LoadFromDir(resolveIndex, name)
	}
	if err := index.EnsureFresh(resolveIndexTTL); err != nil {
		return nil, err
	}
	return index.Load(name)
}

func init() {
	pluginResolveCmd.Flags().StringVar(&resolveVersion, "version", "", "plugin version (required)")
	pluginResolveCmd.Flags().StringVar(&resolveOS, "os", "", "target GOOS (required)")
	pluginResolveCmd.Flags().StringVar(&resolveArch, "arch", "", "target GOARCH (required)")
	pluginResolveCmd.Flags().StringVar(&resolveIndex, "index", "", "read plugins/<name>.yaml directly from this local index checkout, bypassing the cache")
	pluginResolveCmd.Flags().StringVar(&resolveFormat, "format", "env", "output format: env or json")
	_ = pluginResolveCmd.MarkFlagRequired("version")
	_ = pluginResolveCmd.MarkFlagRequired("os")
	_ = pluginResolveCmd.MarkFlagRequired("arch")

	pluginCmd.AddCommand(pluginListCmd, pluginSearchCmd, pluginInstallCmd, pluginUninstallCmd, pluginResolveCmd)
}
