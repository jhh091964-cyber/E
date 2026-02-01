package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"mailops/internal/deploy/profiles"
	"mailops/internal/dns/cloudflare"
	"mailops/internal/protocol"
	"mailops/internal/ssh"
	"mailops/internal/security"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Task represents a deployment task
type Task struct {
	RowID       int
	Server      ServerConfig
	State       protocol.TaskState
	Attempt     int
	StartTime   time.Time
	EndTime     time.Time
	CurrentStep string
	Error       *TaskError
	Ctx         context.Context
	Cancel      context.CancelFunc
	Report      *TaskReport
}

// ServerConfig represents server configuration
type ServerConfig struct {
	RowID          int
	CFAPIToken     string
	CFZone         string
	ServerIP       string
	ServerPort     int
	ServerUser     string
	ServerPassword string
	ServerKeyPath  string
	Host           string
	Domain         string
	DeployProfile  string
	EmailUse       string
	Solution       string
}

// TaskError represents a task error
type TaskError struct {
	Code    protocol.ErrorCode
	Message string
}

// TaskReport represents deployment report
type TaskReport struct {
	RowID         int               `json:"row_id"`
	Domain        string            `json:"domain"`
	ServerIP      string            `json:"server_ip"`
	ServerPort    int               `json:"server_port"`
	DeployProfile string            `json:"deploy_profile"`
	Status        string            `json:"status"`
	StartTime     string            `json:"start_time"`
	EndTime       string            `json:"end_time"`
	DurationMs    int64             `json:"duration_ms"`
	Error         string            `json:"error,omitempty"`
	Steps         []StepResult      `json:"steps"`
	DNSChanges    []DNSChange       `json:"dns_changes,omitempty"`
	HealthCheck   HealthCheckResult `json:"health_check"`
}

// StepResult represents step execution result
type StepResult struct {
	Step     string `json:"step"`
	Success  bool   `json:"success"`
	Duration int64  `json:"duration_ms"`
	Message  string `json:"message"`
}

// DNSChange represents DNS change
type DNSChange struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Action  string `json:"action"` // "create" or "update"
}

// HealthCheckResult represents health check results
type HealthCheckResult struct {
	Ports    map[string]bool   `json:"ports"`
	Services map[string]string `json:"services"`
}

// Scheduler manages task execution
type Scheduler struct {
	tasks        map[int]*Task
	taskQueue    chan *Task
	workers      int
	retryMax     int
	retryBackoff time.Duration
	encoder      *protocol.Encoder
	mu           sync.RWMutex
	cancelChan   chan struct{}
	running      bool
	logger       Logger
	appConfig    *Config
	dnsDryRun    bool
	runID        string
	masker       *security.Masker
}

// Config represents app config
type Config struct {
	SSHTimeoutMs   int
	CmdTimeoutMs   int
	DKIMSelector   string
	SPFTemplate    string
	DMARCTemplate  string
}

// Logger interface for task logging
type Logger interface {
	Log(runID string, rowID int, level protocol.LogLevel, msg string)
}

// NewScheduler creates a new scheduler
func NewScheduler(workers, retryMax int, retryBackoff time.Duration, encoder *protocol.Encoder, logger Logger, appConfig *Config, dnsDryRun bool, runID string, masker *security.Masker) *Scheduler {
	return &Scheduler{
		tasks:        make(map[int]*Task),
		taskQueue:    make(chan *Task, 1000),
		workers:      workers,
		retryMax:     retryMax,
		retryBackoff: retryBackoff,
		encoder:      encoder,
		cancelChan:   make(chan struct{}),
		logger:       logger,
		appConfig:    appConfig,
		dnsDryRun:    dnsDryRun,
		runID:        runID,
		masker:       masker,
	}
}

// AddTask adds a task to the scheduler
func (s *Scheduler) AddTask(task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	ctx, cancel := context.WithCancel(context.Background())
	task.Ctx = ctx
	task.Cancel = cancel
	task.State = protocol.Pending
	task.Report = &TaskReport{
		RowID:         task.RowID,
		Domain:        task.Server.Domain,
		ServerIP:      task.Server.ServerIP,
		ServerPort:    task.Server.ServerPort,
		DeployProfile: task.Server.DeployProfile,
		Status:        "PENDING",
		StartTime:     time.Now().Format(time.RFC3339),
		Steps:         make([]StepResult, 0),
		DNSChanges:    make([]DNSChange, 0),
		HealthCheck: HealthCheckResult{
			Ports:    make(map[string]bool),
			Services: make(map[string]string),
		},
	}
	
	s.tasks[task.RowID] = task
	s.taskQueue <- task
}

// GetTask returns a task by row ID
func (s *Scheduler) GetTask(rowID int) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[rowID]
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.mu.Unlock()

	// Start worker pool
	for i := 0; i < s.workers; i++ {
		go s.worker(i)
	}

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	s.running = false
	close(s.cancelChan)
	s.mu.Unlock()
}

// CancelRun cancels all tasks in the current run
func (s *Scheduler) CancelRun() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, task := range s.tasks {
		if task.State == protocol.Pending || task.State == protocol.Running {
			task.Cancel()
		}
	}
}

// CancelTask cancels a specific task by row ID (string)
func (s *Scheduler) CancelTask(rowIDStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	rowID, err := strconv.Atoi(rowIDStr)
	if err != nil {
		return fmt.Errorf("invalid row ID: %s", rowIDStr)
	}
	
	task, ok := s.tasks[rowID]
	if !ok {
		return fmt.Errorf("task not found: %d", rowID)
	}
	
	task.Cancel()
	return nil
}

func (s *Scheduler) worker(workerID int) {
	for {
		select {
		case task := <-s.taskQueue:
			s.processTask(task, workerID)
		case <-s.cancelChan:
			return
		}
	}
}

// processTask processes a single task
func (s *Scheduler) processTask(task *Task, workerID int) {
	task.StartTime = time.Now()
	
	// Update state to validating
	s.UpdateTaskState(task.RowID, protocol.Validating, task.Attempt)
	
	// Validate input
	if err := s.validateInput(task); err != nil {
		s.handleTaskError(task, err)
		return
	}
	
	// Update state to running
	s.UpdateTaskState(task.RowID, protocol.Running, task.Attempt)
	
	// Execute task steps
	steps := []string{
		"validate_input",
		"ssh_connect_test",
		"server_prepare",
		"deploy_mailstack",
		"generate_dkim",
		"dns_apply",
		"healthcheck",
		"finalize_report",
	}
	
	for _, step := range steps {
		select {
		case <-task.Ctx.Done():
			s.handleTaskCancelled(task)
			return
		default:
			if err := s.executeStep(task, step); err != nil {
				if protocol.IsRetryable(err.Code) && task.Attempt < s.retryMax {
					s.handleTaskRetry(task, step, err)
					return
				} else {
					s.handleTaskError(task, err)
					return
				}
			}
		}
	}
	
	// Task completed successfully
	if err := s.UpdateTaskState(task.RowID, protocol.Success, task.Attempt); err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("Failed to update state: %v", err))
	}
	
	// Write success record
	s.writeSuccessRecord(task)
	
	// Write task report
	s.writeTaskReport(task)
}

// validateInput validates task input
func (s *Scheduler) validateInput(task *Task) *TaskError {
	if task.Server.CFAPIToken == "" {
		return &TaskError{Code: protocol.MissingRequiredField, Message: "CF API token is required"}
	}
	if task.Server.CFZone == "" {
		return &TaskError{Code: protocol.MissingRequiredField, Message: "CF zone is required"}
	}
	if task.Server.ServerIP == "" {
		return &TaskError{Code: protocol.MissingRequiredField, Message: "Server IP is required"}
	}
	if task.Server.Domain == "" {
		return &TaskError{Code: protocol.MissingRequiredField, Message: "Domain is required"}
	}
	return nil
}

// executeStep executes a single task step
func (s *Scheduler) executeStep(task *Task, step string) *TaskError {
	task.CurrentStep = step
	
	// Emit step start event with envelope
	startEvent := protocol.NewTaskStepStartEvent(task.RowID, string(step), "Starting "+string(step))
	rowIDStr := strconv.Itoa(task.RowID)
	s.encoder.Encode(protocol.TaskStep, s.runID, rowIDStr, startEvent)
	
	startTime := time.Now()
	
	// Execute step logic
	err := s.executeStepLogic(task, step)
	
	duration := time.Since(startTime).Milliseconds()
	
	// Record step result
	stepResult := StepResult{
		Step:     step,
		Success:  err == nil,
		Duration: duration,
		Message:  fmt.Sprintf("Step %s completed", step),
	}
	if err != nil {
		stepResult.Message = fmt.Sprintf("Step %s failed: %s", step, err.Message)
	}
	task.Report.Steps = append(task.Report.Steps, stepResult)
	
	// Emit step end event with envelope
	endEvent := protocol.NewTaskStepEndEvent(task.RowID, string(step), "Completed "+string(step), err == nil)
	if duration > 0 {
		endEvent.Duration = duration
	}
	if encodeErr := s.encoder.Encode(protocol.TaskStep, s.runID, rowIDStr, endEvent); encodeErr != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Failed to encode step event: %v", encodeErr))
	}
	
	return err
}

// executeStepLogic executes the actual step logic
func (s *Scheduler) executeStepLogic(task *Task, step string) *TaskError {
	switch step {
	case "validate_input":
		return s.stepValidateInput(task)
	case "ssh_connect_test":
		return s.stepSSHConnectTest(task)
	case "server_prepare":
		return s.stepServerPrepare(task)
	case "deploy_mailstack":
		return s.stepDeployMailstack(task)
	case "generate_dkim":
		return s.stepGenerateDKIM(task)
	case "dns_apply":
		return s.stepDNSApply(task)
	case "healthcheck":
		return s.stepHealthcheck(task)
	case "finalize_report":
		return s.stepFinalizeReport(task)
	default:
		return &TaskError{Code: protocol.InvalidConfig, Message: fmt.Sprintf("Unknown step: %s", step)}
	}
}

// stepValidateInput validates input configuration
func (s *Scheduler) stepValidateInput(task *Task) *TaskError {
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Validating input configuration...")
	
	// Check required fields
	if task.Server.CFAPIToken == "" {
		return &TaskError{Code: protocol.MissingRequiredField, Message: "CF API token is required"}
	}
	if task.Server.CFZone == "" {
		return &TaskError{Code: protocol.MissingRequiredField, Message: "CF zone is required"}
	}
	if task.Server.ServerIP == "" {
		return &TaskError{Code: protocol.MissingRequiredField, Message: "Server IP is required"}
	}
	if task.Server.Domain == "" {
		return &TaskError{Code: protocol.MissingRequiredField, Message: "Domain is required"}
	}
	if task.Server.ServerPort == 0 {
		return &TaskError{Code: protocol.InvalidConfig, Message: "Server port must be specified"}
	}
	if task.Server.ServerUser == "" {
		return &TaskError{Code: protocol.InvalidConfig, Message: "Server user must be specified"}
	}
	
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Input validation passed")
	return nil
}

// stepSSHConnectTest tests SSH connection to the server
func (s *Scheduler) stepSSHConnectTest(task *Task) *TaskError {
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Testing SSH connection...")
	
	// Create SSH client config
	sshConfig := ssh.Config{
		Host:     task.Server.ServerIP,
		Port:     task.Server.ServerPort,
		User:     task.Server.ServerUser,
		Password: task.Server.ServerPassword,
		KeyPath:  task.Server.ServerKeyPath,
		Timeout:  time.Duration(s.appConfig.SSHTimeoutMs) * time.Millisecond,
	}
	
	// Create SSH client
	client, err := ssh.NewClient(sshConfig)
	if err != nil {
		return &TaskError{Code: protocol.SSHConn, Message: fmt.Sprintf("Failed to create SSH client: %v", err)}
	}
	defer client.Close()
	
	// Test connection
	if err := client.TestConnection(); err != nil {
		return &TaskError{Code: protocol.SSHConn, Message: fmt.Sprintf("SSH connection test failed: %v", err)}
	}
	
	s.logger.Log(s.runID, task.RowID, protocol.Info, "SSH connection successful")
	return nil
}

// stepServerPrepare prepares the server for deployment
func (s *Scheduler) stepServerPrepare(task *Task) *TaskError {
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Preparing server...")
	
	// Create SSH client
	sshConfig := ssh.Config{
		Host:     task.Server.ServerIP,
		Port:     task.Server.ServerPort,
		User:     task.Server.ServerUser,
		Password: task.Server.ServerPassword,
		KeyPath:  task.Server.ServerKeyPath,
		Timeout:  time.Duration(s.appConfig.CmdTimeoutMs) * time.Millisecond,
	}
	
	client, err := ssh.NewClient(sshConfig)
	if err != nil {
		return &TaskError{Code: protocol.SSHConn, Message: fmt.Sprintf("Failed to create SSH client: %v", err)}
	}
	defer client.Close()
	
	// Update package lists
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Updating package lists...")
	output, err := client.ExecuteCommandWithOutput("apt-get update", 120*time.Second)
	if err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("apt-get update failed: %v, output: %s", err, output))
		return &TaskError{Code: protocol.DeployFailed, Message: fmt.Sprintf("Failed to update package lists: %v", err)}
	}
	
	// Install common dependencies
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Installing common dependencies...")
	dependencies := []string{
		"apt-transport-https",
		"ca-certificates",
		"curl",
		"gnupg",
		"lsb-release",
		"net-tools",
	}
	
	for _, pkg := range dependencies {
		err := client.InstallPackage(pkg)
		if err != nil {
			s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Failed to install %s: %v", pkg, err))
		}
	}
	
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Server preparation completed")
	return nil
}

// stepDeployMailstack deploys the mail server stack
func (s *Scheduler) stepDeployMailstack(task *Task) *TaskError {
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Deploying mail server stack...")
	
	// Create SSH client
	sshConfig := ssh.Config{
		Host:     task.Server.ServerIP,
		Port:     task.Server.ServerPort,
		User:     task.Server.ServerUser,
		Password: task.Server.ServerPassword,
		KeyPath:  task.Server.ServerKeyPath,
		Timeout:  time.Duration(s.appConfig.CmdTimeoutMs) * time.Millisecond,
	}
	
	client, err := ssh.NewClient(sshConfig)
	if err != nil {
		return &TaskError{Code: protocol.SSHConn, Message: fmt.Sprintf("Failed to create SSH client: %v", err)}
	}
	defer client.Close()
	
	var deployResult *profiles.DeployResult
	
	switch task.Server.DeployProfile {
	case "postfix_dovecot":
		s.logger.Log(s.runID, task.RowID, protocol.Info, "Deploying Postfix + Dovecot profile...")
		profile := &profiles.PostfixDovecotProfile{
			Domain:       task.Server.Domain,
			Hostname:     task.Server.Host,
			DKIMSelector: s.appConfig.DKIMSelector,
			DKIMKeySize:  2048,
		}
		deployResult, err = profile.Deploy(client)
		if err != nil {
			return &TaskError{Code: protocol.DeployFailed, Message: fmt.Sprintf("Deployment failed: %v", err)}
		}
		
	case "docker_mailserver":
		s.logger.Log(s.runID, task.RowID, protocol.Info, "Deploying Docker MailServer profile...")
		profile := &profiles.DockerMailserverProfile{
			Domain:        task.Server.Domain,
			Hostname:      task.Server.Host,
			ContainerName: fmt.Sprintf("mailserver-%d", task.RowID),
		}
		deployResult, err = profile.Deploy(client)
			DKIMSelector:  s.appConfig.DKIMSelector,
		if err != nil {
			return &TaskError{Code: protocol.DeployFailed, Message: fmt.Sprintf("Deployment failed: %v", err)}
		}
		
	default:
		return &TaskError{Code: protocol.InvalidConfig, Message: fmt.Sprintf("Unknown deploy profile: %s", task.Server.DeployProfile)}
	}
	
	s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("Deployment completed: %s", deployResult.Version))
	return nil
}

// stepGenerateDKIM generates DKIM keys
func (s *Scheduler) stepGenerateDKIM(task *Task) *TaskError {
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Generating DKIM keys...")
	
	// Create SSH client
	sshConfig := ssh.Config{
		Host:     task.Server.ServerIP,
		Port:     task.Server.ServerPort,
		User:     task.Server.ServerUser,
		Password: task.Server.ServerPassword,
		KeyPath:  task.Server.ServerKeyPath,
		Timeout:  time.Duration(s.appConfig.CmdTimeoutMs) * time.Millisecond,
	}
	
	client, err := ssh.NewClient(sshConfig)
	if err != nil {
		return &TaskError{Code: protocol.SSHConn, Message: fmt.Sprintf("Failed to create SSH client: %v", err)}
	}
	defer client.Close()
	
	var dkimPublicKey string
	dkimSelector := s.appConfig.DKIMSelector
	if dkimSelector == "" {
		dkimSelector = "s1"  // Default selector
	}
	
	// Different DKIM generation based on deploy profile
	if task.Server.DeployProfile == "docker_mailserver" {
		s.logger.Log(s.runID, task.RowID, protocol.Info, "Using docker-mailserver DKIM generation...")
		
		// Use docker-mailserver's profile method
		profile := &profiles.DockerMailserverProfile{
			Domain:        task.Server.Domain,
			Hostname:      task.Server.Host,
			ContainerName: fmt.Sprintf("mailserver-%d", task.RowID),
			DKIMSelector:  dkimSelector,
		}
		
		dkimPublicKey, err = profile.GenerateDKIM(client)
		if err != nil {
			return &TaskError{Code: protocol.DeployFailed, Message: fmt.Sprintf("Failed to generate DKIM with docker-mailserver: %v", err)}
		}
	} else {
		// Use traditional opendkim-tools method
		s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("Generating %d-bit DKIM key for %s...", 2048, task.Server.Domain))
		
		// Install opendkim-tools if needed
		err = client.InstallPackage("opendkim-tools")
		if err != nil {
			s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Failed to install opendkim-tools: %v", err))
		}
		
		// Create DKIM directory
		mkdirCmd := fmt.Sprintf("mkdir -p /etc/opendkim/keys/%s", task.Server.Domain)
		_, err = client.ExecuteCommandWithOutput(mkdirCmd, 30*time.Second)
		if err != nil {
			return &TaskError{Code: protocol.DeployFailed, Message: fmt.Sprintf("Failed to create DKIM directory: %v", err)}
		}
		
		// Generate DKIM key
		genKeyCmd := fmt.Sprintf("opendkim-genkey -b 2048 -r -s %s -d %s -D /etc/opendkim/keys/%s", 
			dkimSelector, task.Server.Domain, task.Server.Domain)
		output, err := client.ExecuteCommandWithOutput(genKeyCmd, 60*time.Second)
		if err != nil {
			return &TaskError{Code: protocol.DeployFailed, Message: fmt.Sprintf("Failed to generate DKIM key: %v, output: %s", err, output)}
		}
		
		// Read DKIM public key
		dkimKeyPath := fmt.Sprintf("/etc/opendkim/keys/%s/%s.txt", task.Server.Domain, dkimSelector)
		dkimPublicKey, err = client.ExecuteCommandWithOutput(fmt.Sprintf("cat %s", dkimKeyPath), 30*time.Second)
		if err != nil {
			s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Failed to read DKIM public key: %v", err))
			dkimPublicKey = ""
		}
		
		// Normalize DKIM key
		dkimPublicKey = normalizeDKIMKey(dkimPublicKey)
	}
	
	// Store DKIM public key in task report for DNS step
	if task.Report != nil && dkimPublicKey != "" {
		// Store in report's DNS changes temporarily
		task.Report.DNSChanges = append(task.Report.DNSChanges, DNSChange{
			Type:    "TXT",
			Name:    fmt.Sprintf("%s._domainkey", dkimSelector),
			Content: dkimPublicKey,
			Action:  "pending",
		})
	}
	
	s.logger.Log(s.runID, task.RowID, protocol.Info, "DKIM keys generated successfully")
	return nil
}


// stepDNSApply applies DNS records to Cloudflare
func (s *Scheduler) stepDNSApply(task *Task) *TaskError {
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Applying DNS records to Cloudflare...")
	
	// Get DKIM public key from task report (generated in stepGenerateDKIM)
	dkimSelector := s.appConfig.DKIMSelector
	if dkimSelector == "" {
		dkimSelector = "s1"  // Default selector
	}
	
	// Look for DKIM key in DNS changes from previous step
	dkimPublicKey := ""
	if task.Report != nil {
		for _, change := range task.Report.DNSChanges {
			if change.Type == "TXT" && change.Name == fmt.Sprintf("%s._domainkey", dkimSelector) {
				dkimPublicKey = change.Content
				break
			}
		}
	}
	
	// If not found in report, try to read from server (fallback for non-docker-mailserver)
	if dkimPublicKey == "" {
		sshConfig := ssh.Config{
			Host:     task.Server.ServerIP,
			Port:     task.Server.ServerPort,
			User:     task.Server.ServerUser,
			Password: task.Server.ServerPassword,
			KeyPath:  task.Server.ServerKeyPath,
			Timeout:  time.Duration(s.appConfig.SSHTimeoutMs) * time.Millisecond,
		}
		
		client, err := ssh.NewClient(sshConfig)
		if err == nil {
			defer client.Close()
			
			var dkimKeyPath string
			if task.Server.DeployProfile == "docker_mailserver" {
				dkimKeyPath = fmt.Sprintf("/opt/mailserver/config/opendkim/%s.txt", dkimSelector)
			} else {
				dkimKeyPath = fmt.Sprintf("/etc/opendkim/keys/%s/%s.txt", task.Server.Domain, dkimSelector)
			}
			
			dkimPublicKey, err = client.ExecuteCommandWithOutput(fmt.Sprintf("cat %s", dkimKeyPath), 30*time.Second)
			if err != nil {
				s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Failed to read DKIM public key: %v", err))
				dkimPublicKey = ""
			} else {
				dkimPublicKey = normalizeDKIMKey(dkimPublicKey)
			}
		}
	}
	
	// Create DNS provider
	dnsProvider := cloudflare.NewProvider(task.Server.CFAPIToken, s.dnsDryRun)
	
	// Render templates
	variables := map[string]string{
		"server_ip": task.Server.ServerIP,
		"domain":    task.Server.Domain,
		"host":      task.Server.Host,
	}
	
	spfRecord := renderTemplate(s.appConfig.SPFTemplate, variables)
	if spfRecord == "" {
		spfRecord = "v=spf1 mx -all"
	}
	
	dmarcRecord := renderTemplate(s.appConfig.DMARCTemplate, variables)
	if dmarcRecord == "" {
		dmarcRecord = fmt.Sprintf("v=DMARC1; p=none; rua=mailto:dmarc@%s", task.Server.CFZone)
	}
	
	// Apply DNS records with dry-run support
	if s.dnsDryRun {
		s.logger.Log(s.runID, task.RowID, protocol.Info, "[DRY-RUN] DNS changes that would be applied:")
		s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("  A: %s -> %s", task.Server.Host, task.Server.ServerIP))
		s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("  MX: %s -> %s (priority 10)", task.Server.Domain, task.Server.Host))
		s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("  TXT (@): %s", spfRecord))
		s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("  TXT (_dmarc): %s", dmarcRecord))
		if dkimPublicKey != "" {
			s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("  TXT (%s._domainkey): %s", dkimSelector, dkimPublicKey))
		}
	} else {
		// Create A record (upsert)
		s.logger.Log(s.runID, task.RowID, protocol.Info, "Creating/updating A record...")
		err := dnsProvider.UpsertRecord("A", task.Server.Host, task.Server.ServerIP, nil)
		if err != nil {
			return &TaskError{Code: protocol.DNSAuthFailed, Message: fmt.Sprintf("Failed to create A record: %v", err)}
		}
		task.Report.DNSChanges = append(task.Report.DNSChanges, DNSChange{
			Type:    "A",
			Name:    task.Server.Host,
			Content: task.Server.ServerIP,
			Action:  "create",
		})
		
		// Create MX record (upsert)
		s.logger.Log(s.runID, task.RowID, protocol.Info, "Creating/updating MX record...")
		priority := 10
		err = dnsProvider.UpsertRecord("MX", task.Server.Domain, task.Server.Host, &priority)
		if err != nil {
			return &TaskError{Code: protocol.DNSAuthFailed, Message: fmt.Sprintf("Failed to create MX record: %v", err)}
		}
		task.Report.DNSChanges = append(task.Report.DNSChanges, DNSChange{
			Type:    "MX",
			Name:    task.Server.Domain,
			Content: fmt.Sprintf("%s (priority %d)", task.Server.Host, priority),
			Action:  "create",
		})
		
		// Create SPF TXT record (upsert)
		s.logger.Log(s.runID, task.RowID, protocol.Info, "Creating/updating SPF TXT record...")
		err = dnsProvider.UpsertRecord("TXT", "@", spfRecord, nil)
		if err != nil {
			s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Failed to create SPF record: %v", err))
		}
		task.Report.DNSChanges = append(task.Report.DNSChanges, DNSChange{
			Type:    "TXT",
			Name:    "@",
			Content: spfRecord,
			Action:  "create",
		})
		
		// Create DMARC TXT record (upsert)
		s.logger.Log(s.runID, task.RowID, protocol.Info, "Creating/updating DMARC TXT record...")
		err = dnsProvider.UpsertRecord("TXT", "_dmarc", dmarcRecord, nil)
		if err != nil {
			s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Failed to create DMARC record: %v", err))
		}
		task.Report.DNSChanges = append(task.Report.DNSChanges, DNSChange{
			Type:    "TXT",
			Name:    "_dmarc",
			Content: dmarcRecord,
			Action:  "create",
		})
		
		// Create DKIM TXT record (upsert)
		if dkimPublicKey != "" {
			s.logger.Log(s.runID, task.RowID, protocol.Info, "Creating/updating DKIM TXT record...")
			dkimRecordName := fmt.Sprintf("%s._domainkey", dkimSelector)
			err = dnsProvider.UpsertRecord("TXT", dkimRecordName, dkimPublicKey, nil)
			if err != nil {
				s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Failed to create DKIM record: %v", err))
			}
			task.Report.DNSChanges = append(task.Report.DNSChanges, DNSChange{
				Type:    "TXT",
				Name:    dkimRecordName,
				Content: dkimPublicKey,
				Action:  "create",
			})
		}
	}
	
	s.logger.Log(s.runID, task.RowID, protocol.Info, "DNS records applied successfully")
	return nil
}


// stepHealthcheck performs health checks on the deployed mail server
func (s *Scheduler) stepHealthcheck(task *Task) *TaskError {
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Performing health checks...")
	
	// Create SSH client
	sshConfig := ssh.Config{
		Host:     task.Server.ServerIP,
		Port:     task.Server.ServerPort,
		User:     task.Server.ServerUser,
		Password: task.Server.ServerPassword,
		KeyPath:  task.Server.ServerKeyPath,
		Timeout:  time.Duration(s.appConfig.CmdTimeoutMs) * time.Millisecond,
	}
	
	client, err := ssh.NewClient(sshConfig)
	if err != nil {
		return &TaskError{Code: protocol.SSHConn, Message: fmt.Sprintf("Failed to create SSH client: %v", err)}
	}
	defer client.Close()
	
	// Check ports
	ports := []int{25, 587, 465, 143, 993}
	s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("Checking ports: %v", ports))
	
	for _, port := range ports {
		open := client.CheckPort(port, 10*time.Second)
		task.Report.HealthCheck.Ports[strconv.Itoa(port)] = open
		if open {
			s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("Port %d: OPEN", port))
		} else {
			s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Port %d: CLOSED or not responding", port))
		}
	}
	
	// Check service status
	services := []string{"postfix", "dovecot"}
	for _, svc := range services {
		cmd := fmt.Sprintf("systemctl is-active %s", svc)
		output, err := client.ExecuteCommandWithOutput(cmd, 10*time.Second)
		status := "inactive"
		if err == nil {
			status = strings.TrimSpace(output)
		}
		task.Report.HealthCheck.Services[svc] = status
		s.logger.Log(s.runID, task.RowID, protocol.Info, fmt.Sprintf("Service %s: %s", svc, status))
	}
	
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Health checks completed")
	return nil
}

// stepFinalizeReport finalizes the deployment report
func (s *Scheduler) stepFinalizeReport(task *Task) *TaskError {
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Finalizing deployment report...")
	
	task.Report.EndTime = time.Now().Format(time.RFC3339)
	task.Report.DurationMs = time.Since(task.StartTime).Milliseconds()
	task.Report.Status = "SUCCESS"
	
	report := fmt.Sprintf(`
========================================
MailOps Deployment Report
========================================

Task ID: %d
Domain: %s
Server: %s:%d
Profile: %s
Status: SUCCESS

========================================
Deployment Details
========================================

1. Server: %s
2. Domain: %s
3. Hostname: %s
4. Deploy Profile: %s
5. Email Use: %s
6. Solution: %s

========================================
Deployment Steps
========================================

✓ Input Validation
✓ SSH Connection Test
✓ Server Preparation
✓ Mail Stack Deployment
✓ DKIM Key Generation
✓ DNS Configuration
✓ Health Checks
✓ Report Finalization

========================================
Access Information
========================================

SMTP: mail.%s:%d
IMAP: mail.%s:%d
Webmail: http://mail.%s

========================================
Time: %s
========================================
`,
		task.RowID,
		task.Server.Domain,
		task.Server.ServerIP,
		task.Server.ServerPort,
		task.Server.DeployProfile,
		task.Server.ServerIP,
		task.Server.Domain,
		task.Server.Host,
		task.Server.DeployProfile,
		task.Server.EmailUse,
		task.Server.Solution,
		task.Server.Domain, 25,
		task.Server.Domain, 143,
		task.Server.Host,
		time.Now().Format("2006-01-02 15:04:05"),
	)
	
	s.logger.Log(s.runID, task.RowID, protocol.Info, "Deployment report completed")
	s.logger.Log(s.runID, task.RowID, protocol.Info, report)
	
	return nil
}

// UpdateTaskState updates task state and emits event
func (s *Scheduler) UpdateTaskState(rowID int, state protocol.TaskState, attempt int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	task, ok := s.tasks[rowID]
	if !ok {
		return fmt.Errorf("task not found: %d", rowID)
	}
	
	task.State = state
	task.Attempt = attempt
	
	if state == protocol.Success || state == protocol.Failed || state == protocol.Cancelled {
		task.EndTime = time.Now()
		if task.Report != nil {
			task.Report.EndTime = task.EndTime.Format(time.RFC3339)
			task.Report.DurationMs = task.EndTime.Sub(task.StartTime).Milliseconds()
			task.Report.Status = string(state)
		}
	}
	
	event := protocol.NewTaskStateEventWithRetry(rowID, state, string(state), attempt)
	rowIDStr := strconv.Itoa(rowID)
	return s.encoder.Encode(protocol.TaskStateEvt, s.runID, rowIDStr, event)
}

// handleTaskError handles task error
func (s *Scheduler) handleTaskError(task *Task, taskErr *TaskError) {
	task.Error = taskErr
	task.State = protocol.Failed
	
	if err := s.UpdateTaskState(task.RowID, protocol.Failed, task.Attempt); err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("Failed to update state: %v", err))
	}
	
	// Write failed record
	s.writeFailedRecord(task, taskErr)
	
	// Write task report
	s.writeTaskReport(task)
	
	errorEvent := protocol.NewErrorEvent(taskErr.Code, taskErr.Message, task.RowID)
	rowIDStr := strconv.Itoa(task.RowID)
	if err := s.encoder.Encode(protocol.ErrorEvt, s.runID, rowIDStr, errorEvent); err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("Failed to encode error event: %v", err))
	}
	
	s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("[%s] %s", taskErr.Code, taskErr.Message))
}

// handleTaskRetry handles task retry
func (s *Scheduler) handleTaskRetry(task *Task, step string, taskErr *TaskError) {
	task.Error = taskErr
	task.State = protocol.Retrying
	
	if err := s.UpdateTaskState(task.RowID, protocol.Retrying, task.Attempt); err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("Failed to update state: %v", err))
	}
	
	// Calculate backoff delay
	backoff := s.retryBackoff * time.Duration(1<<uint(task.Attempt-1))
	jitter := time.Duration(100) * time.Millisecond
	delay := backoff + jitter
	
	s.logger.Log(s.runID, task.RowID, protocol.Warn, fmt.Sprintf("Retry %d/%d in %v: [%s] %s", task.Attempt+1, s.retryMax, delay, taskErr.Code, taskErr.Message))
	
	// Schedule retry
	go func() {
		time.Sleep(delay)
		
		// Increment attempt
		task.Attempt++
		
		// Re-queue task
		s.taskQueue <- task
	}()
}

// handleTaskCancelled handles task cancellation
func (s *Scheduler) handleTaskCancelled(task *Task) {
	task.State = protocol.Cancelled
	task.EndTime = time.Now()
	
	if err := s.UpdateTaskState(task.RowID, protocol.Cancelled, task.Attempt); err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("Failed to update state: %v", err))
	}
	
	// Write task report
	s.writeTaskReport(task)
	
	errorEvent := protocol.NewErrorEvent(protocol.AuthFailed, "Task cancelled by user", task.RowID)
	rowIDStr := strconv.Itoa(task.RowID)
	if err := s.encoder.Encode(protocol.ErrorEvt, s.runID, rowIDStr, errorEvent); err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("Failed to encode error event: %v", err))
	}
}

// GetProgress returns current progress statistics
func (s *Scheduler) GetProgress() (done, success, failed, cancelled, running, pending int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	for _, task := range s.tasks {
		switch task.State {
		case protocol.Success:
			done++
			success++
		case protocol.Failed:
			done++
			failed++
		case protocol.Cancelled:
			done++
			cancelled++
		case protocol.Running:
			running++
		case protocol.Pending, protocol.Validating, protocol.Retrying:
			pending++
		}
	}
	
	return
}

// writeSuccessRecord writes a success record
func (s *Scheduler) writeSuccessRecord(task *Task) {
	filePath := filepath.Join("output/results", "success.txt")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	
	file.WriteString(fmt.Sprintf("%d,%s,%s\n", task.RowID, task.Server.Domain, task.Server.ServerIP))
}

// writeFailedRecord writes a failed record
func (s *Scheduler) writeFailedRecord(task *Task, taskErr *TaskError) {
	filePath := filepath.Join("output/results", "failed.txt")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	
	// Format: row_id,error_code,short_reason (masked)
	file.WriteString(fmt.Sprintf("%d,%s,%s\n", task.RowID, taskErr.Code, s.masker.MaskInString(taskErr.Message)))
}

// writeTaskReport writes a task report to JSON file
func (s *Scheduler) writeTaskReport(task *Task) {
	if task.Report == nil {
		return
	}
	
	reportDir := filepath.Join("output/reports", s.runID)
	os.MkdirAll(reportDir, 0755)
	
	reportPath := filepath.Join(reportDir, fmt.Sprintf("%d.json", task.RowID))
	
	data, err := json.MarshalIndent(task.Report, "", "  ")
	if err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("Failed to marshal report: %v", err))
		return
	}
	
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		s.logger.Log(s.runID, task.RowID, protocol.Error, fmt.Sprintf("Failed to write report: %v", err))
	}
}

// normalizeDKIMKey normalizes DKIM key by extracting p= value
func normalizeDKIMKey(dkimContent string) string {
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

// renderTemplate renders a template with given variables
func renderTemplate(template string, variables map[string]string) string {
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("{%s}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}