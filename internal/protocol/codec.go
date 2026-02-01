package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Encoder writes NDJSON events to stdout
type Encoder struct {
	writer *bufio.Writer
}

// NewEncoder creates a new NDJSON encoder
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{
		writer: bufio.NewWriter(w),
	}
}

// Encode writes an event as NDJSON line with envelope
func (e *Encoder) Encode(eventType EventType, runID, rowID string, data any) error {
	envelope := NewEnvelope(eventType, runID, rowID, data)
	dataBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope: %w", err)
	}

	_, err = e.writer.Write(dataBytes)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	_, err = e.writer.Write([]byte("\n"))
	if err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return e.writer.Flush()
}

// Decode reads the next command
func (d *Decoder) Decode() (interface{}, error) {
	if !d.scanner.Scan() {
		if err := d.scanner.Err(); err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}
		return nil, io.EOF
	}

	line := d.scanner.Text()

	// Parse type field first
	var typeStruct struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(line), &typeStruct); err != nil {
		return nil, fmt.Errorf("failed to parse type: %w", err)
	}

	// Decode based on type
	switch CommandType(typeStruct.Type) {
	case StartRun:
		var cmd StartRunCommand
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			return nil, fmt.Errorf("failed to parse START_RUN: %w", err)
		}
		return &cmd, nil

	case CancelRun:
		var cmd CancelRunCommand
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			return nil, fmt.Errorf("failed to parse CANCEL_RUN: %w", err)
		}
		return &cmd, nil

	case CancelTask:
		var cmd CancelTaskCommand
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			return nil, fmt.Errorf("failed to parse CANCEL_TASK: %w", err)
		}
		return &cmd, nil

	case Ping:
		var cmd PingCommand
		if err := json.Unmarshal([]byte(line), &cmd); err != nil {
			return nil, fmt.Errorf("failed to parse PING: %w", err)
		}
		return &cmd, nil

	default:
		return nil, fmt.Errorf("unknown command type: %s", typeStruct.Type)
	}
}

// Decoder reads NDJSON commands from stdin
type Decoder struct {
	scanner *bufio.Scanner
}

// NewDecoder creates a new NDJSON decoder
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		scanner: bufio.NewScanner(r),
	}
}

// Helper functions for creating events

func NewRunStartedEvent(runID string, total, concurrency int, dryRun bool) *RunStartedEvent {
	return &RunStartedEvent{
		RunID:       runID,
		TotalTasks:  total,
		Concurrency: concurrency,
		DryRun:      dryRun,
	}
}

func NewRunProgressEvent(runID string, completed, total, success, failed, cancelled, running, pending int) *RunProgressEvent {
	return &RunProgressEvent{
		RunID:     runID,
		Completed: completed,
		Total:     total,
		Success:   success,
		Failed:    failed,
		Cancelled: cancelled,
		Running:   running,
		Pending:   pending,
	}
}

func NewTaskStateEvent(rowID int, state TaskState, message string) *TaskStateEvent {
	return &TaskStateEvent{
		RowID:   rowID,
		State:   state,
		Message: message,
	}
}

func NewTaskStateEventWithRetry(rowID int, state TaskState, message string, retries int) *TaskStateEvent {
	return &TaskStateEvent{
		RowID:   rowID,
		State:   state,
		Message: message,
		Retries: retries,
	}
}

func NewTaskStepStartEvent(rowID int, step, message string) *TaskStepEvent {
	return &TaskStepEvent{
		RowID:   rowID,
		Step:    step,
		Phase:   StepStart,
		Message: message,
	}
}

func NewTaskStepEndEvent(rowID int, step, message string, success bool) *TaskStepEvent {
	return &TaskStepEvent{
		RowID:   rowID,
		Step:    step,
		Phase:   StepEnd,
		Message: message,
		Success: success,
	}
}

func NewLogLineEvent(level LogLevel, message string) *LogLineEvent {
	return &LogLineEvent{
		Level:     level,
		Message:   message,
		Timestamp: GetCurrentTimestamp(),
	}
}

func NewErrorEvent(code ErrorCode, message string, rowID ...int) *ErrorEvent {
	event := &ErrorEvent{
		Code:    code,
		Message: message,
	}
	if len(rowID) > 0 {
		event.RowID = rowID[0]
	}
	return event
}

func NewRunFinishedEvent(runID, status string, total, success, failed, cancelled int, outputs map[string]string) *RunFinishedEvent {
	return &RunFinishedEvent{
		RunID:      runID,
		Status:     status,
		TotalTasks: total,
		Success:    success,
		Failed:     failed,
		Cancelled:  cancelled,
		Outputs:    outputs,
	}
}