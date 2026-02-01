package profiles

import (
	"fmt"
	"mailops/internal/ssh"
	"time"
)

// DockerMailserverProfile implements Docker MailServer deployment
type DockerMailserverProfile struct {
	Domain        string
	Hostname      string
	ContainerName string
}

// Deploy deploys Docker MailServer
func (p *DockerMailserverProfile) Deploy(sshClient *ssh.Client) (*DeployResult, error) {
	// Install Docker
	if err := p.installDocker(sshClient); err != nil {
		return nil, fmt.Errorf("failed to install Docker: %v", err)
	}
	
	// Create docker-compose file
	if err := p.createDockerCompose(sshClient); err != nil {
		return nil, fmt.Errorf("failed to create docker-compose: %v", err)
	}
	
	// Start containers
	if err := p.startContainers(sshClient); err != nil {
		return nil, fmt.Errorf("failed to start containers: %v", err)
	}
	
	return &DeployResult{
		Version: "Docker MailServer",
		Message: "Deployment successful",
	}, nil
}

// installDocker installs Docker
func (p *DockerMailserverProfile) installDocker(sshClient *ssh.Client) error {
	// Update system
	_, err := sshClient.ExecuteCommandWithOutput("apt-get update", 120*time.Second)
	if err != nil {
		return err
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

// createDockerCompose creates docker-compose.yml file
func (p *DockerMailserverProfile) createDockerCompose(sshClient *ssh.Client) error {
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