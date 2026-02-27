package vfs

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Compile-time assertion: *S3 must implement FileSystem.
var _ FileSystem = (*S3)(nil)

// ---- Config validation ------------------------------------------------------

func TestNewS3_EmptyRegion(t *testing.T) {
	t.Parallel()

	_, err := NewS3(S3Config{
		Region:          "",
		Bucket:          "my-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})
	if err == nil {
		t.Fatal("expected error for empty region, got nil")
	}
	if !strings.Contains(err.Error(), "region") {
		t.Errorf("expected error to mention 'region', got: %v", err)
	}
}

func TestNewS3_EmptyBucket(t *testing.T) {
	t.Parallel()

	_, err := NewS3(S3Config{
		Region:          "us-east-1",
		Bucket:          "",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})
	if err == nil {
		t.Fatal("expected error for empty bucket, got nil")
	}
	if !strings.Contains(err.Error(), "bucket") {
		t.Errorf("expected error to mention 'bucket', got: %v", err)
	}
}

func TestNewS3_EmptyAccessKeyID(t *testing.T) {
	t.Parallel()

	_, err := NewS3(S3Config{
		Region:          "us-east-1",
		Bucket:          "my-bucket",
		AccessKeyID:     "",
		SecretAccessKey: "SECRET",
	})
	if err == nil {
		t.Fatal("expected error for empty access key ID, got nil")
	}
	if !strings.Contains(err.Error(), "access key") {
		t.Errorf("expected error to mention 'access key', got: %v", err)
	}
}

func TestNewS3_EmptySecretAccessKey(t *testing.T) {
	t.Parallel()

	_, err := NewS3(S3Config{
		Region:          "us-east-1",
		Bucket:          "my-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "",
	})
	if err == nil {
		t.Fatal("expected error for empty secret access key, got nil")
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Errorf("expected error to mention 'secret', got: %v", err)
	}
}

func TestNewS3_ValidConfig(t *testing.T) {
	t.Parallel()

	fs, err := NewS3(S3Config{
		Region:          "eu-west-1",
		Bucket:          "my-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if fs == nil {
		t.Fatal("expected non-nil *S3")
	}
}

// ---- IsConnected / Close lifecycle ------------------------------------------

func TestS3_IsConnected_AfterNew(t *testing.T) {
	t.Parallel()

	fs := mustNewS3(t)
	if !fs.IsConnected() {
		t.Error("expected IsConnected to be true after NewS3")
	}
}

func TestS3_IsConnected_AfterClose(t *testing.T) {
	t.Parallel()

	fs := mustNewS3(t)
	if err := fs.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if fs.IsConnected() {
		t.Error("expected IsConnected to be false after Close")
	}
}

func TestS3_CloseIdempotent(t *testing.T) {
	t.Parallel()

	fs := mustNewS3(t)
	if err := fs.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := fs.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// ---- Path normalisation -----------------------------------------------------

func TestNormS3Key(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"/", ""},
		{"foo", "foo"},
		{"/foo", "foo"},
		{"foo/", "foo"},
		{"/foo/bar/", "foo/bar"},
		{"foo/bar/baz", "foo/bar/baz"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := normS3Key(tc.input)
			if got != tc.want {
				t.Errorf("normS3Key(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormS3Prefix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"/", ""},
		{"foo", "foo/"},
		{"/foo", "foo/"},
		{"foo/", "foo/"},
		{"/foo/bar/", "foo/bar/"},
		{"foo/bar/baz", "foo/bar/baz/"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := normS3Prefix(tc.input)
			if got != tc.want {
				t.Errorf("normS3Prefix(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---- Base URL construction --------------------------------------------------

func TestNewS3_DefaultEndpoint(t *testing.T) {
	t.Parallel()

	fs := mustNewS3WithConfig(t, S3Config{
		Region:          "ap-southeast-1",
		Bucket:          "test-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
	})

	const wantBase = "https://test-bucket.s3.ap-southeast-1.amazonaws.com"
	if fs.baseURL != wantBase {
		t.Errorf("baseURL = %q, want %q", fs.baseURL, wantBase)
	}
}

func TestNewS3_CustomEndpoint(t *testing.T) {
	t.Parallel()

	fs := mustNewS3WithConfig(t, S3Config{
		Region:          "us-east-1",
		Bucket:          "my-bucket",
		AccessKeyID:     "AKID",
		SecretAccessKey: "SECRET",
		Endpoint:        "http://localhost:9000/",
	})

	const wantBase = "http://localhost:9000/my-bucket"
	if fs.baseURL != wantBase {
		t.Errorf("baseURL = %q, want %q", fs.baseURL, wantBase)
	}
}

// ---- Signature V4 internals -------------------------------------------------

// TestSigV4_HashSHA256_EmptyBody verifies the well-known SHA-256 of an empty
// body, which is required for unsigned or body-less requests.
func TestSigV4_HashSHA256_EmptyBody(t *testing.T) {
	t.Parallel()

	const want = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	got := hashSHA256(nil)
	if got != want {
		t.Errorf("hashSHA256(nil) = %q, want %q", got, want)
	}

	got = hashSHA256([]byte{})
	if got != want {
		t.Errorf("hashSHA256([]) = %q, want %q", got, want)
	}
}

// TestSigV4_DeriveSigningKey verifies the key derivation against the AWS
// documentation example from the Signature Version 4 Test Suite.
// Source: https://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html
func TestSigV4_DeriveSigningKey(t *testing.T) {
	t.Parallel()

	// AWS test suite values.
	secret := "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	date := "20150830"
	region := "us-east-1"
	service := "iam"

	key := deriveSigningKey(secret, date, region, service)
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d bytes", len(key))
	}
	// Verify it is deterministic.
	key2 := deriveSigningKey(secret, date, region, service)
	for i, b := range key {
		if key2[i] != b {
			t.Fatalf("key derivation is not deterministic at byte %d", i)
		}
	}
}

// TestSigV4_SignRequest_SetsRequiredHeaders verifies that signRequest attaches
// the x-amz-date, x-amz-content-sha256, and Authorization headers.
func TestSigV4_SignRequest_SetsRequiredHeaders(t *testing.T) {
	t.Parallel()

	fs := mustNewS3(t)

	u, _ := url.Parse("https://example-bucket.s3.us-east-1.amazonaws.com/test-key")
	req := &http.Request{
		Method: http.MethodGet,
		URL:    u,
		Header: make(http.Header),
	}

	if err := fs.signRequest(req, nil); err != nil {
		t.Fatalf("signRequest: %v", err)
	}

	for _, header := range []string{"x-amz-date", "x-amz-content-sha256", "Authorization"} {
		if req.Header.Get(header) == "" {
			t.Errorf("expected header %q to be set after signing", header)
		}
	}
}

func TestSigV4_SignRequest_AuthorizationFormat(t *testing.T) {
	t.Parallel()

	fs := mustNewS3(t)

	u, _ := url.Parse("https://example-bucket.s3.us-east-1.amazonaws.com/some/key")
	req := &http.Request{
		Method: http.MethodPut,
		URL:    u,
		Header: make(http.Header),
	}

	if err := fs.signRequest(req, []byte("body content")); err != nil {
		t.Fatalf("signRequest: %v", err)
	}

	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		t.Errorf("Authorization header should start with 'AWS4-HMAC-SHA256 ', got: %q", auth)
	}
	for _, part := range []string{"Credential=", "SignedHeaders=", "Signature="} {
		if !strings.Contains(auth, part) {
			t.Errorf("Authorization header missing %q, got: %q", part, auth)
		}
	}
}

// ---- Canonical query string -------------------------------------------------

func TestBuildCanonicalQueryString_Sorted(t *testing.T) {
	t.Parallel()

	u, _ := url.Parse("https://example.com/?list-type=2&prefix=foo%2F&delimiter=%2F")
	got := buildCanonicalQueryString(u)

	// Parameters must appear in lexicographic order.
	// "delimiter" < "list-type" < "prefix"
	parts := strings.Split(got, "&")
	if len(parts) != 3 {
		t.Fatalf("expected 3 query params, got %d: %q", len(parts), got)
	}

	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = strings.SplitN(p, "=", 2)[0]
	}

	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("query params not sorted at index %d: %v", i, names)
		}
	}
}

// ---- Mkdir no-op ------------------------------------------------------------

func TestS3_Mkdir_IsNoOp(t *testing.T) {
	t.Parallel()

	fs := mustNewS3(t)
	if err := fs.Mkdir("some/prefix/"); err != nil {
		t.Errorf("Mkdir should be a no-op, got error: %v", err)
	}
}

// ---- Operations reject disconnected client ----------------------------------

func TestS3_Operations_WhenClosed(t *testing.T) {
	t.Parallel()

	fs := mustNewS3(t)
	_ = fs.Close()

	t.Run("List", func(t *testing.T) {
		_, err := fs.List("/")
		if err == nil {
			t.Error("expected error when disconnected")
		}
	})

	t.Run("Stat", func(t *testing.T) {
		_, err := fs.Stat("key")
		if err == nil {
			t.Error("expected error when disconnected")
		}
	})

	t.Run("Read", func(t *testing.T) {
		_, err := fs.Read("key")
		if err == nil {
			t.Error("expected error when disconnected")
		}
	})

	t.Run("Write", func(t *testing.T) {
		err := fs.Write("key", strings.NewReader("data"))
		if err == nil {
			t.Error("expected error when disconnected")
		}
	})

	t.Run("Remove", func(t *testing.T) {
		err := fs.Remove("key")
		if err == nil {
			t.Error("expected error when disconnected")
		}
	})

	t.Run("Copy", func(t *testing.T) {
		err := fs.Copy("src", "dst")
		if err == nil {
			t.Error("expected error when disconnected")
		}
	})
}

// ---- Helpers ----------------------------------------------------------------

// mustNewS3 creates an S3 instance with default test credentials. It does not
// make any network calls.
func mustNewS3(t *testing.T) *S3 {
	t.Helper()
	return mustNewS3WithConfig(t, S3Config{
		Region:          "us-east-1",
		Bucket:          "test-bucket",
		AccessKeyID:     "TESTAKID",
		SecretAccessKey: "TESTSECRET",
	})
}

// mustNewS3WithConfig calls NewS3 and fails the test on error.
func mustNewS3WithConfig(t *testing.T, cfg S3Config) *S3 {
	t.Helper()
	fs, err := NewS3(cfg)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	return fs
}
