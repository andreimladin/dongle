package compat

import (
	"strings"
	"testing"
)

func TestSatisfiesHost(t *testing.T) {
	cases := []struct {
		host       string
		constraint string
		want       bool
	}{
		{"2.3.1", "", true},
		{"2.3.1", "   ", true},
		{"2.3.1", "2.3.1", true},
		{"2.3.1", "2.3.2", false},
		{"2.3.1", ">=2.0.0", true},
		{"1.9.9", ">=2.0.0", false},
		{"2.0.0", ">=2.0.0", true},
		{"2.0.0", ">= 2.0.0", true},
		{"v2.0.0", ">=v1.10.0", true}, // leading v on either side; numeric, not lexical
		{"1.10.0", ">=1.9.0", true},
		{"2.0.0-rc.1", "2.0.0", true}, // prerelease/build suffix ignored
		{"2.0.0+build.5", ">=2.0.0", true},
	}
	for _, c := range cases {
		got, err := SatisfiesHost(c.host, c.constraint)
		if err != nil {
			t.Errorf("SatisfiesHost(%q, %q) unexpected error: %v", c.host, c.constraint, err)
			continue
		}
		if got != c.want {
			t.Errorf("SatisfiesHost(%q, %q) = %v, want %v", c.host, c.constraint, got, c.want)
		}
	}
}

func TestSatisfiesHostErrors(t *testing.T) {
	cases := []struct{ host, constraint string }{
		{"dev", ">=1.0.0"},   // non-semver host with a real constraint
		{"1.0.0", ">=1.0"},   // too few parts
		{"1.0.0", ">=x.0.0"}, // non-numeric
		{"1.0.0", "^1.0.0"},  // unsupported operator
		{"1.0.0", "1.0.z"},
	}
	for _, c := range cases {
		if _, err := SatisfiesHost(c.host, c.constraint); err == nil {
			t.Errorf("SatisfiesHost(%q, %q): want error", c.host, c.constraint)
		}
	}
	// An empty constraint never parses the host, so even "dev" passes.
	if ok, err := SatisfiesHost("dev", ""); err != nil || !ok {
		t.Errorf(`SatisfiesHost("dev", "") = %v, %v; want true, nil`, ok, err)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.10.0", "1.9.0", 1}, // numeric, not lexical
		{"2.0.0", "10.0.0", -1},
		{"1.2.3-beta", "1.2.3", 0},
	}
	for _, c := range cases {
		got, err := CompareVersions(c.a, c.b)
		if err != nil {
			t.Errorf("CompareVersions(%q, %q) unexpected error: %v", c.a, c.b, err)
			continue
		}
		if got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	if _, err := CompareVersions("bad", "1.0.0"); err == nil {
		t.Error("want error for invalid a")
	}
	if _, err := CompareVersions("1.0.0", "1.0"); err == nil {
		t.Error("want error for invalid b")
	}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name       string
		host       string
		req        Requires
		wantOK     bool
		wantReason string // substring
		wantErr    bool
	}{
		{name: "no requirements", host: "1.0.0", wantOK: true},
		{name: "host and protocol ok", host: "1.2.0", req: Requires{Host: ">=1.0.0", Protocol: "v1"}, wantOK: true},
		{name: "host too old", host: "0.9.0", req: Requires{Host: ">=1.0.0"}, wantReason: "requires host >=1.0.0 but this host is 0.9.0"},
		{name: "protocol mismatch", host: "1.0.0", req: Requires{Protocol: "v2"}, wantReason: `speaks protocol "v2" but this host supports "v1"`},
		{name: "bad constraint", host: "1.0.0", req: Requires{Host: ">=nope"}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, reason, err := Check(c.host, "v1", c.req)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
			if !strings.Contains(reason, c.wantReason) || (c.wantReason == "" && reason != "") {
				t.Fatalf("reason = %q, want %q", reason, c.wantReason)
			}
		})
	}
}
