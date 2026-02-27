//go:build darwin

package credentials

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// randomSuffix returns an 8-digit pseudo-random decimal string suitable for
// use in test service names to prevent collisions between parallel runs.
func randomSuffix() string {
	return fmt.Sprintf("%08d", rand.N(100_000_000))
}

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()

	service := "filepilot-test-" + randomSuffix()
	account := "test-account"
	password := "s3cr3t-p@ssword"

	t.Cleanup(func() {
		// Best-effort cleanup; ignore errors because the entry may already be gone.
		_ = Delete(service, account)
	})

	if err := Save(service, account, password); err != nil {
		t.Fatalf("Save: unexpected error: %v", err)
	}

	got, err := Load(service, account)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	if got != password {
		t.Errorf("Load returned %q, want %q", got, password)
	}
}

func TestSaveUpdate(t *testing.T) {
	t.Parallel()

	service := "filepilot-test-" + randomSuffix()
	account := "update-account"

	t.Cleanup(func() {
		_ = Delete(service, account)
	})

	if err := Save(service, account, "original"); err != nil {
		t.Fatalf("Save (initial): %v", err)
	}

	if err := Save(service, account, "updated"); err != nil {
		t.Fatalf("Save (update): %v", err)
	}

	got, err := Load(service, account)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got != "updated" {
		t.Errorf("Load returned %q after update, want %q", got, "updated")
	}
}

func TestLoadNotFound(t *testing.T) {
	t.Parallel()

	service := "filepilot-test-" + randomSuffix()
	account := "nonexistent-account"

	got, err := Load(service, account)
	if err != nil {
		t.Fatalf("Load: expected nil error for missing entry, got: %v", err)
	}

	if got != "" {
		t.Errorf("Load returned %q, want empty string for missing entry", got)
	}
}

func TestDeleteNotFound(t *testing.T) {
	t.Parallel()

	service := "filepilot-test-" + randomSuffix()
	account := "nonexistent-account"

	if err := Delete(service, account); err != nil {
		t.Fatalf("Delete: expected nil error for missing entry, got: %v", err)
	}
}

func TestDeleteRemovesEntry(t *testing.T) {
	t.Parallel()

	service := "filepilot-test-" + randomSuffix()
	account := "delete-account"

	t.Cleanup(func() {
		_ = Delete(service, account)
	})

	if err := Save(service, account, "to-be-deleted"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := Delete(service, account); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := Load(service, account)
	if err != nil {
		t.Fatalf("Load after Delete: unexpected error: %v", err)
	}

	if got != "" {
		t.Errorf("Load after Delete returned %q, want empty string", got)
	}
}

func TestConnectionCredentialRoundTrip(t *testing.T) {
	t.Parallel()

	connID := "test-conn-" + randomSuffix()
	password := "conn-password-123"

	t.Cleanup(func() {
		_ = DeleteConnectionCredential(connID)
	})

	if err := SaveConnectionCredential(connID, password); err != nil {
		t.Fatalf("SaveConnectionCredential: %v", err)
	}

	got, err := LoadConnectionCredential(connID)
	if err != nil {
		t.Fatalf("LoadConnectionCredential: %v", err)
	}

	if got != password {
		t.Errorf("LoadConnectionCredential returned %q, want %q", got, password)
	}
}

func TestDeleteConnectionCredentialNotFound(t *testing.T) {
	t.Parallel()

	connID := "nonexistent-conn-" + randomSuffix()

	if err := DeleteConnectionCredential(connID); err != nil {
		t.Fatalf("DeleteConnectionCredential: expected nil for missing entry, got: %v", err)
	}
}
