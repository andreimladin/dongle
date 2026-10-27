package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/index"
)

const manifestHead = `name: deploy
version: v2.3.1
requires:
  host: ">=0.1.0"
  protocol: "v1"
feed:
  organization: acme
  feed: dongle-plugins
  packageType: upack
  packageName: dongle-deploy
support:
  documentation: https://docs.acme.internal/dongle-deploy
  channel: https://acme.slack.com/archives/C0DEPLOY
platforms:
`

const (
	darwinEntry  = "  - selector: { os: darwin, arch: arm64 }\n    package: dongle-deploy_2.3.1_darwin_arm64\n"
	windowsEntry = "  - selector: { os: windows, arch: amd64 }\n    package: dongle-deploy_2.3.1_windows_amd64\n"
	linuxEntry   = "  - selector: { os: linux, arch: amd64 }\n    package: dongle-deploy_2.3.1_linux_amd64\n"
)

func writeManifest(t *testing.T, platforms ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deploy.yaml")
	body := manifestHead + strings.Join(platforms, "")
	if len(platforms) == 0 {
		body += "  []\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckCorrectnessRequiresAllPlatforms(t *testing.T) {
	tests := []struct {
		name      string
		platforms []string
		wantErrs  []string
	}{
		{"all three", []string{darwinEntry, windowsEntry, linuxEntry}, nil},
		{"missing linux", []string{darwinEntry, windowsEntry}, []string{
			"manifest deploy.yaml is missing a Linux platform entry (selector.os: linux)",
		}},
		{"missing windows", []string{darwinEntry, linuxEntry}, []string{
			"manifest deploy.yaml is missing a Windows platform entry (selector.os: windows)",
		}},
		{"missing macOS", []string{windowsEntry, linuxEntry}, []string{
			"manifest deploy.yaml is missing a macOS platform entry (selector.os: darwin)",
		}},
		{"linux only", []string{linuxEntry}, []string{
			"manifest deploy.yaml is missing a macOS platform entry (selector.os: darwin)",
			"manifest deploy.yaml is missing a Windows platform entry (selector.os: windows)",
		}},
		{"none", nil, []string{
			"platforms: at least one platform entry is required",
			"manifest deploy.yaml is missing a macOS platform entry (selector.os: darwin)",
			"manifest deploy.yaml is missing a Windows platform entry (selector.os: windows)",
			"manifest deploy.yaml is missing a Linux platform entry (selector.os: linux)",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := checkCorrectness(writeManifest(t, tt.platforms...))
			if strings.Join(errs, "\n") != strings.Join(tt.wantErrs, "\n") {
				t.Errorf("errs:\n%s\nwant:\n%s", strings.Join(errs, "\n"), strings.Join(tt.wantErrs, "\n"))
			}
		})
	}
}

func TestCheckExistenceReportsEachPlatform(t *testing.T) {
	m, errs := checkCorrectness(writeManifest(t, darwinEntry, windowsEntry, linuxEntry))
	if len(errs) > 0 {
		t.Fatalf("unexpected correctness errors: %v", errs)
	}

	orig := packageExists
	t.Cleanup(func() { packageExists = orig })
	var asked []string
	packageExists = func(feed index.Feed, pkg, version string) error {
		asked = append(asked, pkg+"@"+version)
		if strings.Contains(pkg, "linux") {
			return errors.New("az artifacts universal download: exit status 1: package not found")
		}
		return nil
	}

	results := checkExistence(m, true)
	want := []string{
		"dongle-deploy_2.3.1_darwin_arm64@2.3.1",
		"dongle-deploy_2.3.1_windows_amd64@2.3.1",
		"dongle-deploy_2.3.1_linux_amd64@2.3.1",
	}
	if strings.Join(asked, ",") != strings.Join(want, ",") {
		t.Fatalf("checked %v, want %v", asked, want)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.os == "linux" {
			if r.err == nil || !strings.HasPrefix(r.err.Error(), `deploy: linux executable not found in feed "dongle-plugins": `) {
				t.Errorf("linux err = %v", r.err)
			}
		} else if r.err != nil {
			t.Errorf("%s err = %v, want nil", r.os, r.err)
		}
	}
}

func TestCheckExistenceWithoutAz(t *testing.T) {
	m, _ := checkCorrectness(writeManifest(t, darwinEntry, windowsEntry, linuxEntry))
	orig := packageExists
	t.Cleanup(func() { packageExists = orig })
	packageExists = func(index.Feed, string, string) error {
		t.Fatal("packageExists called without az")
		return nil
	}
	for _, r := range checkExistence(m, false) {
		if r.err == nil {
			t.Errorf("%s/%s: expected failure without az", r.os, r.arch)
		}
	}
}
