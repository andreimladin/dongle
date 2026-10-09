package index

import (
	"os"
	"time"
)

// CheckTTL is how long a cached index is trusted after it was last
// downloaded from, or confirmed current against, the feed. Within it
// Prepare(CheckIfStale, ...) skips the feed entirely — no feed call, so
// the command layer has nothing to prompt about. CheckAlways (`dongle
// update`, --sync) ignores it.
const CheckTTL = time.Hour

// Policy is how hard Prepare tries to bring the cached index up to date.
type Policy int

const (
	// CheckIfStale queries the feed for a newer index only when the cache
	// is older than CheckTTL (the automatic check search/install/upgrade
	// run first).
	CheckIfStale Policy = iota
	// CheckAlways queries the feed regardless of the TTL (`dongle update`,
	// --sync).
	CheckAlways
	// CheckNever only makes sure some index is cached (--no-sync).
	CheckNever
)

// Step names a slow step Prepare is about to start, so a caller can show
// progress for it (a spinner) without this package doing any terminal I/O.
type Step int

const (
	// StepDownload: nothing is cached and nothing is embedded, so the
	// latest index is being downloaded from the feed.
	StepDownload Step = iota
	// StepCheck: the feed is being asked for its latest index version
	// (metadata only).
	StepCheck
)

// Status is what Prepare did and found. It never downloads a newer index
// over an existing cache: when UpdateAvailable reports one, the caller
// decides (prompting, --sync, `dongle update`) whether to Download it.
type Status struct {
	// Cached is the version of the index now cached ("" if unknown).
	Cached string
	// Downloaded: nothing was cached or embedded, so the latest index was
	// just downloaded as the first cache. No check follows: it is the
	// latest by definition.
	Downloaded bool
	// Checked: the feed was queried successfully and Latest is its latest
	// index version. False when the cache was fresh (CheckIfStale within
	// CheckTTL), with CheckNever, or when the query failed (CheckErr).
	Checked bool
	Latest  string
	// CheckErr is why the feed query failed. The cache stays usable, so
	// it is not Prepare's error; callers decide whether it is fatal.
	CheckErr error
}

// UpdateAvailable reports whether the feed has a newer index than the
// cached one.
func (s Status) UpdateAvailable() bool { return s.Checked && IsNewer(s.Latest, s.Cached) }

// Prepare makes sure a usable index is cached — seeding it from the
// embedded copy, or downloading it when nothing was embedded — and then,
// per policy, asks the feed for its latest index version. When the feed's
// version is not newer the cache is marked as confirmed current, which
// restarts CheckTTL. onStep (may be nil) is called before each slow step.
//
// The returned error means no index could be made available at all; a
// failed feed query is reported in Status.CheckErr instead.
func Prepare(policy Policy, onStep func(Step)) (Status, error) {
	step := func(s Step) {
		if onStep != nil {
			onStep(s)
		}
	}

	if !HasCache() && !HasEmbedded() {
		step(StepDownload)
		if err := EnsureCache(); err != nil {
			return Status{}, err
		}
		v, _ := CachedVersion()
		return Status{Cached: v, Downloaded: true}, nil
	}
	if err := EnsureCache(); err != nil {
		return Status{}, err
	}
	st := Status{}
	st.Cached, _ = CachedVersion()
	if policy == CheckNever || (policy == CheckIfStale && IsFresh(CheckTTL)) {
		return st, nil
	}

	step(StepCheck)
	latest, err := LatestVersion()
	if err != nil {
		st.CheckErr = err
		return st, nil
	}
	st.Checked, st.Latest = true, latest
	if !st.UpdateAvailable() {
		_ = MarkChecked()
	}
	return st, nil
}

// Download downloads index version v from the feed and installs it as
// the cache (Fetch, then Apply), restarting CheckTTL.
func Download(v string) error {
	latest, err := Fetch(v)
	if err != nil {
		return err
	}
	return latest.Apply()
}

// MarkChecked records (in index.meta) when the cached index was last
// confirmed current against the feed, without re-downloading it.
func MarkChecked() error { return touchMeta() }

// IsFresh reports whether the cached index was refreshed from (or
// confirmed current against) the feed less than ttl ago. A cache with no
// freshness stamp — never checked, or seeded from the embedded copy — is
// never fresh.
func IsFresh(ttl time.Duration) bool {
	age, err := cacheAge()
	return err == nil && age >= 0 && age < ttl
}

// cacheAge is how long ago index.meta was stamped by touchMeta.
func cacheAge() (time.Duration, error) {
	fi, err := os.Stat(metaPath())
	if err != nil {
		return 0, err
	}
	return time.Since(fi.ModTime()), nil
}

func touchMeta() error {
	now := time.Now()
	if err := os.WriteFile(metaPath(), []byte(now.Format(time.RFC3339)), 0o644); err != nil {
		return err
	}
	return os.Chtimes(metaPath(), now, now)
}
