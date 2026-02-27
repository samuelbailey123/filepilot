package vfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const (
	defaultSFTPPort    = 22
	connectionTimeout  = 10 * time.Second
)

// SFTPConfig holds the configuration required to establish an SFTP connection.
// Either Password or PrivateKeyPath must be provided for authentication.
// If both are provided, PrivateKeyPath is tried first.
type SFTPConfig struct {
	// Host is the remote server hostname or IP address.
	Host string

	// Port is the SSH port. Defaults to 22 when zero.
	Port int

	// User is the SSH login username.
	User string

	// Password is used for password-based authentication (optional).
	Password string

	// PrivateKeyPath is the path to an SSH private key file (optional).
	PrivateKeyPath string

	// PrivateKeyPassphrase is the passphrase for the private key (optional).
	PrivateKeyPassphrase string
}

// SFTP implements the FileSystem interface using an SSH/SFTP connection.
// Operations are guarded by a mutex to make the type safe for concurrent use.
type SFTP struct {
	mu          sync.Mutex
	sshClient   *ssh.Client
	sftpClient  *sftp.Client
	connected   bool
}

// NewSFTP establishes an SSH connection and opens an SFTP subsystem session.
// Authentication order: private key (if PrivateKeyPath is set), then password.
// The SSH dial uses a 10-second timeout.
func NewSFTP(config SFTPConfig) (*SFTP, error) {
	if config.Host == "" {
		return nil, errors.New("sftp: host must not be empty")
	}
	if config.User == "" {
		return nil, errors.New("sftp: user must not be empty")
	}

	port := config.Port
	if port == 0 {
		port = defaultSFTPPort
	}

	authMethods, err := buildAuthMethods(config)
	if err != nil {
		return nil, fmt.Errorf("sftp: build auth methods: %w", err)
	}
	if len(authMethods) == 0 {
		return nil, errors.New("sftp: no authentication method configured; provide PrivateKeyPath or Password")
	}

	sshCfg := &ssh.ClientConfig{
		User:            config.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // host-key verification is a caller responsibility
		Timeout:         connectionTimeout,
	}

	addr := net.JoinHostPort(config.Host, fmt.Sprintf("%d", port))
	sshClient, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("sftp: dial %s: %w", addr, err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, fmt.Errorf("sftp: open sftp subsystem: %w", err)
	}

	return &SFTP{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		connected:  true,
	}, nil
}

// buildAuthMethods constructs the ordered list of SSH authentication methods
// from the provided config. Private key is attempted before password.
func buildAuthMethods(config SFTPConfig) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if config.PrivateKeyPath != "" {
		signer, err := loadPrivateKey(config.PrivateKeyPath, config.PrivateKeyPassphrase)
		if err != nil {
			return nil, fmt.Errorf("load private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if config.Password != "" {
		methods = append(methods, ssh.Password(config.Password))
	}

	return methods, nil
}

// loadPrivateKey parses an SSH private key from disk. If passphrase is
// non-empty it is used to decrypt the key.
func loadPrivateKey(path, passphrase string) (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read private key %s: %w", path, err)
	}

	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("parse private key with passphrase: %w", err)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return signer, nil
}

// IsConnected reports whether the SFTP session is currently active.
func (s *SFTP) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

// Close terminates the SFTP subsystem session and the underlying SSH connection.
// It is safe to call Close multiple times.
func (s *SFTP) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected {
		return nil
	}
	s.connected = false

	var errs []error
	if err := s.sftpClient.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close sftp client: %w", err))
	}
	if err := s.sshClient.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close ssh connection: %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// List returns the contents of the directory at path.
func (s *SFTP) List(path string) ([]FileEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	infos, err := s.sftpClient.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("sftp list %s: %w", path, err)
	}

	entries := make([]FileEntry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, fileEntryFromInfo(filepath.Join(path, info.Name()), info))
	}
	return entries, nil
}

// Stat returns metadata for the file or directory at path.
func (s *SFTP) Stat(path string) (*FileEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := s.sftpClient.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("sftp stat %s: %w", path, err)
	}

	entry := fileEntryFromInfo(path, info)
	return &entry, nil
}

// Read opens the remote file at path for reading. The caller is responsible
// for closing the returned ReadCloser.
func (s *SFTP) Read(path string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := s.sftpClient.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sftp open %s: %w", path, err)
	}
	return f, nil
}

// Write creates or overwrites the remote file at path with the contents of r.
// Parent directories must already exist.
func (s *SFTP) Write(path string, r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := s.sftpClient.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("sftp create %s: %w", path, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("sftp write %s: %w", path, err)
	}
	return f.Close()
}

// Mkdir creates the directory at path, including any missing parent directories.
func (s *SFTP) Mkdir(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.sftpClient.MkdirAll(path); err != nil {
		return fmt.Errorf("sftp mkdir %s: %w", path, err)
	}
	return nil
}

// Remove deletes the file or directory at path. For directories the entry must
// be empty; the SFTP protocol does not support recursive removal.
func (s *SFTP) Remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try file removal first; fall back to directory removal.
	err := s.sftpClient.Remove(path)
	if err == nil {
		return nil
	}

	// If the target is a directory, Remove returns an error on most servers.
	if rmDirErr := s.sftpClient.RemoveDirectory(path); rmDirErr != nil {
		// Surface the original error, not the rmdir error, as it is more informative.
		return fmt.Errorf("sftp remove %s: %w", path, err)
	}
	return nil
}

// Rename renames or moves oldPath to newPath on the remote server.
func (s *SFTP) Rename(oldPath, newPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.sftpClient.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("sftp rename %s -> %s: %w", oldPath, newPath, err)
	}
	return nil
}

// Copy copies the file at src to dst on the remote server by streaming its
// content through a local buffer. The SFTP protocol has no native server-side
// copy primitive so data must travel through the client.
func (s *SFTP) Copy(src, dst string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	srcFile, err := s.sftpClient.Open(src)
	if err != nil {
		return fmt.Errorf("sftp copy open src %s: %w", src, err)
	}
	defer srcFile.Close()

	// Buffer the entire source in memory so we hold the lock throughout and
	// avoid potential interleaving on the underlying SSH channel.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, srcFile); err != nil {
		return fmt.Errorf("sftp copy read src %s: %w", src, err)
	}

	dstFile, err := s.sftpClient.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("sftp copy create dst %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, &buf); err != nil {
		return fmt.Errorf("sftp copy write dst %s: %w", dst, err)
	}
	return dstFile.Close()
}

// Move moves src to dst on the remote server. Because both paths are on the
// same SFTP server this is implemented as a rename, which is an atomic
// operation on the remote filesystem.
func (s *SFTP) Move(src, dst string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.sftpClient.Rename(src, dst); err != nil {
		return fmt.Errorf("sftp move %s -> %s: %w", src, dst, err)
	}
	return nil
}

// fileEntryFromInfo converts an os.FileInfo into a FileEntry using the
// provided absolute path.
func fileEntryFromInfo(path string, info os.FileInfo) FileEntry {
	return FileEntry{
		Path:        path,
		Name:        info.Name(),
		Size:        info.Size(),
		IsDir:       info.IsDir(),
		ModTime:     info.ModTime(),
		Permissions: int(info.Mode().Perm()),
	}
}
