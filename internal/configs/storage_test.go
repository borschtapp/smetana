package configs

import (
	"strings"
	"testing"

	"borscht.app/smetana/internal/storage"
)

func TestNewStorage_LocalMediaBaseAlwaysMatchesUploadsRoute(t *testing.T) {
	// main.go only ever mounts local uploads under the "/uploads" static route,
	// so whatever BASE_URL is configured, the derived media base must end in
	// "/uploads" too — otherwise generated image URLs 404.
	testCases := map[string]struct {
		baseURL string
		host    string
		port    int
		want    string
	}{
		"no BASE_URL falls back to host:port/uploads": {
			baseURL: "",
			host:    "",
			port:    3000,
			want:    "http://localhost:3000/uploads",
		},
		"no BASE_URL with explicit host": {
			baseURL: "",
			host:    "0.0.0.0",
			port:    8080,
			want:    "http://0.0.0.0:8080/uploads",
		},
		"BASE_URL without trailing slash": {
			baseURL: "http://localhost:3000",
			want:    "http://localhost:3000/uploads",
		},
		"BASE_URL with trailing slash": {
			baseURL: "http://localhost:3000/",
			want:    "http://localhost:3000/uploads",
		},
		"BASE_URL is a public https origin": {
			baseURL: "https://smetana.example.com",
			want:    "https://smetana.example.com/uploads",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("BASE_URL", tc.baseURL)
			t.Setenv("S3_BUCKET", "")
			t.Setenv("STORAGE_ROOT", t.TempDir())

			cfg := NewStorage(tc.host, tc.port)

			if cfg.BaseURL != tc.want {
				t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, tc.want)
			}
			if got := cfg.Storage.GetBaseURL(); got != tc.want {
				t.Errorf("Storage.GetBaseURL() = %q, want %q", got, tc.want)
			}

			assertURLRoundTrips(t, cfg.Storage, tc.want)
		})
	}
}

func TestNewStorage_S3IgnoresBaseURL(t *testing.T) {
	// The S3 base URL is derived from S3_ENDPOINT/S3_BUCKET and is served
	// directly by the S3-compatible provider, not by the local /uploads
	// route, so it must not be influenced by BASE_URL.
	t.Setenv("BASE_URL", "https://smetana.example.com")
	t.Setenv("S3_BUCKET", "smetana-media")
	t.Setenv("S3_HOST", "s3.example.com")

	cfg := NewStorage("", 3000)

	want := "https://s3.example.com/smetana-media"
	if cfg.BaseURL != want {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, want)
	}

	assertURLRoundTrips(t, cfg.Storage, want)
}

// assertURLRoundTrips exercises the exact path storage.Path.MarshalJSON uses in
// API responses (AbsoluteUrl) and the reverse used when parsing feed input
// (RelativeUrl), asserting: (1) the object key is reachable under the media
// base with a single "/" separator, and (2) converting back recovers the
// original object key, so URLs sent to clients are the same ones stored.
func assertURLRoundTrips(t *testing.T, fileStorage storage.FileStorage, wantBase string) {
	t.Helper()

	storage.SetDefault(fileStorage)
	t.Cleanup(func() { storage.SetDefault(nil) })

	const objectKey = "recipes/abc-123/img.jpg"

	absolute := storage.AbsoluteUrl(objectKey)
	wantAbsolute := wantBase + "/" + objectKey
	if absolute != wantAbsolute {
		t.Errorf("AbsoluteUrl(%q) = %q, want %q", objectKey, absolute, wantAbsolute)
	}
	if strings.Contains(strings.TrimPrefix(absolute, wantBase), "//") {
		t.Errorf("AbsoluteUrl(%q) = %q has a doubled path separator", objectKey, absolute)
	}

	if got := storage.RelativeUrl(absolute); got != objectKey {
		t.Errorf("RelativeUrl(%q) = %q, want %q", absolute, got, objectKey)
	}
}
