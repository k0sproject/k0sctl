// Package download transfers URL sources to a host without repeating work that
// a previous run already did.
//
// A download is remembered by writing down what the server said about the URL at
// the time it was fetched. A later run asks the server the same question with an
// HTTP HEAD and skips the transfer when the answers still agree, which is what
// keeps a re-apply from pulling gigabytes that are already on the host.
//
// The records live in the login user's cache directory on the host rather than
// beside the downloaded file: the destination is typically a root-owned
// directory, and one of them, the k0s images directory, is scanned by k0s
// itself, where an unexpected file has no business being.
package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/v2/remotefs"

	log "github.com/sirupsen/logrus"
)

// ErrChecksumMismatch is returned when a downloaded file does not match the
// sha256 sum it was expected to have.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// errNoCacheDir is returned when the host has no cache directory to keep the
// download records in, which turns the bookkeeping off rather than failing
// anything.
var errNoCacheDir = errors.New("no usable user cache directory on the host")

// resumeAttempts is how many times a transfer is continued within a single run
// before the run gives up on it. Only reached when a sha256 sum is known, since
// that is what confirms the resumed bytes afterwards.
const resumeAttempts = 3

// Record describes a file that was downloaded to the host.
//
// The validators are what the server reported for the URL when the file was
// fetched. They are compared against a fresh HEAD response to tell whether the
// content behind the URL is still the content that was downloaded.
//
// The URL itself is not kept, only a digest of it. A download URL routinely
// carries its own authorization -- a presigned object store link is the ordinary
// way to serve a private artifact -- and a record stays on the host long after
// the apply that wrote it, backups included. The digest does everything the
// record needs the URL for, which is telling one download apart from another.
type Record struct {
	URLDigest   string `json:"urlDigest"`
	Destination string `json:"destination"`

	// Size and ModTime are of the destination as the download left it. They are
	// what separates a destination that has been left alone from one that has
	// been written to since, which is something no server header and no recorded
	// checksum can say.
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`

	ContentLength int64     `json:"contentLength"`
	ETag          string    `json:"etag,omitempty"`
	LastModified  time.Time `json:"lastModified,omitempty"`
	Sha256        string    `json:"sha256,omitempty"`

	// DownloadedAt is not compared against anything and is here for whoever
	// ends up reading these files.
	DownloadedAt time.Time `json:"downloadedAt"`
}

// Tracker downloads URLs to a single host and remembers what it downloaded.
//
// Create one per host and reuse it for every file, so that the host's cache
// directory is only looked up once.
type Tracker struct {
	// files is the filesystem the downloads are written to, usually sudo-enabled
	// since the destinations are commonly root-owned.
	files remotefs.FS

	// user is the same host without elevated privileges, and is used for the
	// records alone: they land in the login user's cache directory, which is
	// what keeps the bookkeeping out of sudo's way.
	//
	// Nothing that talks to the server goes through it. A HEAD has to describe
	// what the transfer would fetch, and the transfer runs as files: the two
	// identities can differ in proxy variables, credentials and .netrc, and a
	// validator collected under one of them says nothing about the other.
	user remotefs.FS

	// label prefixes the log messages, normally the host it is tracking.
	label string

	dirOnce sync.Once
	dir     string
}

// NewTracker returns a Tracker for a host. The files filesystem is where the
// downloads land and where the requests to the server are made from, so it needs
// whatever privileges the destinations require; user is the same host as the
// login user, and is where the records are kept.
func NewTracker(files, user remotefs.FS, label string) *Tracker {
	return &Tracker{files: files, user: user, label: label}
}

// Verdict is what [Tracker.Needed] concluded about a destination.
type Verdict struct {
	// Download is whether the transfer has to happen.
	Download bool

	// Reason is why, for a log line rather than for anyone to act on.
	Reason string

	// Adopt is set when the destination already holds the file the
	// configuration asks for but nothing on the host records that, so the next
	// run would have to read the whole file again to find out. Passing it to
	// [Tracker.Remember] settles that, once the caller is somewhere it may write
	// to the host.
	Adopt bool
}

// Needed reports whether url has to be transferred to dst, along with the reason
// for the answer, which is meant for a log line rather than for a user to act
// on.
//
// wantSum is the sha256 sum the configuration expects of the destination, or
// empty when it names none. It is the strongest answer available and takes over
// from the header comparison entirely: a destination that was verified against
// the sum the configuration asks for is the file that was asked for, whatever
// the server serves now, so the server is not even asked.
//
// Anything that cannot be determined counts as needing the download: it is only
// ever wasted work, where a wrong skip leaves the host with the wrong file.
//
// Nothing is written here, so it is safe to ask during a dry run or from a phase
// that is only gathering facts. Only [Tracker.Fetch] leaves a record behind,
// which is why a destination that k0sctl did not download itself is checksummed
// on every run rather than once.
func (t *Tracker) Needed(ctx context.Context, dlURL, dst, wantSum string) Verdict {
	stat, err := t.files.Stat(dst)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Debugf("%s: stat %s failed: %s", t.label, dst, err)
		}
		return Verdict{Download: true, Reason: "not downloaded yet"}
	}

	rec := t.loadRecord(dlURL, dst)
	ok, why := usable(rec, stat, wantSum)
	if ok && wantSum != "" {
		// The record says this destination was verified against exactly the sum
		// the configuration asks for, and it has not been written to since.
		// Nothing the server could say makes the file more correct than the
		// content the configuration named, so it is not asked.
		return Verdict{Reason: "sha256 matches the recorded download"}
	}
	if ok {
		info, err := remotefs.HTTPHead(ctx, t.files, dlURL)
		switch {
		case err != nil:
			// The error names the url, and rig's does too, so it is kept to the
			// debug log: a reason travels into the ordinary output of an apply.
			log.Debugf("%s: head request for %s failed: %s", t.label, dst, err)
			return Verdict{Download: true, Reason: "could not check whether the url changed"}
		case info.StatusCode < 200 || info.StatusCode > 299:
			// Includes the 405 of a server that refuses HEAD. Whether the URL is
			// gone or just unwilling to answer, the download is what finds out.
			return Verdict{Download: true, Reason: fmt.Sprintf("the url answered %d", info.StatusCode)}
		}
		same, why := unchanged(rec, info)
		return Verdict{Download: !same, Reason: why}
	}
	if wantSum == "" {
		return Verdict{Download: true, Reason: why}
	}

	// Without a usable record the sum is the only thing that can still spare the
	// transfer, and it is worth the read: reading the file on the host beats
	// pulling it over the network again.
	sum, err := t.files.Sha256(dst)
	if err != nil {
		return Verdict{Download: true, Reason: fmt.Sprintf("can not checksum %s: %s", dst, err)}
	}
	if !strings.EqualFold(sum, wantSum) {
		return Verdict{Download: true, Reason: "checksum does not match the configured sha256"}
	}
	// Reading the whole file is what answered this, and nothing on the host
	// remembers the answer, so the caller is told to write it down.
	return Verdict{Reason: "checksum matches the configured sha256", Adopt: true}
}

// Remember records a destination that already holds the file the configuration
// asks for, so that a later run can tell without reading the whole file again.
//
// It belongs with the Adopt of a [Verdict]. Needed itself writes nothing, since
// it is asked during dry runs and by phases that only gather facts, which leaves
// the caller to say when the host may be written to.
//
// Failing to record is not an error, only a later run doing the reading over.
func (t *Tracker) Remember(ctx context.Context, dlURL, dst, sum string) {
	if sum == "" {
		// Without a sum there is nothing a record could be believed on: the
		// validators belong to a download this did not make.
		return
	}
	stat, err := t.files.Stat(dst)
	if err != nil {
		log.Debugf("%s: not recording %s: %s", t.label, dst, err)
		return
	}
	// Asked so that the record can answer a later run that has no sum to go on,
	// for instance after the sum is taken out of the configuration again.
	info, err := remotefs.HTTPHead(ctx, t.files, dlURL)
	if err != nil {
		log.Debugf("%s: no url information for %s: %s", t.label, dst, err)
	}
	t.record(dlURL, dst, stat, sum, info)
}

// Fetch downloads url to dst on the host and records what was downloaded.
//
// When wantSum is set the file is verified against it, and an attempt that dies
// part way through is continued rather than restarted, which is what carries a
// large download over a link that drops. Resuming is deliberately limited to
// that case: neither curl nor wget can tell whether the bytes already on disk
// are still a valid prefix of what the URL serves, and the sum over the finished
// file is what settles it.
//
// Resuming lives and dies with the call: the attempts continue each other, and
// once they are spent what is left of the transfer is discarded rather than left
// on the host for a later run to find.
//
// A transfer reaches the destination before it can be checksummed, so a download
// that turns out not to match takes whatever was at the destination with it and
// is then removed: the destination is never left holding content the
// configuration did not ask for.
func (t *Tracker) Fetch(ctx context.Context, dlURL, dst, wantSum string) error {
	// Asking before the transfer rather than after means the record describes
	// what the transfer was started against. Should the content change in
	// between, the record is of the older content and the next run downloads
	// again, where recording afterwards could describe content that was never
	// downloaded and skip a transfer that is due.
	info, headErr := remotefs.HTTPHead(ctx, t.files, dlURL)
	if headErr != nil {
		log.Debugf("%s: no url information for %s: %s", t.label, dst, headErr)
	}

	if err := t.transfer(ctx, dlURL, dst, wantSum != ""); err != nil {
		return err
	}

	if wantSum != "" {
		if err := t.verify(dlURL, dst, wantSum); err != nil {
			return err
		}
	}

	// The size and modification time of what was just written are what a later
	// run holds the destination against, so they are read here rather than
	// guessed. Without them there is nothing to compare and the record is not
	// worth writing.
	stat, err := t.files.Stat(dst)
	if err != nil {
		log.Debugf("%s: stat %s after download failed: %s", t.label, dst, err)
	}
	t.record(dlURL, dst, stat, wantSum, info)
	return nil
}

// transfer runs the download itself, retrying a resumable one so that a
// transfer that dies part way through continues within the same run.
//
// What it says about a failure names the destination. The url is left to the
// error underneath, which rig composes and which is all that ever repeats it: a
// url can carry its own authorization, and an error travels further than a debug
// line does.
func (t *Tracker) transfer(ctx context.Context, dlURL, dst string, resume bool) error {
	if !isHTTP(dlURL) {
		// rig's Download only speaks http and https, but curl and wget accept
		// more than that and always have here, so anything else keeps going
		// through the plain download. It gets no resume, and no HEAD to compare
		// against later either, so without a configured sha256 such a source is
		// downloaded on every run, exactly as it was before.
		log.Debugf("%s: the source of %s is not an http url, downloading without resume support", t.label, dst)
		if err := t.files.DownloadURL(dlURL, dst); err != nil {
			return fmt.Errorf("download to %s: %w", dst, err)
		}
		return nil
	}

	if !resume {
		if err := remotefs.Download(ctx, t.files, dlURL, dst); err != nil {
			return fmt.Errorf("download to %s: %w", dst, err)
		}
		return nil
	}

	err := retry.Times(ctx, resumeAttempts, func(ctx context.Context) error {
		return remotefs.Download(ctx, t.files, dlURL, dst, remotefs.WithResume())
	})
	if err != nil {
		// Every attempt is spent and nothing is going to continue what is left of
		// the transfer, so the run clears up after itself rather than parking the
		// bytes beside the destination indefinitely. In a directory k0s reads,
		// like the images directory, that leftover is a truncated artifact that
		// something else would eventually try to make sense of.
		//
		// A cancelled run is no exception. Every Fetch asks to resume, so a
		// partial left by one would be continued by the next apply -- the
		// across-applies resuming that keeping this to a single call is meant to
		// rule out, and a way to end up assembling bytes from two different
		// versions of the url. Removing it is best effort: a host that has gone
		// away cannot be told to remove anything, and that is what the warning
		// is for.
		if dErr := remotefs.DiscardPartial(t.files, dlURL, dst); dErr != nil {
			log.Warnf("%s: failed to discard the partial download of %s: %s", t.label, dst, dErr)
		}
		return fmt.Errorf("download to %s: %w", dst, err)
	}
	return nil
}

// verify compares the downloaded file against the expected sum, and clears out
// the bytes when they do not match.
func (t *Tracker) verify(dlURL, dst, wantSum string) error {
	sum, err := t.files.Sha256(dst)
	if err != nil {
		// Not knowing what the bytes are is the same position as knowing they
		// are wrong: either way nothing has confirmed them, and the destination
		// does not get to keep content that never passed its check.
		t.discard(dlURL, dst)
		return fmt.Errorf("checksum %s: %w", dst, err)
	}
	if strings.EqualFold(sum, wantSum) {
		return nil
	}
	t.discard(dlURL, dst)
	return fmt.Errorf("%w: %s: expected sha256 %s, got %s", ErrChecksumMismatch, dst, wantSum, sum)
}

// discard throws away a download that could not be confirmed, along with
// everything that would let a later run take it for one that was.
//
// Leaving the bytes at the destination would hand whatever reads them a file
// that failed its check. A completed transfer has already had its partial
// renamed onto the destination, so discarding one is usually a no-op, but a
// partial that did outlive the transfer would be resumed onto the same bad
// bytes forever.
func (t *Tracker) discard(dlURL, dst string) {
	if err := t.files.Remove(dst); err != nil {
		log.Warnf("%s: failed to remove the unverified download %s: %s", t.label, dst, err)
	}
	if err := remotefs.DiscardPartial(t.files, dlURL, dst); err != nil {
		log.Debugf("%s: failed to discard the partial download of %s: %s", t.label, dst, err)
	}
	t.forget(dlURL, dst)
}

// usable reports whether a record still describes the file at the destination,
// and when it does not, why.
//
// Both the size and the modification time have to match. Everything else a
// record carries is about the URL, so without this the destination could be
// replaced by same-length content and every later run would go on skipping it.
func usable(rec *Record, stat fs.FileInfo, wantSum string) (bool, string) {
	switch {
	case rec == nil:
		return false, "no record of an earlier download"
	case rec.Size != stat.Size():
		// Someone or something has written to the file since, so what the record
		// says about it no longer describes what is there.
		return false, fmt.Sprintf("file size %d does not match the downloaded %d", stat.Size(), rec.Size)
	case rec.ModTime.IsZero():
		// The destination could not be measured when the download was recorded,
		// which leaves nothing to hold it against now.
		return false, "the earlier download was recorded without a modification time"
	case !rec.ModTime.Equal(stat.ModTime()):
		return false, fmt.Sprintf("file was modified at %s, the download left it at %s", stat.ModTime().UTC(), rec.ModTime.UTC())
	case wantSum != "" && !strings.EqualFold(rec.Sha256, wantSum):
		// The configured sum is not the sum the file was accepted with, either
		// because it changed in the configuration or because it was added after
		// the download. Either way the file has not been checked against it.
		return false, "configured sha256 differs from the downloaded file's sum"
	}
	return true, ""
}

// strongETag reports whether an entity tag is one that proves byte identity. A
// weak validator, the ones prefixed with W/, promises only that two
// representations mean the same thing, which is not enough to leave a file that
// has to match the URL byte for byte alone.
func strongETag(tag string) bool {
	return tag != "" && !strings.HasPrefix(tag, "W/")
}

// unchanged reports whether a HEAD response still describes the content that was
// downloaded, and what that verdict rests on.
//
// Only a strong ETag or a Last-Modified can conclude that nothing changed. A
// Content-Length cannot: a server that swaps an artifact for different bytes of
// the same length would report the same one forever, so a length is only ever
// used to corroborate a timestamp. Where neither validator is on offer the
// answer is to download, which is what every URL source did before any of this
// was recorded, and a configured sha256 is the way out of it.
func unchanged(rec *Record, info *remotefs.URLInfo) (bool, string) {
	if rec.ETag != "" && info.ETag != "" {
		if rec.ETag != info.ETag {
			// Two different tags are two different representations, whether the
			// tags are weak or strong.
			return false, "etag changed"
		}
		if strongETag(rec.ETag) && strongETag(info.ETag) {
			return true, "etag unchanged"
		}
		// Equal weak tags leave it open, so the timestamp below gets a say.
	}
	if !rec.LastModified.IsZero() && !info.LastModified.IsZero() {
		if !rec.LastModified.Equal(info.LastModified) {
			return false, "last-modified changed"
		}
		if info.ContentLength >= 0 && info.ContentLength != rec.Size {
			return false, "content-length changed"
		}
		return true, "last-modified unchanged"
	}
	return false, "server reports no strong etag or last-modified"
}

// record writes down a download. A record is a shortcut for the next run, so
// failing to write one is logged and otherwise ignored: the only consequence is
// that the next run has to download the file again.
func (t *Tracker) record(dlURL, dst string, stat fs.FileInfo, sum string, info *remotefs.URLInfo) {
	rec := Record{
		URLDigest:     urlDigest(dlURL),
		Destination:   dst,
		Size:          -1,
		ContentLength: -1,
		Sha256:        sum,
		DownloadedAt:  time.Now().UTC(),
	}
	if stat != nil {
		rec.Size = stat.Size()
		rec.ModTime = stat.ModTime()
	}
	if info != nil {
		rec.ContentLength = info.ContentLength
		rec.ETag = info.ETag
		rec.LastModified = info.LastModified
	}
	switch {
	case rec.ModTime.IsZero():
		// Without the state of the destination there is nothing to tell a file
		// that was left alone from one that was replaced.
		log.Debugf("%s: not recording the download of %s: the destination could not be measured", t.label, dst)
		return
	case !strongETag(rec.ETag) && rec.LastModified.IsZero() && sum == "":
		// Nothing here could ever confirm the file later: a weak etag and a
		// length can say that something changed but never that nothing did, and
		// a record that can only answer "download it again" is not worth
		// keeping around.
		log.Debugf("%s: not recording the download of %s: nothing to compare it by later", t.label, dst)
		return
	}

	data, err := json.Marshal(rec)
	if err != nil {
		log.Debugf("%s: failed to marshal the download record for %s: %s", t.label, dst, err)
		return
	}
	dir, err := t.recordsDir()
	if err != nil {
		log.Debugf("%s: not recording the download of %s: %s", t.label, dst, err)
		return
	}
	if err := t.user.MkdirAll(dir, 0o700); err != nil {
		log.Debugf("%s: failed to create %s: %s", t.label, dir, err)
		return
	}
	path := t.user.Join(dir, recordName(t.user, dlURL, dst))
	if err := t.user.WriteFile(path, data, 0o600); err != nil {
		log.Debugf("%s: failed to write the download record %s: %s", t.label, path, err)
		return
	}
	log.Debugf("%s: recorded the download of %s in %s", t.label, dst, path)
}

// forget drops the record for a download, so that the next run makes its own
// mind up about the destination instead of trusting a record that has been
// proven wrong.
func (t *Tracker) forget(dlURL, dst string) {
	dir, err := t.recordsDir()
	if err != nil {
		return
	}
	path := t.user.Join(dir, recordName(t.user, dlURL, dst))
	if err := t.user.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Debugf("%s: failed to remove the download record %s: %s", t.label, path, err)
	}
}

// loadRecord reads the record for a download, or returns nil when there is none
// to be had. An unreadable or unparseable record is treated as no record at all,
// since the answer to both is the same download.
func (t *Tracker) loadRecord(dlURL, dst string) *Record {
	dir, err := t.recordsDir()
	if err != nil {
		log.Debugf("%s: no download records available: %s", t.label, err)
		return nil
	}
	path := t.user.Join(dir, recordName(t.user, dlURL, dst))
	data, err := t.user.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Debugf("%s: failed to read the download record %s: %s", t.label, path, err)
		}
		return nil
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		log.Debugf("%s: failed to parse the download record %s: %s", t.label, path, err)
		return nil
	}
	// The name is a truncated digest, so a record found under it should be for
	// this download; a mismatch would mean answering for a different one, which
	// is not worth doing on the off chance.
	if rec.URLDigest != urlDigest(dlURL) || rec.Destination != dst {
		log.Debugf("%s: the download record %s is for a different download (%s), ignoring it", t.label, path, rec.Destination)
		return nil
	}
	return &rec
}

// recordsDir is where this host keeps its download records. The user's cache
// directory is looked up on the host, which costs a command, so the answer is
// kept for the lifetime of the Tracker.
func (t *Tracker) recordsDir() (string, error) {
	t.dirOnce.Do(func() {
		cache := t.user.UserCacheDir()
		if !isAbs(cache) {
			// An empty or relative cache directory would put the records
			// somewhere that depends on where a command happened to run, which
			// is no place to look for them later.
			log.Debugf("%s: no usable user cache directory (%q), download records are disabled", t.label, cache)
			return
		}
		t.dir = t.user.Join(cache, "k0sctl", "downloads")
	})
	if t.dir == "" {
		return "", errNoCacheDir
	}
	return t.dir, nil
}

// urlDigest identifies a url without keeping it, so that a record can be matched
// to a download without writing whatever the url carries to the host.
func urlDigest(dlURL string) string {
	sum := sha256.Sum256([]byte(dlURL))
	return hex.EncodeToString(sum[:])
}

// recordName is the file name a download's record is kept under.
//
// Both the url and the destination go into the digest so that a record can only
// ever be found by a lookup for the same pair, and the destination's base name
// is kept in front of it to make the directory readable to whoever ends up
// looking at it.
func recordName(fsys remotefs.FS, dlURL, dst string) string {
	sum := sha256.Sum256([]byte(dlURL + "\x00" + dst))
	return fsys.Base(dst) + "-" + hex.EncodeToString(sum[:8]) + ".json"
}

// isHTTP reports whether a url is one that rig's Download can handle.
func isHTTP(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

// isAbs reports whether a remote path is absolute in either the posix or the
// windows sense. The host's own path rules are not exposed as a predicate, and
// the two shapes are distinct enough to tell apart here.
func isAbs(path string) bool {
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) {
		return true
	}
	// A windows path like C:/Users/foo or C:\Users\foo.
	if len(path) > 2 && path[1] == ':' && (path[2] == '/' || path[2] == '\\') {
		return true
	}
	return false
}
