package index

import "testing"

func TestParseLatestVersion(t *testing.T) {
	body := []byte(`{"count":2,"value":[
		{"name":"dongle-index-extra","versions":[{"version":"9.9.9","isLatest":true}]},
		{"name":"Dongle-Index","versions":[{"version":"1.4.0","isLatest":true}]}]}`)
	got, err := parseLatestVersion(body, "dongle-index")
	if err != nil || got != "1.4.0" {
		t.Fatalf("got %q, %v; want 1.4.0", got, err)
	}

	noFlag := []byte(`{"value":[{"name":"dongle-index","versions":[{"version":"1.2.0"},{"version":"1.10.0"}]}]}`)
	if got, err := parseLatestVersion(noFlag, "dongle-index"); err != nil || got != "1.10.0" {
		t.Fatalf("got %q, %v; want 1.10.0", got, err)
	}

	if _, err := parseLatestVersion([]byte(`{"value":[]}`), "dongle-index"); err == nil {
		t.Fatal("want an error for a package with no versions")
	}
}
