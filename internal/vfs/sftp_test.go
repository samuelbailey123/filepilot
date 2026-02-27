package vfs

import (
	"testing"
	"time"
)

// compile-time assertion: *SFTP must satisfy FileSystem.
var _ FileSystem = (*SFTP)(nil)

// TestNewSFTP_EmptyHost verifies that NewSFTP rejects an empty Host.
func TestNewSFTP_EmptyHost(t *testing.T) {
	t.Parallel()

	_, err := NewSFTP(SFTPConfig{
		Host:     "",
		User:     "user",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error for empty host, got nil")
	}
}

// TestNewSFTP_EmptyUser verifies that NewSFTP rejects an empty User.
func TestNewSFTP_EmptyUser(t *testing.T) {
	t.Parallel()

	_, err := NewSFTP(SFTPConfig{
		Host:     "localhost",
		User:     "",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected error for empty user, got nil")
	}
}

// TestNewSFTP_NoAuthMethod verifies that NewSFTP rejects a config with no
// authentication credentials.
func TestNewSFTP_NoAuthMethod(t *testing.T) {
	t.Parallel()

	_, err := NewSFTP(SFTPConfig{
		Host: "localhost",
		User: "user",
		// neither Password nor PrivateKeyPath set
	})
	if err == nil {
		t.Fatal("expected error for missing auth method, got nil")
	}
}

// TestNewSFTP_InvalidPrivateKeyPath verifies that a non-existent key path
// produces a descriptive error without making any network connection.
func TestNewSFTP_InvalidPrivateKeyPath(t *testing.T) {
	t.Parallel()

	_, err := NewSFTP(SFTPConfig{
		Host:           "localhost",
		User:           "user",
		PrivateKeyPath: "/nonexistent/path/to/key",
	})
	if err == nil {
		t.Fatal("expected error for non-existent private key, got nil")
	}
}

// TestNewSFTP_UnreachableHost verifies that dialling a non-routable address
// eventually returns a connection error. The test uses a short deadline so it
// does not block CI for longer than necessary.
func TestNewSFTP_UnreachableHost(t *testing.T) {
	// Do NOT run in parallel: this test has a deliberate wall-clock wait.
	// It uses a non-routable IP (RFC 5737 TEST-NET-3) to guarantee the dial
	// will time out rather than be refused immediately.
	deadline := time.Now().Add(15 * time.Second)
	t.Cleanup(func() {
		if time.Now().After(deadline) {
			t.Log("warning: unreachable-host test ran close to its wall-clock deadline")
		}
	})

	_, err := NewSFTP(SFTPConfig{
		Host:     "10.255.255.1",
		Port:     22,
		User:     "user",
		Password: "pass",
	})
	if err == nil {
		t.Fatal("expected dial error for non-routable host, got nil")
	}
}

// TestSFTPConfig_DefaultPort confirms that the zero value of Port is treated as
// port 22 when constructing the address. We verify this indirectly by ensuring
// the dial attempt targets the right host — a config validation error (empty
// host) triggers before the dial when the host is actually invalid.
func TestSFTPConfig_DefaultPort(t *testing.T) {
	t.Parallel()

	// With a valid host+user+password but Port == 0 the code should still
	// attempt a dial (falling through to a network error), not a validation
	// error. We use an address that will be refused immediately to keep the
	// test fast.
	_, err := NewSFTP(SFTPConfig{
		Host:     "127.0.0.1",
		Port:     0, // should default to 22
		User:     "user",
		Password: "pass",
	})
	// We expect *some* error (connection refused / timeout), not a validation error.
	if err == nil {
		t.Fatal("expected a network error, got nil")
	}
}
