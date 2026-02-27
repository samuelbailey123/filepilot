//go:build darwin

// Package credentials manages stored connection credentials via the macOS Keychain.
package credentials

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	// servicePrefix is prepended to every Keychain service name.
	servicePrefix = "filepilot-"

	// connServicePrefix is the service name prefix used for connection credentials.
	connServicePrefix = servicePrefix + "conn-"

	// cmdTimeout is the maximum time allowed for a single `security` CLI invocation.
	cmdTimeout = 5 * time.Second
)

// Credential holds the three fields that identify a Keychain entry.
type Credential struct {
	Service  string
	Account  string
	Password string
}

// Save stores password under service and account in the macOS Keychain.
// If an entry already exists it is updated in place (via the -U flag).
func Save(service, account, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"security", "add-generic-password",
		"-U",
		"-s", service,
		"-a", account,
		"-w", password,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("credentials: save %q/%q: %w: %s", service, account, err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// Load retrieves the password for service and account from the macOS Keychain.
// It returns an empty string and a nil error when the entry does not exist.
func Load(service, account string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"security", "find-generic-password",
		"-s", service,
		"-a", account,
		"-w",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if isNotFound(stderr.String()) {
			return "", nil
		}
		return "", fmt.Errorf("credentials: load %q/%q: %w: %s", service, account, err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimRight(stdout.String(), "\n"), nil
}

// Delete removes the Keychain entry for service and account.
// It returns nil when the entry does not exist.
func Delete(service, account string) error {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"security", "delete-generic-password",
		"-s", service,
		"-a", account,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if isNotFound(stderr.String()) {
			return nil
		}
		return fmt.Errorf("credentials: delete %q/%q: %w: %s", service, account, err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// SaveConnectionCredential stores the password for a connection identified by connID.
// The Keychain service name is derived as "filepilot-conn-{connID}".
func SaveConnectionCredential(connID, password string) error {
	return Save(connServicePrefix+connID, connID, password)
}

// LoadConnectionCredential retrieves the password for a connection identified by connID.
// It returns an empty string and a nil error when no credential exists for connID.
func LoadConnectionCredential(connID string) (string, error) {
	return Load(connServicePrefix+connID, connID)
}

// DeleteConnectionCredential removes the Keychain entry for a connection identified by connID.
// It returns nil when no credential exists for connID.
func DeleteConnectionCredential(connID string) error {
	return Delete(connServicePrefix+connID, connID)
}

// isNotFound reports whether the stderr output from the `security` tool indicates
// that the requested Keychain item does not exist.
func isNotFound(stderr string) bool {
	return strings.Contains(stderr, "could not be found")
}
