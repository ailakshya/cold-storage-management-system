package infra

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHService handles SSH connections to remote nodes
type SSHService struct {
	defaultUser    string
	privateKeyPath string
	timeout        time.Duration
}

// SSHConfig holds SSH connection configuration
type SSHConfig struct {
	Host       string
	Port       int
	User       string
	PrivateKey []byte
	Password   string // For initial setup before key auth
	Timeout    time.Duration
}

// CommandResult holds the result of a command execution
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// NewSSHService creates a new SSH service
func NewSSHService(defaultUser, privateKeyPath string) *SSHService {
	return &SSHService{
		defaultUser:    defaultUser,
		privateKeyPath: privateKeyPath,
		timeout:        30 * time.Second,
	}
}

// getPrivateKey reads the private key from file or returns the provided key
func (s *SSHService) getPrivateKey() ([]byte, error) {
	if s.privateKeyPath == "" {
		return nil, fmt.Errorf("no private key path configured")
	}
	return os.ReadFile(s.privateKeyPath)
}

// getSSHConfig creates SSH client config from parameters
func (s *SSHService) getSSHConfig(user string, privateKey []byte, password string) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	// Try key-based auth first
	if len(privateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// Fallback to password if provided
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method available")
	}

	return &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: implement known_hosts
		Timeout:         s.timeout,
	}, nil
}

// Connect establishes SSH connection to a host
func (s *SSHService) Connect(host string, port int, user string, privateKey []byte, password string) (*ssh.Client, error) {
	if user == "" {
		user = s.defaultUser
	}
	if port == 0 {
		port = 22
	}

	// Use service's private key if not provided
	if len(privateKey) == 0 && s.privateKeyPath != "" {
		var err error
		privateKey, err = s.getPrivateKey()
		if err != nil && password == "" {
			return nil, fmt.Errorf("failed to read private key and no password provided: %w", err)
		}
	}

	config, err := s.getSSHConfig(user, privateKey, password)
	if err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	return client, nil
}

// Execute runs a command on a remote host
func (s *SSHService) Execute(ctx context.Context, host string, command string) (*CommandResult, error) {
	return s.ExecuteWithConfig(ctx, host, 22, s.defaultUser, nil, "", command)
}

// ExecuteWithConfig runs a command with custom SSH config
func (s *SSHService) ExecuteWithConfig(ctx context.Context, host string, port int, user string, privateKey []byte, password string, command string) (*CommandResult, error) {
	start := time.Now()

	client, err := s.Connect(host, port, user, privateKey, password)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Handle context cancellation
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGKILL)
		return nil, ctx.Err()
	case err := <-done:
		result := &CommandResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			Duration: time.Since(start),
		}

		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				result.ExitCode = exitErr.ExitStatus()
			} else {
				return result, err
			}
		}

		return result, nil
	}
}

// TestConnection tests SSH connectivity to a host
func (s *SSHService) TestConnection(ctx context.Context, host string, port int, user string, privateKey []byte, password string) error {
	client, err := s.Connect(host, port, user, privateKey, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// Run a simple command to verify
	result, err := s.ExecuteWithConfig(ctx, host, port, user, privateKey, password, "echo 'OK'")
	if err != nil {
		return err
	}

	if !strings.Contains(result.Stdout, "OK") {
		return fmt.Errorf("connection test failed: unexpected response")
	}

	return nil
}

// Ping tests if host is reachable via TCP
func (s *SSHService) Ping(host string, port int, timeout time.Duration) error {
	if port == 0 {
		port = 22
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("host %s is not reachable: %w", host, err)
	}
	conn.Close()
	return nil
}

// CopyFile copies a local file to a remote host using SCP
func (s *SSHService) CopyFile(ctx context.Context, host string, port int, user string, privateKey []byte, password string, localPath, remotePath string) error {
	client, err := s.Connect(host, port, user, privateKey, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// Read local file
	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	return s.CopyContent(ctx, client, content, remotePath, 0644)
}

// CopyContent copies content to a remote file
func (s *SSHService) CopyContent(ctx context.Context, client *ssh.Client, content []byte, remotePath string, mode os.FileMode) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Use cat to write file content
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		io.Copy(w, bytes.NewReader(content))
	}()

	cmd := fmt.Sprintf("cat > %s && chmod %o %s", remotePath, mode, remotePath)
	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to write remote file: %w", err)
	}

	return nil
}

// CopySSHKey copies the public key to remote authorized_keys
func (s *SSHService) CopySSHKey(ctx context.Context, host string, port int, user string, password string, publicKey []byte) error {
	client, err := s.Connect(host, port, user, nil, password)
	if err != nil {
		return err
	}
	defer client.Close()

	// Ensure .ssh directory exists and has correct permissions
	setupCmd := `mkdir -p ~/.ssh && chmod 700 ~/.ssh`
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	session.Run(setupCmd)
	session.Close()

	// Append public key to authorized_keys
	session, err = client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	keyStr := strings.TrimSpace(string(publicKey))
	cmd := fmt.Sprintf(`grep -qxF '%s' ~/.ssh/authorized_keys 2>/dev/null || echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`, keyStr, keyStr)

	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
	}()

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("failed to add SSH key: %w", err)
	}

	return nil
}

// GetOSInfo returns OS information from remote host
func (s *SSHService) GetOSInfo(ctx context.Context, host string, port int, user string, privateKey []byte, password string) (string, error) {
	result, err := s.ExecuteWithConfig(ctx, host, port, user, privateKey, password, "cat /etc/os-release | grep -E '^(ID|VERSION_ID)=' | tr '\n' ' '")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// ExecuteScript runs a multi-line script on remote host
func (s *SSHService) ExecuteScript(ctx context.Context, host string, port int, user string, privateKey []byte, password string, script string) (*CommandResult, error) {
	// Wrap script in bash
	wrappedScript := fmt.Sprintf("bash -c '%s'", strings.ReplaceAll(script, "'", "'\"'\"'"))
	return s.ExecuteWithConfig(ctx, host, port, user, privateKey, password, wrappedScript)
}
