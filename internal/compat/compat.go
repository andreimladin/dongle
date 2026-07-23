// Package compat decides whether a plugin may run on this host: a host-version
// range plus an exact protocol match. It is the single source of truth for both
// install-time and dispatch-time checks, so the logic lives in exactly one place.
package compat

import (
	"fmt"
	"strconv"
	"strings"
)

// Requires is a plugin's compatibility declaration. It carries both json and
// yaml tags because it appears in the runtime manifest (plugin.json) and in the
// index manifest (<name>.yaml).
type Requires struct {
	Host     string `json:"host"     yaml:"host"`     // min host version, e.g. ">=0.1.0"
	Protocol string `json:"protocol" yaml:"protocol"` // host<->plugin contract, e.g. "v1"
}

// Check reports whether a plugin is compatible with this host. When it isn't, it
// returns a human-readable reason so callers can print a clear message; the
// decision logic itself is never duplicated across callers.
func Check(hostVersion, protocol string, req Requires) (ok bool, reason string, err error) {
	okHost, err := SatisfiesHost(hostVersion, req.Host)
	if err != nil {
		return false, "", err
	}
	if !okHost {
		return false, fmt.Sprintf("requires host %s but this host is %s", req.Host, hostVersion), nil
	}
	if req.Protocol != "" && req.Protocol != protocol {
		return false, fmt.Sprintf("speaks protocol %q but this host supports %q", req.Protocol, protocol), nil
	}
	return true, "", nil
}

// SatisfiesHost supports the constraint forms dongle uses: ">=x.y.z", exact
// "x.y.z", and "" (any). For full ranges (^, ~, <, ||) swap in
// github.com/Masterminds/semver — this is the only function that would change.
func SatisfiesHost(hostVersion, constraint string) (bool, error) {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true, nil
	}
	hv, err := parseSemver(hostVersion)
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(constraint, ">=") {
		cv, err := parseSemver(strings.TrimSpace(constraint[2:]))
		if err != nil {
			return false, err
		}
		return compareSemver(hv, cv) >= 0, nil
	}
	cv, err := parseSemver(constraint)
	if err != nil {
		return false, err
	}
	return compareSemver(hv, cv) == 0, nil
}

func parseSemver(s string) ([3]int, error) {
	var out [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return out, fmt.Errorf("invalid semver %q", s)
	}
	for i := 0; i < 3; i++ {
		p := parts[i]
		if i == 2 { // strip -prerelease / +build from patch
			if j := strings.IndexAny(p, "-+"); j >= 0 {
				p = p[:j]
			}
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("invalid semver %q", s)
		}
		out[i] = n
	}
	return out, nil
}

func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return sign(a[i] - b[i])
		}
	}
	return 0
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
