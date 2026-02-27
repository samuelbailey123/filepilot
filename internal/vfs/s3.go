package vfs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// S3Config holds the configuration required to communicate with an S3-compatible
// object store. Endpoint is optional; when empty the AWS regional endpoint is
// derived from Region and Bucket.
type S3Config struct {
	// Region is the AWS region (e.g., "us-east-1"). Required.
	Region string

	// Bucket is the S3 bucket name. Required.
	Bucket string

	// AccessKeyID is the AWS access key identifier. Required.
	AccessKeyID string

	// SecretAccessKey is the AWS secret access key. Required.
	SecretAccessKey string

	// Endpoint is an optional base URL for S3-compatible stores such as MinIO.
	// When empty the standard AWS endpoint is used:
	//   https://{bucket}.s3.{region}.amazonaws.com
	Endpoint string
}

// s3Object is the XML element returned inside a ListObjectsV2 response for each
// non-directory object.
type s3Object struct {
	Key          string    `xml:"Key"`
	Size         int64     `xml:"Size"`
	LastModified time.Time `xml:"LastModified"`
}

// s3CommonPrefix is the XML element for a logical directory in a ListObjectsV2
// response (objects that share a key prefix up to the delimiter).
type s3CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

// s3ListResult is the parsed XML body of a ListObjectsV2 (list-type=2) response.
type s3ListResult struct {
	XMLName        xml.Name         `xml:"ListBucketResult"`
	Contents       []s3Object       `xml:"Contents"`
	CommonPrefixes []s3CommonPrefix `xml:"CommonPrefixes"`
	IsTruncated    bool             `xml:"IsTruncated"`
}

// S3 implements the FileSystem interface against an S3-compatible object store.
// All HTTP requests are signed using AWS Signature Version 4 using only the
// standard library — no external AWS SDK is required.
//
// Operations are protected by a mutex that guards the connected flag. The
// underlying http.Client is safe for concurrent use and does not require
// serialisation.
type S3 struct {
	mu        sync.Mutex
	config    S3Config
	client    *http.Client
	baseURL   string
	connected bool
}

// NewS3 validates config and creates an S3 filesystem instance. It does not
// make any network calls; use IsConnected after the first real operation if
// you need to verify reachability.
func NewS3(config S3Config) (*S3, error) {
	if config.Region == "" {
		return nil, errors.New("s3: region must not be empty")
	}
	if config.Bucket == "" {
		return nil, errors.New("s3: bucket must not be empty")
	}
	if config.AccessKeyID == "" {
		return nil, errors.New("s3: access key ID must not be empty")
	}
	if config.SecretAccessKey == "" {
		return nil, errors.New("s3: secret access key must not be empty")
	}

	baseURL := config.Endpoint
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", config.Bucket, config.Region)
	} else {
		// Strip trailing slash so all path concatenation is uniform.
		baseURL = strings.TrimRight(baseURL, "/")
		// For S3-compatible stores the bucket is part of the path, not the host.
		baseURL = baseURL + "/" + config.Bucket
	}

	return &S3{
		config:    config,
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   baseURL,
		connected: true,
	}, nil
}

// Close marks the S3 filesystem as disconnected. The underlying HTTP client has
// no persistent connections that need explicit teardown, so Close is a logical
// operation only.
func (s *S3) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connected = false
	return nil
}

// IsConnected reports whether the S3 filesystem is active (i.e., Close has not
// been called).
func (s *S3) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

// List returns the objects and logical directories immediately under path using
// the S3 ListObjectsV2 API with a "/" delimiter. Path "" or "/" lists the
// bucket root.
func (s *S3) List(keyPath string) ([]FileEntry, error) {
	if !s.IsConnected() {
		return nil, errors.New("s3: not connected")
	}

	prefix := normS3Prefix(keyPath)

	reqURL := s.baseURL + "/?list-type=2&delimiter=%2F"
	if prefix != "" {
		reqURL += "&prefix=" + url.QueryEscape(prefix)
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("s3 list build request: %w", err)
	}

	if err := s.signRequest(req, nil); err != nil {
		return nil, fmt.Errorf("s3 list sign request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 list %s: %w", keyPath, err)
	}
	defer resp.Body.Close()

	if err := s3CheckStatus(resp); err != nil {
		return nil, fmt.Errorf("s3 list %s: %w", keyPath, err)
	}

	var result s3ListResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("s3 list decode response: %w", err)
	}

	entries := make([]FileEntry, 0, len(result.Contents)+len(result.CommonPrefixes))

	for _, cp := range result.CommonPrefixes {
		dirKey := strings.TrimSuffix(cp.Prefix, "/")
		entries = append(entries, FileEntry{
			Path:        dirKey,
			Name:        path.Base(dirKey),
			IsDir:       true,
			Permissions: 0755,
		})
	}

	for _, obj := range result.Contents {
		// Skip the prefix itself when it is returned as a zero-byte object.
		if obj.Key == prefix {
			continue
		}
		entries = append(entries, FileEntry{
			Path:        obj.Key,
			Name:        path.Base(obj.Key),
			Size:        obj.Size,
			IsDir:       false,
			ModTime:     obj.LastModified,
			Permissions: 0644,
		})
	}

	return entries, nil
}

// Stat returns metadata for the object at keyPath using an HTTP HEAD request.
// For logical directories (keys ending with "/") a synthetic FileEntry is
// returned when the object exists.
func (s *S3) Stat(keyPath string) (*FileEntry, error) {
	if !s.IsConnected() {
		return nil, errors.New("s3: not connected")
	}

	key := normS3Key(keyPath)
	req, err := http.NewRequest(http.MethodHead, s.objectURL(key), nil)
	if err != nil {
		return nil, fmt.Errorf("s3 stat build request: %w", err)
	}

	if err := s.signRequest(req, nil); err != nil {
		return nil, fmt.Errorf("s3 stat sign request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 stat %s: %w", keyPath, err)
	}
	defer resp.Body.Close()

	if err := s3CheckStatus(resp); err != nil {
		return nil, fmt.Errorf("s3 stat %s: %w", keyPath, err)
	}

	modTime, _ := http.ParseTime(resp.Header.Get("Last-Modified"))

	var size int64
	fmt.Sscanf(resp.Header.Get("Content-Length"), "%d", &size)

	return &FileEntry{
		Path:        key,
		Name:        path.Base(key),
		Size:        size,
		IsDir:       strings.HasSuffix(key, "/"),
		ModTime:     modTime,
		Permissions: 0644,
	}, nil
}

// Read fetches the object at keyPath with an HTTP GET and returns a streaming
// ReadCloser. The caller is responsible for closing the returned reader.
func (s *S3) Read(keyPath string) (io.ReadCloser, error) {
	if !s.IsConnected() {
		return nil, errors.New("s3: not connected")
	}

	key := normS3Key(keyPath)
	req, err := http.NewRequest(http.MethodGet, s.objectURL(key), nil)
	if err != nil {
		return nil, fmt.Errorf("s3 read build request: %w", err)
	}

	if err := s.signRequest(req, nil); err != nil {
		return nil, fmt.Errorf("s3 read sign request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 read %s: %w", keyPath, err)
	}

	if err := s3CheckStatus(resp); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("s3 read %s: %w", keyPath, err)
	}

	return resp.Body, nil
}

// Write uploads the content of r to S3 at keyPath using an HTTP PUT. Content
// length is not required but performance is improved if r also implements
// io.Seeker (allowing the SDK to set Content-Length without buffering).
func (s *S3) Write(keyPath string, r io.Reader) error {
	if !s.IsConnected() {
		return errors.New("s3: not connected")
	}

	key := normS3Key(keyPath)

	// Determine content length if the reader exposes it, otherwise use -1 to
	// stream with chunked transfer encoding.
	var contentLength int64 = -1
	if sizer, ok := r.(interface{ Len() int }); ok {
		contentLength = int64(sizer.Len())
	}

	req, err := http.NewRequest(http.MethodPut, s.objectURL(key), r)
	if err != nil {
		return fmt.Errorf("s3 write build request: %w", err)
	}
	req.ContentLength = contentLength

	if err := s.signRequest(req, nil); err != nil {
		return fmt.Errorf("s3 write sign request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("s3 write %s: %w", keyPath, err)
	}
	defer resp.Body.Close()

	return s3CheckStatus(resp)
}

// Mkdir is a no-op for S3 because the object store does not have real
// directories — they are simulated by key prefixes. The method exists only to
// satisfy the FileSystem interface.
func (s *S3) Mkdir(_ string) error {
	return nil
}

// Remove deletes the object at keyPath using an HTTP DELETE.
func (s *S3) Remove(keyPath string) error {
	if !s.IsConnected() {
		return errors.New("s3: not connected")
	}

	key := normS3Key(keyPath)
	req, err := http.NewRequest(http.MethodDelete, s.objectURL(key), nil)
	if err != nil {
		return fmt.Errorf("s3 remove build request: %w", err)
	}

	if err := s.signRequest(req, nil); err != nil {
		return fmt.Errorf("s3 remove sign request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("s3 remove %s: %w", keyPath, err)
	}
	defer resp.Body.Close()

	// 204 No Content is the success status for DELETE.
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}
	return s3CheckStatus(resp)
}

// Rename moves an object by copying it to the new key then deleting the
// original. S3 has no native rename primitive so this is a two-step operation
// and is not atomic.
func (s *S3) Rename(oldPath, newPath string) error {
	if err := s.Copy(oldPath, newPath); err != nil {
		return fmt.Errorf("s3 rename copy %s -> %s: %w", oldPath, newPath, err)
	}
	if err := s.Remove(oldPath); err != nil {
		return fmt.Errorf("s3 rename remove %s: %w", oldPath, err)
	}
	return nil
}

// Copy duplicates the object at src to dst using the S3 server-side copy
// primitive (PUT with x-amz-copy-source). No data traverses the client.
func (s *S3) Copy(src, dst string) error {
	if !s.IsConnected() {
		return errors.New("s3: not connected")
	}

	srcKey := normS3Key(src)
	dstKey := normS3Key(dst)

	req, err := http.NewRequest(http.MethodPut, s.objectURL(dstKey), nil)
	if err != nil {
		return fmt.Errorf("s3 copy build request: %w", err)
	}

	// The copy-source header must include the bucket name when using path-style
	// endpoints; for virtual-host style the path alone is sufficient. Including
	// the bucket is always safe because AWS accepts both forms.
	copySource := url.PathEscape("/" + s.config.Bucket + "/" + srcKey)
	req.Header.Set("x-amz-copy-source", copySource)

	if err := s.signRequest(req, nil); err != nil {
		return fmt.Errorf("s3 copy sign request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("s3 copy %s -> %s: %w", src, dst, err)
	}
	defer resp.Body.Close()

	return s3CheckStatus(resp)
}

// Move is identical to Rename for S3 — a server-side copy followed by a delete.
func (s *S3) Move(src, dst string) error {
	return s.Rename(src, dst)
}

// objectURL constructs the full URL for a specific S3 object key.
func (s *S3) objectURL(key string) string {
	return s.baseURL + "/" + key
}

// normS3Key trims leading and trailing slashes and returns a clean object key.
// An empty string maps to the bucket root (represented as "").
func normS3Key(p string) string {
	return strings.Trim(p, "/")
}

// normS3Prefix converts a path into a key prefix suitable for use in
// ListObjectsV2. A non-empty prefix always ends with "/" so that the delimiter
// "/" will correctly yield one level of results.
func normS3Prefix(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	return p + "/"
}

// s3CheckStatus returns a descriptive error for any non-2xx HTTP response.
// The response body is drained so the connection can be reused.
func s3CheckStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// ---- AWS Signature Version 4 -----------------------------------------------

// signRequest computes and attaches an AWS Signature V4 Authorization header to
// req. bodyBytes must be the raw request body (nil or empty for requests without
// a body). The signing uses HMAC-SHA256 throughout, as specified by AWS.
//
// Reference: https://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-authenticating-requests.html
func (s *S3) signRequest(req *http.Request, bodyBytes []byte) error {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	// Step 1: Canonical request.
	payloadHash := hashSHA256(bodyBytes)

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	if req.Host == "" {
		u, err := url.Parse(s.baseURL)
		if err != nil {
			return fmt.Errorf("s3 sign: parse base URL: %w", err)
		}
		req.Host = u.Host
	}

	canonicalHeaders, signedHeaders := buildCanonicalHeaders(req)

	canonicalQueryString := buildCanonicalQueryString(req.URL)

	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// Step 2: String to sign.
	credentialScope := strings.Join([]string{
		dateStamp,
		s.config.Region,
		"s3",
		"aws4_request",
	}, "/")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hashSHA256([]byte(canonicalRequest)),
	}, "\n")

	// Step 3: Signing key.
	signingKey := deriveSigningKey(s.config.SecretAccessKey, dateStamp, s.config.Region, "s3")

	// Step 4: Signature.
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// Step 5: Authorization header.
	authorization := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.config.AccessKeyID,
		credentialScope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", authorization)
	return nil
}

// buildCanonicalHeaders returns the canonical headers string and the
// semicolon-separated list of signed header names as required by SigV4.
// Only the headers that must appear in the signature are included.
func buildCanonicalHeaders(req *http.Request) (canonicalHeaders, signedHeaders string) {
	// Collect the header names we need to sign.
	headersToSign := map[string]string{
		"host":                 hostHeader(req),
		"x-amz-date":          req.Header.Get("x-amz-date"),
		"x-amz-content-sha256": req.Header.Get("x-amz-content-sha256"),
	}

	// Include x-amz-copy-source when present.
	if v := req.Header.Get("x-amz-copy-source"); v != "" {
		headersToSign["x-amz-copy-source"] = v
	}

	names := make([]string, 0, len(headersToSign))
	for k := range headersToSign {
		names = append(names, k)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		sb.WriteString(name)
		sb.WriteByte(':')
		sb.WriteString(strings.TrimSpace(headersToSign[name]))
		sb.WriteByte('\n')
	}

	return sb.String(), strings.Join(names, ";")
}

// hostHeader returns the value for the Host header used in canonical signing.
// It prefers req.Host, then req.URL.Host.
func hostHeader(req *http.Request) string {
	if req.Host != "" {
		return req.Host
	}
	return req.URL.Host
}

// buildCanonicalQueryString returns the percent-encoded, lexicographically
// sorted query string required by the SigV4 canonical request.
func buildCanonicalQueryString(u *url.URL) string {
	// Re-parse the raw query to get individual parameters, then sort and
	// percent-encode them according to the SigV4 specification.
	params, _ := url.ParseQuery(u.RawQuery)

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		vals := params[k]
		sort.Strings(vals)
		for _, v := range vals {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

// deriveSigningKey computes the SigV4 derived signing key using four successive
// HMAC-SHA256 operations over the secret key, date, region, and service name.
func deriveSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

// hmacSHA256 computes HMAC-SHA256 of data using key.
func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// hashSHA256 returns the lowercase hex-encoded SHA-256 digest of data. A nil
// or empty slice produces the well-known empty-body hash.
func hashSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
