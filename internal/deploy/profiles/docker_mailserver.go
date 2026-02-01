package profiles

import (
	"fmt"
	"mailops/internal/ssh"
	"strings"
	"time"
)

// DockerMailserverProfile implements Docker MailServer deployment
type DockerMailserverProfile struct {
	Domain        string
	Hostname      string
	ContainerName string
	DKIMSelector  string
}

// Deploy deploys Docker MailServer
func (p *DockerMailserverProfile) Deploy(sshClient *ssh.Client) (*DeployResult, error) {
	// Check and install Docker
	if err := p.checkAndInstallDocker(sshClient); err != nil {
		return nil, fmt.Errorf("failed to check/install Docker: %v", err)
	}
	
	// Check and install Docker Compose
	if err := p.checkAndInstallDockerCompose(sshClient); err != nil {
		return nil, fmt.Errorf("failed to check/install Docker Compose: %v", err)
	}
	
	// Create docker-compose file
	if err := p.createDockerCompose(sshClient); err != nil {
		return nil, fmt.Errorf("failed to create docker-compose: %v", err)
	}
	
	// Start containers
	if err := p.startContainers(sshClient); err != nil {
		return nil, fmt.Errorf("failed to start containers: %v", err)
	}
	
	// Health check
	if err := p.healthCheck(sshClient); err != nil {
		return nil, fmt.Errorf("health check failed: %v", err)
	}
	
	return &DeployResult{
		Version: "Docker MailServer",
		Message: "Deployment successful",
	}, nil
}

// checkAndInstallDocker checks if Docker is installed and installs if needed
func (p *DockerMailserverProfile) checkAndInstallDocker(sshClient *ssh.Client) error {
	// Check if Docker is already installed
	checkCmd := "docker --version"
	result, err := sshClient.ExecuteCommand(checkCmd, 10*time.Second)
	if err == nil && result.ExitCode == 0 {
		// Docker is installed
		return nil
	}
	
	// Install Docker
	_, err = sshClient.ExecuteCommandWithOutput("apt-get update", 120*time.Second)
	if err != nil {
		return fmt.Errorf("apt-get update failed: %v", err)
	}
	
	// Install Docker dependencies
	packages := []string{
		"apt-transport-https",
		"ca-certificates",
		"curl",
		"gnupg",
		"lsb-release",
	}
	
	for _, pkg := range packages {
		if err := sshClient.InstallPackage(pkg); err != nil {
			return fmt.Errorf("failed to install %s: %v", pkg, err)
		}
	}
	
	// Add Docker's official GPG key
	cmd := "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg"
	_, err = sshClient.ExecuteCommandWithOutput(cmd, 60*time.Second)
	if err != nil {
		return err
	}
	
	// Set up Docker repository
	cmd = `echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null`
	_, err = sshClient.ExecuteCommandWithOutput(cmd, 60*time.Second)
	if err != nil {
		return err
	}
	
	// Update and install Docker
	_, err = sshClient.ExecuteCommandWithOutput("apt-get update", 60*time.Second)
	if err != nil {
		return err
	}
	
	if err := sshClient.InstallPackage("docker-ce"); err != nil {
		return err
	}
	
	// Enable and start Docker
	_, err = sshClient.ExecuteCommandWithOutput("systemctl enable docker", 30*time.Second)
	if err != nil {
		return err
	}
	
	_, err = sshClient.ExecuteCommandWithOutput("systemctl start docker", 30*time.Second)
	if err != nil {
		return err
	}
	
	return nil
}

// checkAndInstallDockerCompose checks if Docker Compose is installed and installs if needed
func (p *DockerMailserverProfile) checkAndInstallDockerCompose(sshClient *ssh.Client) error {
	// Check if docker-compose is already installed
	checkCmd := "docker-compose --version"
	result, err := sshClient.ExecuteCommand(checkCmd, 10*time.Second)
	if err == nil && result.ExitCode == 0 {
		// docker-compose is installed
		return nil
	}
	
	// Install docker-compose using official script
	cmd := "curl -L &quot;https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)&quot; -o /usr/local/bin/docker-compose"
	_, err = sshClient.ExecuteCommandWithOutput(cmd, 120*time.Second)
	if err != nil {
		return fmt.Errorf("failed to download docker-compose: %v", err)
	}
	
	// Make executable
	_, err = sshClient.ExecuteCommandWithOutput("chmod +x /usr/local/bin/docker-compose", 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to make docker-compose executable: %v", err)
	}
	
	return nil
}

// createDockerCompose creates docker-compose.yml file
func (p *DockerMailserverProfile) createDockerCompose(sshClient *ssh.Client) error {
	selector := p.DKIMSelector
	if selector == "" {
		selector = "mail"
	}

	dockerCompose := fmt.Sprintf(`
version: '3.8'
services:
  mailserver:
    image: mailserver/docker-mailserver:latest
    container_name: %s
    hostname: %s.%s
    ports:
      - "25:25"
      - "587:587"
      - "465:465"
      - "143:143"
      - "993:993"
      - "110:110"
      - "995:995"
    environment:
      - TZ=UTC
      - ENABLE_SPAMASSASSIN=1
      - ENABLE_CLAMAV=1
      - ENABLE_POSTGREY=1
      - ENABLE_FAIL2BAN=1
      - ENABLE_MANAGESIEVE=1
      - ONE_DIR=1
      - ENABLE_POP3=1
      - SSL_TYPE=self-signed
      - ENABLE_OPENDKIM=1
      - ENABLE_OPENDMARC=1
      - ENABLE_POLICYD_SPF=1
      - POSTFIX_DKIM_SELECTOR=%s
    volumes:
      - ./maildata:/var/mail
      - ./mailstate:/var/mail-state
      - ./maillogs:/var/log/mail
      - ./config:/tmp/docker-mailserver
    cap_add:
      - NET_ADMIN
      - SYS_PTRACE
    restart: always
`,
		p.ContainerName,
		p.Hostname,
		p.Domain,
		selector,
	)
	
	cmd := fmt.Sprintf("mkdir -p /opt/mailserver && cd /opt/mailserver && cat > docker-compose.yml << 'EOF'\n%s\nEOF", dockerCompose)
	_, err := sshClient.ExecuteCommandWithOutput(cmd, 60*time.Second)
	if err != nil {
		return err
	}
	
	return nil
}

// startContainers starts the mailserver container
func (p *DockerMailserverProfile) startContainers(sshClient *ssh.Client) error {
	cmd := "cd /opt/mailserver && docker-compose pull"
	_, err := sshClient.ExecuteCommandWithOutput(cmd, 300*time.Second)
	if err != nil {
		return err
	}
	
	cmd = "cd /opt/mailserver && docker-compose up -d"
	_, err = sshClient.ExecuteCommandWithOutput(cmd, 120*time.Second)
	if err != nil {
		return err
	}
	
	return nil
}

// healthCheck performs basic health checks on the mailserver
func (p *DockerMailserverProfile) healthCheck(sshClient *ssh.Client) error {
	// Wait for container to be ready
	time.Sleep(10 * time.Second)
	
	// Check if container is running
	checkCmd := fmt.Sprintf("docker ps | grep %s", p.ContainerName)
	result, err := sshClient.ExecuteCommand(checkCmd, 10*time.Second)
	if err != nil || result.ExitCode != 0 {
		return fmt.Errorf("container %s is not running", p.ContainerName)
	}
	
	// Check critical ports
	ports := []int{25, 587, 465, 143, 993}
	for _, port := range ports {
		if !sshClient.CheckPort(port, 5*time.Second) {
			return fmt.Errorf("port %d is not responding", port)
		}
	}
	
	return nil
}

// GenerateDKIM generates DKIM keys using docker-mailserver
func (p *DockerMailserverProfile) GenerateDKIM(sshClient *ssh.Client) (string, error) {
	selector := p.DKIMSelector
	if selector == "" {
		selector = "mail"
	}
	
	// Generate DKIM keys using docker-mailserver's setup script
	cmd := fmt.Sprintf("docker exec %s setup config dkim", p.ContainerName)
	output, err := sshClient.ExecuteCommandWithOutput(cmd, 60*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to generate DKIM keys: %v, output: %s", err, output)
	}
	
	// Read DKIM public key from the config volume
	// The key file is located at: /tmp/docker-mailserver/opendkim/<selector>.txt
	dkimKeyPath := fmt.Sprintf("/opt/mailserver/config/opendkim/%s.txt", selector)
	
	// Wait a moment for files to be written
	time.Sleep(2 * time.Second)
	
	// Read the public key file
	dkimPublicKey, err := sshClient.ExecuteCommandWithOutput(fmt.Sprintf("cat %s", dkimKeyPath), 30*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to read DKIM public key: %v", err)
	}
	
	// Normalize DKIM key - extract just the p= value
	dkimPublicKey = p.normalizeDKIMKey(dkimPublicKey)
	
	if dkimPublicKey == "" {
		return "", fmt.Errorf("DKIM public key is empty")
	}
	
	return dkimPublicKey, nil
}

// normalizeDKIMKey extracts the p= value from DKIM record
func (p *DockerMailserverProfile) normalizeDKIMKey(dkimContent string) string {
	lines := strings.Split(dkimContent, "\n")
	var keyParts []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.Contains(line, "p=") {
			// Extract the p= value
			parts := strings.SplitN(line, "p=", 2)
			if len(parts) == 2 {
				keyValue := strings.TrimSpace(parts[1])
				// Remove quotes and trailing semicolon
				keyValue = strings.Trim(keyValue, `"`)
				keyValue = strings.TrimSuffix(keyValue, ";")
				keyValue = strings.TrimSpace(keyValue)
				keyParts = append(keyParts, keyValue)
			}
		}
	}
	
	if len(keyParts) == 0 {
		return dkimContent
	}
	
	return fmt.Sprintf("v=DKIM1; k=rsa; p=%s", keyParts[0])
}