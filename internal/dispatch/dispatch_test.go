package dispatch

import "testing"

func TestSatisfiesHost(t *testing.T) {
	cases := []struct {
		host       string
		constraint string
		want       bool
	}{
		{"2.3.1", "", true},
		{"2.3.1", "2.3.1", true},
		{"2.3.1", "2.3.2", false},
		{"2.3.1", ">=2.0.0", true},
		{"1.9.9", ">=2.0.0", false},
		{"2.0.0", "<=2.0.0", true},
		{"2.0.1", "<=2.0.0", false},
		{"2.0.1", ">2.0.0", true},
		{"2.0.0", ">2.0.0", false},
		{"1.9.9", "<2.0.0", true},
		{"2.0.0", "<2.0.0", false},
		{"2.5.0", "^2.1.0", true},
		{"3.0.0", "^2.1.0", false},
		{"2.0.9", "^2.1.0", false},
		{"2.1.4", "~2.1.0", true},
		{"2.2.0", "~2.1.0", false},
		{"2.1.0", "~2.1.0", true},
	}
	for _, c := range cases {
		got, err := satisfiesHost(c.host, c.constraint)
		if err != nil {
			t.Errorf("satisfiesHost(%q, %q) unexpected error: %v", c.host, c.constraint, err)
			continue
		}
		if got != c.want {
			t.Errorf("satisfiesHost(%q, %q) = %v, want %v", c.host, c.constraint, got, c.want)
		}
	}
}
