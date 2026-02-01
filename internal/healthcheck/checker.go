package healthcheck

import (
	"fmt"
	"mailops/internal/ssh"
	"time"
)

// CheckResult represents health check result
type CheckResult struct {
	Overall bool
	Checks  []SingleCheck
}

// SingleCheck represents a single health check
type SingleCheck struct {
	Name    string
	Passed  bool
	Reason  string
	Duration time.Duration
}

// Checker performs health checks
type Checker struct {
	sshClient *ssh.Client
	ports     []int
	services  []string
	timeout   time.Duration
}

// NewChecker creates a new health checker
func NewChecker(sshClient *ssh.Client, ports []int, services []string, timeout time.Duration) *Checker {
	return &Checker{
		sshClient: sshClient,
		ports:     ports,
		services:  services,
		timeout:   timeout,
	}
}

// Check performs all health checks
func (c *Checker) Check() (*CheckResult, error) {
	result := &CheckResult{
		Overall: true,
		Checks:  make([]SingleCheck, 0),
	}
	
	// Check ports
	for _, port := range c.ports {
		check := c.checkPort(port)
		result.Checks = append(result.Checks, check)
		if !check.Passed {
			result.Overall = false
		}
	}
	
	// Check services
	for _, service := range c.services {
		check := c.checkService(service)
		result.Checks = append(result.Checks, check)
		if !check.Passed {
			result.Overall = false
		}
	}
	
	return result, nil
}

// checkPort checks if a port is listening
func (c *Checker) checkPort(port int) SingleCheck {
	startTime := time.Now()
	
	output, err := c.sshClient.ExecuteCommandWithOutput(
		fmt.Sprintf("netstat -tln 2>/dev/null | grep ':%d ' || ss -tln 2>/dev/null | grep ':%d '", port, port),
		c.timeout,
	)
	
	duration := time.Since(startTime)
	
	if err != nil {
		return SingleCheck{
			Name:     fmt.Sprintf("Port %d", port),
			Passed:   false,
			Reason:   err.Error(),
			Duration: duration,
		}
	}
	
	if output == "" {
		return SingleCheck{
			Name:     fmt.Sprintf("Port %d", port),
			Passed:   false,
			Reason:   "Port not listening",
			Duration: duration,
		}
	}
	
	return SingleCheck{
		Name:     fmt.Sprintf("Port %d", port),
		Passed:   true,
		Reason:   "Port is listening",
		Duration: duration,
	}
}

// checkService checks if a service is running
func (c *Checker) checkService(service string) SingleCheck {
	startTime := time.Now()
	
	// Check service status using systemctl
	output, err := c.sshClient.ExecuteCommandWithOutput(
		fmt.Sprintf("systemctl is-active %s 2>/dev/null", service),
		c.timeout,
	)
	
	duration := time.Since(startTime)
	
	if err != nil {
		return SingleCheck{
			Name:     fmt.Sprintf("Service %s", service),
			Passed:   false,
			Reason:   err.Error(),
			Duration: duration,
		}
	}
	
	if output != "active" {
		return SingleCheck{
			Name:     fmt.Sprintf("Service %s", service),
			Passed:   false,
			Reason:   fmt.Sprintf("Service status: %s", output),
			Duration: duration,
		}
	}
	
	return SingleCheck{
		Name:     fmt.Sprintf("Service %s", service),
		Passed:   true,
		Reason:   "Service is active",
		Duration: duration,
	}
}

// CheckWithReport performs health checks and returns a formatted report
func (c *Checker) CheckWithReport() (string, error) {
	result, err := c.Check()
	if err != nil {
		return "", err
	}
	
	report := "=== Health Check Report ===\n"
	report += fmt.Sprintf("Overall Status: %s\n", map[bool]string{true: "PASS", false: "FAIL"}[result.Overall])
	report += "\n"
	
	for _, check := range result.Checks {
		status := "✓"
		if !check.Passed {
			status = "✗"
		}
		report += fmt.Sprintf("%s %s: %s (%v)\n", status, check.Name, check.Reason, check.Duration)
	}
	
	return report, nil
}