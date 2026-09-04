package main

// Build-time build inputs, injected via -ldflags at build time (see
// scripts/build-binaries.sh, and configs/ — the human-edited source of
// truth that script bakes these values from).
// A plain `go build ./cmd` leaves them at these defaults: hostVersion
// "dev", no index URL (DONGLE_INDEX_URL is then required to use `dongle
// index`/`dongle plugin` commands), indexBranch "main".
var (
	hostVersion = "dev"
	indexURL    = ""
	indexBranch = "main"
)

// protocol is the host<->plugin contract version. It is not a build
// input — it changes only when the connector spec itself changes — so it
// stays a const rather than joining the vars above.
const protocol = "v1"
