package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// Config represents SSH client configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	KeyPath  string
	Timeout  time.Duration
}

// Client represents an SSH client
type Client struct {
	host     string
	port     int
	user     string
	password string
	keyPath  string
	timeout  time.Duration
	client   *ssh.Client
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// NewClient creates a new SSH client
func NewClient(config Config) (*Client, error) {
	c := &Client{
		host:     config.Host,
		port:     config.Port,
		user:     config.User,
		password: config.Password,
		keyPath:  config.KeyPath,
		timeout:  config.Timeout,
	}
	
	if err := c.connect(); err != nil {
		return nil, err
	}
	
	return c, nil
}

func (c *Client) connect() error {
	sshConfig := &ssh.ClientConfig{
		User:            c.user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         c.timeout,
	}
	
	if c.keyPath != "" {
		keyBytes, err := os.ReadFile(c.keyPath)
		if err != nil {
			return fmt.Errorf("failed to read SSH key: %w", err)
		}
		key, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return fmt.Errorf("failed to parse SSH key: %w", err)
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	} else if c.password != "" {
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(c.password)}
	} else {
		return errors.New("no authentication method provided")
	}
	
	address := fmt.Sprintf("%s:%d", c.host, c.port)
	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	
	c.client = client
	return nil
}

// ExecuteCommand executes a command on the remote server
func (c *Client) ExecuteCommand(cmd string, timeout time.Duration) (*CommandResult, error) {
	if c.client == nil {
		return nil, errors.New("SSH client not connected")
	}
	
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()
	
	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	
	errChan := make(chan error, 1)
	go func() {
		errChan <- session.Run(cmd)
	}()
	
	select {
	case err := <-errChan:
		exitCode := 0
		if err != nil {
			exitCode = 1
		}
		return &CommandResult{
			ExitCode: exitCode,
			Stdout:   stdoutBuf.String(),
			Stderr:   stderrBuf.String(),
		}, err
	case <-time.After(timeout):
		return nil, fmt.Errorf("command timeout after %v", timeout)
	}
}

// ExecuteCommandWithOutput executes a command and returns combined output
func (c *Client) ExecuteCommandWithOutput(cmd string, timeout time.Duration) (string, error) {
	result, err := c.ExecuteCommand(cmd, timeout)
	if err != nil && result.ExitCode != 0 {
		return result.Stdout, fmt.Errorf("command failed (exit code %d): %s", result.ExitCode, result.Stderr)
	}
	
	return result.Stdout, nil
}

// InstallPackage installs a package on the remote server
func (c *Client) InstallPackage(packageName string) error {
	cmd := fmt.Sprintf("apt-get update && apt-get install -y %s", packageName)
	_, err := c.ExecuteCommand(cmd, 120*time.Second)
	return err
}

// CheckPort checks if a port is open
func (c *Client) CheckPort(port int, timeout time.Duration) bool {
	cmd := fmt.Sprintf("nc -z -w5 localhost %d", port)
	result, err := c.ExecuteCommand(cmd, timeout)
	return err == nil && result.ExitCode == 0
}

// TestConnection tests the SSH connection
func (c *Client) TestConnection() error {
	if err := c.connect(); err != nil {
		return err
	}
	return c.Close()
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
