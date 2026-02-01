package protocol

import (
	"fmt"
	"math/rand"
	"time"
)

// Event types
type EventType string

const (
	RunStarted  EventType = "RUN_STARTED"
	RunProgress EventType = "RUN_PROGRESS"
	TaskStateEvt EventType = "TASK_STATE"
	TaskStep    EventType = "TASK_STEP"
	LogLine     EventType = "LOG_LINE"
	ErrorEvt    EventType = "ERROR"
	RunFinished EventType = "RUN_FINISHED"
)

// Command types
type CommandType string

const (
	StartRun   CommandType = "START_RUN"
	CancelRun  CommandType = "CANCEL_RUN"
	CancelTask CommandType = "CANCEL_TASK"
	Ping       CommandType = "PING"
)

// Error codes
type ErrorCode string

const (
	MissingRequiredField ErrorCode = "MISSING_REQUIRED_FIELD"
	RemoteCmdTransient   ErrorCode = "REMOTE_CMD_TRANSIENT"
	SSHConn              ErrorCode = "SSH_CONN"
	SSHTimeout           ErrorCode = "SSH_TIMEOUT"
	InvalidConfig        ErrorCode = "INVALID_CONFIG"
	AuthFailed           ErrorCode = "AUTH_FAILED"
	DeployFailed         ErrorCode = "DEPLOY_FAILED"
	DNSRateLimit         ErrorCode = "DNS_RATE_LIMIT"
	DNSAuthFailed        ErrorCode = "DNS_AUTH_FAILED"
)

// Task states
type TaskState string

const (
	Pending    TaskState = "PENDING"
	Validating TaskState = "VALIDATING"
	Running    TaskState = "RUNNING"
	Retrying   TaskState = "RETRYING"
	Success    TaskState = "SUCCESS"
	Failed     TaskState = "FAILED"
	Cancelled  TaskState = "CANCELLED"
)

// Log levels
type LogLevel string

const (
	Debug LogLevel = "DEBUG"
	Info  LogLevel = "INFO"
	Warn  LogLevel = "WARN"
	Error LogLevel = "ERROR"
)

// Step phases
type StepPhase string

const (
	StepStart StepPhase = "START"
	StepEnd   StepPhase = "END"
)

// Deployment steps
const (
	ValidateInput   = "validate_input"
	SSHConnectTest = "ssh_connect_test"
	ServerPrepare  = "server_prepare"
	DeployMailstack = "deploy_mailstack"
	GenerateDKIM   = "generate_dkim"
	DNSApply       = "dns_apply"
	HealthCheck    = "healthcheck"
	FinalizeReport = "finalize_report"
)

// Retryable error codes
var retryableErrors = map[ErrorCode]bool{
	RemoteCmdTransient: true,
	SSHConn:           true,
	SSHTimeout:        true,
	DeployFailed:      true,
	DNSRateLimit:      true,
}

// IsRetryable checks if an error code is retryable
func IsRetryable(code ErrorCode) bool {
	return retryableErrors[code]
}

// Envelope is the wrapper for all NDJSON events
type Envelope struct {
	Type   string `json:"type"`
	Ts     int64  `json:"ts"`
	RunID  string `json:"run_id"`
	RowID  string `json:"row_id,omitempty"`
	Data   any    `json:"data,omitempty"`
}

// NewEnvelope creates a new event envelope
func NewEnvelope(eventType EventType, runID, rowID string, data any) *Envelope {
	return &Envelope{
		Type:  string(eventType),
		Ts:    time.Now().UnixMilli(),
		RunID: runID,
		RowID: rowID,
		Data:  data,
	}
}

// Event data structures

type RunStartedEvent struct {
	RunID       string `json:"run_id"`
	TotalTasks  int    `json:"total_tasks"`
	Concurrency int    `json:"concurrency"`
	DryRun      bool   `json:"dry_run"`
}

type RunProgressEvent struct {
	RunID     string `json:"run_id"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Success   int    `json:"success"`
	Failed    int    `json:"failed"`
	Cancelled int    `json:"cancelled"`
	Running   int    `json:"running"`
	Pending   int    `json:"pending"`
}

type TaskStateEvent struct {
	RowID   int       `json:"row_id"`
	State   TaskState `json:"state"`
	Message string    `json:"message"`
	Error   string    `json:"error,omitempty"`
	Retries int       `json:"retries,omitempty"`
}

type TaskStepEvent struct {
	RowID    int       `json:"row_id"`
	Step     string    `json:"step"`
	Phase    StepPhase `json:"phase"`
	Message  string    `json:"message"`
	Success  bool      `json:"success"`
	Duration int64     `json:"duration,omitempty"`
}

type LogLineEvent struct {
	Level     LogLevel `json:"level"`
	Message   string   `json:"message"`
	Timestamp string   `json:"timestamp"`
}

type ErrorEvent struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	RowID   int       `json:"row_id,omitempty"`
}

type RunFinishedEvent struct {
	RunID       string            `json:"run_id"`
	Status      string            `json:"status"`
	TotalTasks  int               `json:"total_tasks"`
	Success     int               `json:"success"`
	Failed      int               `json:"failed"`
	Cancelled   int               `json:"cancelled"`
	Outputs     map[string]string `json:"outputs,omitempty"`
	DurationMs  int64             `json:"duration_ms"`
}

// Command structures

type StartRunCommand struct {
	Type_       string `json:"type"`
	RunID       string `json:"run_id,omitempty"`
	ConfigPath  string `json:"config_path"`
	Concurrency int    `json:"concurrency"`
	DNSDryRun   bool   `json:"dns_dry_run,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

type CancelRunCommand struct {
	Type_ string `json:"type"`
	RunID string `json:"run_id,omitempty"`
}

type CancelTaskCommand struct {
	Type_ string `json:"type"`
	RowID int    `json:"row_id"`
}

type PingCommand struct {
	Type_ string `json:"type"`
}

// Generate random correlation ID
func GenerateCorrelationID() string {
	return fmt.Sprintf("%d", rand.Intn(1000000))
}

// Generate run ID
func GenerateRunID() string {
	return fmt.Sprintf("run-%d-%s", time.Now().Unix(), GenerateCorrelationID())
}

// Get current timestamp
func GetCurrentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}