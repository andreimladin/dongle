package builtins

import (
	"errors"
	"strings"
	"testing"

	"github.com/andreimladin/dongle/internal/bootstrap"
	"github.com/andreimladin/dongle/internal/testutil"
)

func TestInitialize(t *testing.T) {
	d1 := bootstrap.Default{Name: "deploy", Version: "1.0.0"}
	d2 := bootstrap.Default{Name: "lint", Version: "0.3.0"}
	cases := []struct {
		name       string
		cached     bool
		payload    fakePayload
		wantSeed   bool
		wantInst   []string
		wantMark   bool
		wantStderr []string
		notStderr  []string
	}{
		{
			name:      "plain build does nothing",
			notStderr: []string{"Initialization complete."},
		},
		{
			name:      "already initialized does nothing",
			cached:    true,
			payload:   fakePayload{hasIndex: true},
			notStderr: []string{"Initialization complete.", "Initializing plugin index"},
		},
		{
			name:       "first run seeds the index and installs defaults",
			payload:    fakePayload{hasIndex: true, defaults: []bootstrap.Default{d1, d2}},
			wantSeed:   true,
			wantInst:   []string{"deploy", "lint"},
			wantMark:   true,
			wantStderr: []string{"Initialized plugin index", "Installed deploy 1.0.0", "Installed lint 0.3.0", "Initialization complete."},
		},
		{
			name:       "defaults only",
			cached:     true,
			payload:    fakePayload{hasIndex: true, defaults: []bootstrap.Default{d1}},
			wantInst:   []string{"deploy"},
			wantMark:   true,
			wantStderr: []string{"Installed deploy 1.0.0", "Initialization complete."},
			notStderr:  []string{"Initialized plugin index"},
		},
		{
			name:       "failures are warnings and the rest still runs",
			payload:    fakePayload{hasIndex: true, seedErr: errors.New("disk full"), defaults: []bootstrap.Default{d1, d2}, installErr: map[string]error{"deploy": errors.New("boom")}, markErr: errors.New("ro fs")},
			wantSeed:   true,
			wantInst:   []string{"lint"},
			wantMark:   true,
			wantStderr: []string{"warning: could not initialize the embedded plugin index: disk full", "warning: installing embedded default deploy: boom", "Installed lint 0.3.0", "warning: could not record embedded defaults bootstrap: ro fs", "Initialization complete."},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newEnv(t)
			if c.cached {
				cacheIndex(t, "1")
			}
			p := c.payload
			orig := payload
			payload = &p
			defer func() { payload = orig }()

			stdout, stderr := testutil.Capture(t, Initialize)
			if stdout != "" {
				t.Errorf("Initialize wrote to stdout: %q", stdout)
			}
			if p.seeded != c.wantSeed {
				t.Errorf("seeded = %v, want %v", p.seeded, c.wantSeed)
			}
			if strings.Join(p.installedAs, ",") != strings.Join(c.wantInst, ",") {
				t.Errorf("installed %v, want %v", p.installedAs, c.wantInst)
			}
			if p.marked != c.wantMark {
				t.Errorf("marked = %v, want %v", p.marked, c.wantMark)
			}
			mustContain(t, "stderr", stderr, c.wantStderr...)
			mustNotContain(t, "stderr", stderr, c.notStderr...)
		})
	}
}
