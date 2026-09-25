// existence.go implements layer (b) of validate-manifest: confirming a
// package exists at a given version in the Azure Artifacts feed a manifest
// names.
//
// This shells out to `az artifacts universal download` — the exact same
// command internal/builtins.downloadArtifact (dongle install)
// uses — into a throwaway temp dir that's removed immediately after: a
// successful download IS the existence proof, and its content is never
// kept or inspected. An `az rest` metadata query was tried first, but it
// does not authenticate with AZURE_DEVOPS_EXT_PAT (the access token CI
// exports for `az artifacts` commands) and fails with 401/403 there; `az
// artifacts` picks that variable up on its own, with no `az login` needed.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/andreimladin/dongle/internal/index"
)

// packageExists attempts to download packageName@version from feed into a
// throwaway temp dir, deleted before returning. feed carries the
// organization/project/feed coordinates straight from the manifest that
// declared the package — this never consults a global/configured feed. A
// nil return means the download succeeded (the package exists and is
// fetchable); a non-nil return carries the az CLI's stderr.
func packageExists(feed index.Feed, packageName, version string) error {
	dest, err := os.MkdirTemp("", "validate-manifest-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(dest)

	args := []string{
		"artifacts", "universal", "download",
		"--organization", "https://dev.azure.com/" + feed.Organization,
		"--feed", feed.Feed,
		"--name", packageName,
		"--version", version,
		"--path", dest,
	}
	if feed.Project != "" {
		args = append(args, "--project", feed.Project, "--scope", "project")
	}

	cmd := exec.Command("az", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("az artifacts universal download: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
