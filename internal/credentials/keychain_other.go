//go:build !darwin

// Package credentials manages stored connection credentials via the macOS Keychain.
package credentials

import "fmt"

var errUnsupported = fmt.Errorf("credentials: keychain is only supported on macOS")

// Credential holds the three fields that identify a Keychain entry.
type Credential struct {
	Service  string
	Account  string
	Password string
}

// Save is not supported on this platform.
func Save(service, account, password string) error {
	return errUnsupported
}

// Load is not supported on this platform.
func Load(service, account string) (string, error) {
	return "", errUnsupported
}

// Delete is not supported on this platform.
func Delete(service, account string) error {
	return errUnsupported
}

// SaveConnectionCredential is not supported on this platform.
func SaveConnectionCredential(connID, password string) error {
	return errUnsupported
}

// LoadConnectionCredential is not supported on this platform.
func LoadConnectionCredential(connID string) (string, error) {
	return "", errUnsupported
}

// DeleteConnectionCredential is not supported on this platform.
func DeleteConnectionCredential(connID string) error {
	return errUnsupported
}
