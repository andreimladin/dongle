// Package azcli is dongle's single seam onto the system Azure CLI: the
// index feed queries and downloads (internal/index) and plugin artifact
// downloads (internal/builtins) all go through a Runner, so tests can
// substitute a fake instead of shelling out to a real `az`.
package azcli

import (
	"io"
	"os/exec"
)

// Runner runs Azure CLI commands.
type Runner interface {
	// LookPath reports an error when az isn't installed (not on PATH).
	LookPath() error
	// Run runs `az <args...>`, writing its output to stdout and stderr
	// (either may be nil to discard it).
	Run(args []string, stdout, stderr io.Writer) error
}

// System is the production Runner: it execs the `az` found on PATH.
type System struct{}

func (System) LookPath() error {
	_, err := exec.LookPath("az")
	return err
}

func (System) Run(args []string, stdout, stderr io.Writer) error {
	cmd := exec.Command("az", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
