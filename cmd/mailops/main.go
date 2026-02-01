package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mailops/internal/protocol"
	"mailops/internal/scheduler"
	"mailops/internal/security"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Config represents application configuration
type Config struct {
	ConcurrencyDefault int               `json:"concurrency_default"`
	RetryMax           int               `json:"retry_max"`
	RetryBackoffMs     int               `json:"retry_backoff_ms"`
	SSHTimeoutMs       int               `json:"ssh_timeout_ms"`
	CmdTimeoutMs       int               `json:"cmd_timeout_ms"`
	DNSDryRunDefault   bool              `json:"dns_dry_run_default"`
	LogMasking         bool              `json:"log_masking"`
	DKIMSelector       string            `json:"dkim_selector"`
	SPFTemplate        string            `json:"spf_template"`
	DMARCTemplate      string            `json:"dmarc_template"`
}

var (
	eventStreamFlag = flag.Bool("event-stream", false, "Enable event stream mode for GUI")
	runOnceFlag     = flag.Bool("run-once", false, "Run once and exit")
	configPathFlag  = flag.String("config", "", "Path to CSV config file")
	concurrencyFlag = flag.Int("concurrency", 10, "Number of concurrent tasks")
	dnsDryRunFlag   = flag.Bool("dns-dry-run", false, "DNS dry-run mode")
	appConfigFlag   = flag.String("app-config", "examples/app.config.json", "Path to app config file")
)

// Global scheduler instance for cancellation
var (
	currentScheduler *scheduler.Scheduler
	currentRunID     string
	currentRunMutex  sync.Mutex
)

func main() {
	flag.Parse()
	
	// Load app config
	appConfig, err := loadAppConfig(*appConfigFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load app config: %v\n", err)
		os.Exit(1)
	}
	
	// Determine mode
	if *eventStreamFlag {
		runEventStreamMode(appConfig)
	} else if *runOnceFlag {
		runOnceMode(appConfig)
	} else {
		runEventStreamMode(appConfig)
	}
}

func loadAppConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	return &config, nil
}

func loadServerConfigs(path string) ([]scheduler.ServerConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()
	
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}
	
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have at least a header and one data row")
	}
	
	configs := make([]scheduler.ServerConfig, 0)
	
	for i, record := range records[1:] {
		if len(record) != len(records[0]) {
			return nil, fmt.Errorf("row %d has incorrect number of columns", i+2)
		}
		
		config := scheduler.ServerConfig{
			RowID:          parseInt(record[0], 0),
			CFAPIToken:     record[1],
			CFZone:         record[2],
			ServerIP:       record[3],
			ServerPort:     parseInt(record[4], 22),
			ServerUser:     record[5],
			ServerPassword: record[6],
			ServerKeyPath:  record[7],
			Host:           record[8],
			Domain:         record[9],
			DeployProfile:  record[10],
			EmailUse:       record[11],
			Solution:       record[12],
		}
		
		configs = append(configs, config)
	}
	
	return configs, nil
}

func parseInt(s string, defaultVal int) int {
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		return defaultVal
	}
	return val
}

type TaskLogger struct {
	masker   *security.Masker
	encoder  *protocol.Encoder
	runID    string
	file     *os.File
	filePath string
}

func NewTaskLogger(masker *security.Masker, encoder *protocol.Encoder) *TaskLogger {
	return &TaskLogger{
		masker:  masker,
		encoder: encoder,
	}
}

func (l *TaskLogger) Log(runID string, rowID int, level protocol.LogLevel, msg string) {
	maskedMsg := l.masker.MaskInString(msg)
	
	event := protocol.NewLogLineEvent(level, fmt.Sprintf("[%s:%d] %s", runID, rowID, maskedMsg))
	
	rowIDStr := ""
	if rowID > 0 {
		rowIDStr = strconv.Itoa(rowID)
	}
	l.encoder.Encode(protocol.LogLine, runID, rowIDStr, event)
	
	if l.file != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		l.file.WriteString(fmt.Sprintf("[%s] [%s] [%s:%d] %s\n", timestamp, level, runID, rowID, maskedMsg))
	}
}

func (l *TaskLogger) SetRunID(runID string) {
	l.runID = runID
}

func (l *TaskLogger) OpenLogFile(filePath string) error {
	var err error
	l.file, err = os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	l.filePath = filePath
	return nil
}

func (l *TaskLogger) Close() {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

func runEventStreamMode(appConfig *Config) {
	masker := security.NewMasker()
	encoder := protocol.NewEncoder(os.Stdout)
	taskLogger := NewTaskLogger(masker, encoder)
	decoder := protocol.NewDecoder(os.Stdin)
	
	taskLogger.Log("", 0, protocol.Info, "MailOps CLI started in event stream mode")
	
	for {
		cmd, err := decoder.Decode()
		if err != nil {
			if err == io.EOF {
				taskLogger.Log("", 0, protocol.Info, "Received EOF, shutting down")
				break
			}
			taskLogger.Log("", 0, protocol.Error, fmt.Sprintf("Failed to decode command: %v", err))
			continue
		}
		
		switch c := cmd.(type) {
		case *protocol.StartRunCommand:
			handleStartRun(c, appConfig, taskLogger, encoder, masker)
			
		case *protocol.CancelRunCommand:
			handleCancelRun(c, taskLogger)
			
		case *protocol.CancelTaskCommand:
			handleCancelTask(c, taskLogger)
			
		case *protocol.PingCommand:
			handlePing(taskLogger)
			
		default:
			taskLogger.Log("", 0, protocol.Warn, "Unknown command type")
		}
	}
}

func handleCancelRun(cmd *protocol.CancelRunCommand, logger *TaskLogger) {
	currentRunMutex.Lock()
	defer currentRunMutex.Unlock()
	
	if currentScheduler == nil {
		logger.Log("", 0, protocol.Warn, "No active run to cancel")
		return
	}
	
	logger.Log(currentRunID, 0, protocol.Info, "Cancelling run...")
	currentScheduler.CancelRun()
	logger.Log(currentRunID, 0, protocol.Info, "Run cancelled")
}

func handleCancelTask(cmd *protocol.CancelTaskCommand, logger *TaskLogger) {
	currentRunMutex.Lock()
	defer currentRunMutex.Unlock()
	
	if currentScheduler == nil {
		logger.Log("", 0, protocol.Warn, "No active run to cancel task")
		return
	}
	
	rowIDStr := strconv.Itoa(cmd.RowID)
	logger.Log(currentRunID, cmd.RowID, protocol.Info, "Cancelling task...")
	currentScheduler.CancelTask(rowIDStr)
	logger.Log(currentRunID, cmd.RowID, protocol.Info, "Task cancelled")
}

func handlePing(logger *TaskLogger) {
	logger.Log("", 0, protocol.Debug, "PONG")
}

func handleStartRun(cmd *protocol.StartRunCommand, appConfig *Config, logger *TaskLogger, encoder *protocol.Encoder, masker *security.Masker) {
	runID := cmd.RunID
	if runID == "" {
		runID = protocol.GenerateRunID()
	}
	
	logger.Log(runID, 0, protocol.Info, fmt.Sprintf("Starting run: %s", runID))
	
	currentRunMutex.Lock()
	currentRunID = runID
	currentRunMutex.Unlock()
	
	servers, err := loadServerConfigs(cmd.ConfigPath)
	if err != nil {
		errorEvent := protocol.NewErrorEvent(protocol.InvalidConfig, err.Error())
		encoder.Encode(protocol.ErrorEvt, runID, "", errorEvent)
		logger.Log(runID, 0, protocol.Error, fmt.Sprintf("Failed to load server configs: %v", err))
		return
	}
	
	logger.Log(runID, 0, protocol.Info, fmt.Sprintf("Loaded %d server configurations", len(servers)))
	
	createOutputDirectories(runID)
	
	globalLogPath := filepath.Join("output/logs", runID+".log")
	if err := logger.OpenLogFile(globalLogPath); err != nil {
		logger.Log(runID, 0, protocol.Warn, fmt.Sprintf("Failed to open log file: %v", err))
	}
	defer logger.Close()
	
	runStartedEvent := protocol.NewRunStartedEvent(runID, len(servers), cmd.Concurrency, (cmd.DryRun || cmd.DNSDryRun))
	encoder.Encode(protocol.RunStarted, runID, "", runStartedEvent)
	
	schedConfig := &scheduler.Config{
		SSHTimeoutMs:   appConfig.SSHTimeoutMs,
		CmdTimeoutMs:   appConfig.CmdTimeoutMs,
		DKIMSelector:   appConfig.DKIMSelector,
		SPFTemplate:    appConfig.SPFTemplate,
		DMARCTemplate:  appConfig.DMARCTemplate,
	}
	
	sched := scheduler.NewScheduler(
		cmd.Concurrency,
		appConfig.RetryMax,
		time.Duration(appConfig.RetryBackoffMs)*time.Millisecond,
		encoder,
		logger,
		schedConfig,
		cmd.DNSDryRun,
		runID,
		masker,
	)
	
	currentRunMutex.Lock()
	currentScheduler = sched
	currentRunMutex.Unlock()
	
	for _, server := range servers {
		task := &scheduler.Task{
			RowID: server.RowID,
			Server: server,
		}
		sched.AddTask(task)
	}
	
	startTime := time.Now()
	if err := sched.Start(); err != nil {
		errorEvent := protocol.NewErrorEvent(protocol.RemoteCmdTransient, err.Error())
		encoder.Encode(protocol.ErrorEvt, runID, "", errorEvent)
		return
	}
	
	progressTicker := time.NewTicker(500 * time.Millisecond)
	defer progressTicker.Stop()
	
	done := make(chan struct{})
	go func() {
		for {
			completed, success, failed, cancelled, running, pending := sched.GetProgress()
			
			progressEvent := protocol.NewRunProgressEvent(runID, completed, len(servers), success, failed, cancelled, running, pending)
			encoder.Encode(protocol.RunProgress, runID, "", progressEvent)
			
			if completed == len(servers) {
				close(done)
				return
			}
			
			<-progressTicker.C
		}
	}()
	
	<-done
	duration := time.Since(startTime).Milliseconds()
	
	outputs := map[string]string{
		"success_list": filepath.Join("output", "results", "success.txt"),
		"failed_list":  filepath.Join("output", "results", "failed.txt"),
		"log_dir":      filepath.Join("output", "logs"),
		"report_dir":   filepath.Join("output", "reports", runID),
	}
	
	_, success, failed, cancelled, _, _ := sched.GetProgress()
	finishedEvent := protocol.NewRunFinishedEvent(runID, "COMPLETED", len(servers), success, failed, cancelled, outputs)
	finishedEvent.DurationMs = duration
	encoder.Encode(protocol.RunFinished, runID, "", finishedEvent)
	
	logger.Log(runID, 0, protocol.Info, fmt.Sprintf("Run completed: %d success, %d failed, %d cancelled in %dms", success, failed, cancelled, duration))
	
	currentRunMutex.Lock()
	currentScheduler = nil
	currentRunID = ""
	currentRunMutex.Unlock()
}

func runOnceMode(appConfig *Config) {
	if *configPathFlag == "" {
		fmt.Fprintf(os.Stderr, "Error: --config flag is required in run-once mode\n")
		os.Exit(1)
	}
	
	concurrency := *concurrencyFlag
	if concurrency <= 0 {
		concurrency = appConfig.ConcurrencyDefault
	}
	
	dnsDryRun := *dnsDryRunFlag
	if !dnsDryRun {
		dnsDryRun = appConfig.DNSDryRunDefault
	}
	
	cmd := &protocol.StartRunCommand{
		Type_:       "START_RUN",
		RunID:       protocol.GenerateRunID(),
		ConfigPath:  *configPathFlag,
		Concurrency: concurrency,
		DNSDryRun:   dnsDryRun,
	}
	
	fmt.Fprintf(os.Stderr, "Running deployment with config: %s\n", *configPathFlag)
	fmt.Fprintf(os.Stderr, "Concurrency: %d\n", concurrency)
	fmt.Fprintf(os.Stderr, "DNS Dry-run: %v\n", dnsDryRun)
	
	masker := security.NewMasker()
	encoder := protocol.NewEncoder(os.Stdout)
	taskLogger := NewTaskLogger(masker, encoder)
	
	handleStartRun(cmd, appConfig, taskLogger, encoder, masker)
}

func createOutputDirectories(runID string) {
	dirs := []string{
		"output/logs",
		filepath.Join("output/logs", runID),
		"output/results",
		filepath.Join("output/reports", runID),
	}
	
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}
}
