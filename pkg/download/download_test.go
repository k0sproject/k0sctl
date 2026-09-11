package download

import (
	"encoding/json"
	"io/fs"
	"testing"
	"time"

	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/stretchr/testify/require"
)

// stubStat is the little of fs.FileInfo that a download record is compared
// against.
type stubStat struct {
	size    int64
	modTime time.Time
}

func (s stubStat) Name() string       { return "stub" }
func (s stubStat) Size() int64        { return s.size }
func (s stubStat) Mode() fs.FileMode  { return 0o644 }
func (s stubStat) ModTime() time.Time { return s.modTime }
func (s stubStat) IsDir() bool        { return false }
func (s stubStat) Sys() any           { return nil }

func TestUsable(t *testing.T) {
	const sum = "1111111111111111111111111111111111111111111111111111111111111111"
	written := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	onDisk := stubStat{size: 100, modTime: written}
	rec := func(mod func(*Record)) *Record {
		r := &Record{
			URLDigest:     urlDigest("https://example.com/f"),
			Destination:   "/tmp/f",
			Size:          100,
			ModTime:       written,
			ContentLength: 100,
			Sha256:        sum,
		}
		if mod != nil {
			mod(r)
		}
		return r
	}

	t.Run("no record", func(t *testing.T) {
		ok, why := usable(nil, onDisk, "")
		require.False(t, ok)
		require.Equal(t, "no record of an earlier download", why)
	})

	t.Run("matching record", func(t *testing.T) {
		ok, _ := usable(rec(nil), onDisk, sum)
		require.True(t, ok)
	})

	t.Run("file was written to after the download", func(t *testing.T) {
		ok, why := usable(rec(nil), stubStat{size: 120, modTime: written}, "")
		require.False(t, ok)
		require.Contains(t, why, "does not match the downloaded")
	})

	t.Run("file was replaced with content of the same length", func(t *testing.T) {
		ok, why := usable(rec(nil), stubStat{size: 100, modTime: written.Add(time.Minute)}, "")
		require.False(t, ok)
		require.Contains(t, why, "was modified at")
	})

	t.Run("modification time in another zone is the same instant", func(t *testing.T) {
		berlin := time.FixedZone("CEST", 2*60*60)
		ok, _ := usable(rec(nil), stubStat{size: 100, modTime: written.In(berlin)}, sum)
		require.True(t, ok)
	})

	t.Run("record without a modification time", func(t *testing.T) {
		ok, why := usable(rec(func(r *Record) { r.ModTime = time.Time{} }), onDisk, "")
		require.False(t, ok)
		require.Contains(t, why, "without a modification time")
	})

	t.Run("configured sum changed", func(t *testing.T) {
		ok, why := usable(rec(nil), onDisk, "2222222222222222222222222222222222222222222222222222222222222222")
		require.False(t, ok)
		require.Contains(t, why, "configured sha256 differs")
	})

	t.Run("sum configured after the download", func(t *testing.T) {
		ok, _ := usable(rec(func(r *Record) { r.Sha256 = "" }), onDisk, sum)
		require.False(t, ok)
	})

	t.Run("configured sum in a different case", func(t *testing.T) {
		ok, _ := usable(rec(func(r *Record) { r.Sha256 = "AAAA" }), onDisk, "aaaa")
		require.True(t, ok)
	})

	t.Run("no sum configured or recorded", func(t *testing.T) {
		ok, _ := usable(rec(func(r *Record) { r.Sha256 = "" }), onDisk, "")
		require.True(t, ok)
	})
}

func TestUnchanged(t *testing.T) {
	modified := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name string
		rec  Record
		info remotefs.URLInfo
		want bool
		why  string
	}{
		{
			name: "same strong etag",
			rec:  Record{ETag: `"abc"`, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ETag: `"abc"`, ContentLength: 100},
			want: true,
			why:  "etag unchanged",
		},
		{
			name: "different etag",
			rec:  Record{ETag: `"abc"`, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ETag: `"def"`, ContentLength: 100},
			want: false,
			why:  "etag changed",
		},
		{
			// A weak validator only promises that the two representations mean
			// the same thing, so it cannot stand in for byte identity.
			name: "same weak etag, nothing else",
			rec:  Record{ETag: `W/"abc"`, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ETag: `W/"abc"`, ContentLength: 100},
			want: false,
			why:  "server reports no strong etag or last-modified",
		},
		{
			name: "same weak etag and last-modified",
			rec:  Record{ETag: `W/"abc"`, LastModified: modified, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ETag: `W/"abc"`, LastModified: modified, ContentLength: 100},
			want: true,
			why:  "last-modified unchanged",
		},
		{
			name: "different weak etag, same last-modified",
			rec:  Record{ETag: `W/"abc"`, LastModified: modified, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ETag: `W/"def"`, LastModified: modified, ContentLength: 100},
			want: false,
			why:  "etag changed",
		},
		{
			name: "etag turned weak",
			rec:  Record{ETag: `"abc"`, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ETag: `W/"abc"`, ContentLength: 100},
			want: false,
			why:  "etag changed",
		},
		{
			name: "same etag but a different length",
			rec:  Record{ETag: `"abc"`, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ETag: `"abc"`, ContentLength: 200},
			want: true,
			why:  "etag unchanged",
		},
		{
			name: "no etag, same last-modified and length",
			rec:  Record{LastModified: modified, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{LastModified: modified, ContentLength: 100},
			want: true,
			why:  "last-modified unchanged",
		},
		{
			name: "no etag, newer last-modified",
			rec:  Record{LastModified: modified, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{LastModified: modified.Add(time.Hour), ContentLength: 100},
			want: false,
			why:  "last-modified changed",
		},
		{
			name: "no etag, same last-modified but a different length",
			rec:  Record{LastModified: modified, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{LastModified: modified, ContentLength: 200},
			want: false,
			why:  "content-length changed",
		},
		{
			name: "no etag, same last-modified and an unknown length",
			rec:  Record{LastModified: modified, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{LastModified: modified, ContentLength: -1},
			want: true,
			why:  "last-modified unchanged",
		},
		{
			// Same length, different bytes is exactly what a length cannot rule
			// out, so a length on its own never confirms anything.
			name: "length only, unchanged",
			rec:  Record{Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ContentLength: 100},
			want: false,
			why:  "server reports no strong etag or last-modified",
		},
		{
			name: "length only, changed",
			rec:  Record{Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ContentLength: 200},
			want: false,
			why:  "server reports no strong etag or last-modified",
		},
		{
			name: "server says nothing at all",
			rec:  Record{Size: 100, ContentLength: -1},
			info: remotefs.URLInfo{ContentLength: -1},
			want: false,
			why:  "server reports no strong etag or last-modified",
		},
		{
			name: "only the recorded side has an etag",
			rec:  Record{ETag: `"abc"`, Size: 100, ContentLength: 100},
			info: remotefs.URLInfo{ContentLength: 100},
			want: false,
			why:  "server reports no strong etag or last-modified",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, why := unchanged(&tc.rec, &tc.info)
			require.Equal(t, tc.want, got, why)
			require.Equal(t, tc.why, why)
		})
	}
}

func TestRecordRoundTrip(t *testing.T) {
	rec := Record{
		URLDigest:     urlDigest("https://example.com/bundle.tar"),
		Destination:   "/var/lib/k0s/images/bundle.tar",
		Size:          1234,
		ModTime:       time.Date(2026, 9, 1, 8, 29, 0, 0, time.UTC),
		ContentLength: 1234,
		ETag:          `"abc"`,
		LastModified:  time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		Sha256:        "1111111111111111111111111111111111111111111111111111111111111111",
		DownloadedAt:  time.Date(2026, 9, 1, 8, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(rec)
	require.NoError(t, err)

	var got Record
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, rec, got)
}

func TestIsHTTP(t *testing.T) {
	for url, want := range map[string]bool{
		"https://example.com/f":  true,
		"http://example.com/f":   true,
		"HTTPS://example.com/f":  true,
		"ftp://example.com/f":    false,
		"file:///tmp/f":          false,
		"example.com/f":          false,
		"://":                    false,
		"https://user@host/f":    true,
		"http://host:8080/a%20b": true,
	} {
		require.Equal(t, want, isHTTP(url), url)
	}
}

func TestIsAbs(t *testing.T) {
	for path, want := range map[string]bool{
		"/home/user/.cache":           true,
		`C:\Users\user\AppData\Local`: true,
		"C:/Users/user/AppData/Local": true,
		`\\server\share`:              true,
		".cache":                      false,
		"":                            false,
		"home/user/.cache":            false,
		"C:":                          false,
		"~/.cache":                    false,
	} {
		require.Equal(t, want, isAbs(path), path)
	}
}

func TestStrongETag(t *testing.T) {
	for tag, want := range map[string]bool{
		`"abc"`:      true,
		`W/"abc"`:    false,
		``:           false,
		`abc`:        true,
		`w/"abc"`:    true, // the prefix is case sensitive per RFC 9110
		`"W/nested"`: true,
	} {
		require.Equal(t, want, strongETag(tag), tag)
	}
}

func TestRecordKeepsNoURL(t *testing.T) {
	// A download url can carry its own authorization, and a record outlives the
	// apply that wrote it.
	const secret = "https://example.com/bundle.tar?X-Amz-Signature=deadbeefcafe"

	rec := Record{URLDigest: urlDigest(secret), Destination: "/tmp/bundle.tar"}
	data, err := json.Marshal(rec)
	require.NoError(t, err)

	require.NotContains(t, string(data), "deadbeefcafe")
	require.NotContains(t, string(data), "example.com")
	require.Equal(t, urlDigest(secret), rec.URLDigest)
	require.NotEqual(t, urlDigest(secret), urlDigest(secret+"x"))
}
