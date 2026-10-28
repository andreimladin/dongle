package main

import (
	"errors"
	"fmt"
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

// entry renders one platforms[] item for an "os/arch" pair.
func entry(osArch string) string {
	goos, goarch, _ := strings.Cut(osArch, "/")
	return fmt.Sprintf("  - selector: { os: %s, arch: %s }\n    package: dongle-deploy_2.3.1_%s_%s\n", goos, goarch, goos, goarch)
}

// allExcept returns every required os/arch pair except the given ones.
func allExcept(skip ...string) []string {
	var out []string
	for _, rp := range requiredPlatforms {
		skipped := false
		for _, s := range skip {
			skipped = skipped || s == rp
		}
		if !skipped {
			out = append(out, rp)
		}
	}
	return out
}

func writeManifest(t *testing.T, platforms ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deploy.yaml")
	body := manifestHead
	for _, p := range platforms {
		body += entry(p)
	}
	if len(platforms) == 0 {
		body += "  []\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func missing(osArch string) string { return "manifest deploy.yaml is missing platform " + osArch }

func TestRequiredPlatformsAreTheSixHostTargets(t *testing.T) {
	want := "darwin/arm64 darwin/amd64 linux/arm64 linux/amd64 windows/arm64 windows/amd64"
	if got := strings.Join(requiredPlatforms, " "); got != want {
		t.Errorf("requiredPlatforms = %s, want %s", got, want)
	}
}

func TestCheckCorrectnessRequiresAllSixPlatforms(t *testing.T) {
	tests := []struct {
		name      string
		platforms []string
		wantErrs  []string
	}{
		{"all six", allExcept(), nil},
		{"all six plus an extra", append(allExcept(), "linux/386"), nil},
		{"one OS on one arch only", []string{"darwin/arm64", "windows/amd64", "linux/amd64"}, []string{
			missing("darwin/amd64"), missing("linux/arm64"), missing("windows/arm64"),
		}},
		{"OS missing entirely", allExcept("windows/arm64", "windows/amd64"), []string{
			missing("windows/arm64"), missing("windows/amd64"),
		}},
		{"none", nil, append([]string{"platforms: at least one platform entry is required"},
			missing("darwin/arm64"), missing("darwin/amd64"),
			missing("linux/arm64"), missing("linux/amd64"),
			missing("windows/arm64"), missing("windows/amd64"),
		)},
	}
	// Each of the six, missing on its own, is reported by name.
	for _, rp := range requiredPlatforms {
		tests = append(tests, struct {
			name      string
			platforms []string
			wantErrs  []string
		}{"missing " + rp, allExcept(rp), []string{missing(rp)}})
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
	m, errs := checkCorrectness(writeManifest(t, allExcept()...))
	if len(errs) > 0 {
		t.Fatalf("unexpected correctness errors: %v", errs)
	}

	orig := packageExists
	t.Cleanup(func() { packageExists = orig })
	var asked []string
	packageExists = func(feed index.Feed, pkg, version string) error {
		asked = append(asked, pkg+"@"+version)
		if strings.HasSuffix(pkg, "_windows_arm64") {
			return errors.New("az artifacts universal download: exit status 1: package not found")
		}
		return nil
	}

	results := checkExistence(m, true)
	var want []string
	for _, rp := range requiredPlatforms {
		want = append(want, "dongle-deploy_2.3.1_"+strings.Replace(rp, "/", "_", 1)+"@2.3.1")
	}
	if strings.Join(asked, ",") != strings.Join(want, ",") {
		t.Fatalf("checked %v, want %v", asked, want)
	}
	if len(results) != len(requiredPlatforms) {
		t.Fatalf("got %d results, want %d", len(results), len(requiredPlatforms))
	}
	for _, r := range results {
		if r.os+"/"+r.arch == "windows/arm64" {
			if r.err == nil || !strings.HasPrefix(r.err.Error(), `deploy: windows/arm64 executable not found in feed "dongle-plugins": `) {
				t.Errorf("windows/arm64 err = %v", r.err)
			}
		} else if r.err != nil {
			t.Errorf("%s/%s err = %v, want nil", r.os, r.arch, r.err)
		}
	}
}

func TestCheckExistenceWithoutAz(t *testing.T) {
	m, _ := checkCorrectness(writeManifest(t, allExcept()...))
	orig := packageExists
	t.Cleanup(func() { packageExists = orig })
	packageExists = func(index.Feed, string, string) error {
		t.Fatal("packageExists called without az")
		return nil
	}
	results := checkExistence(m, false)
	if len(results) != len(requiredPlatforms) {
		t.Fatalf("got %d results, want %d", len(results), len(requiredPlatforms))
	}
	for _, r := range results {
		if r.err == nil {
			t.Errorf("%s/%s: expected failure without az", r.os, r.arch)
		}
	}
}
