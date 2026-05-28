package runlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
)

type Clock func() time.Time

type Options struct {
	CWD        string
	UserPrompt string
	RootDir    string
	RunID      string
	Now        Clock
}

type Logger struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	path    string
	runID   string
	now     Clock
	closed  bool
	err     error
}

type SessionMetaEvent struct {
	Type       string `json:"type"`
	Timestamp  string `json:"timestamp"`
	RunID      string `json:"runId"`
	StartedAt  string `json:"startedAt"`
	CWD        string `json:"cwd"`
	UserPrompt string `json:"userPrompt"`
	Status     string `json:"status"`
}

type RunStatusChangedEvent struct {
	Type        string `json:"type"`
	Timestamp   string `json:"timestamp"`
	RunID       string `json:"runId"`
	Status      string `json:"status"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type UserMessageEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
}

type ConversationClearedEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
}

type ModelStreamStartedEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	RunID     string `json:"runId"`
}

type ModelTextEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
}

type ModelReasoningEvent struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
}

type ModelStreamFinishedEvent struct {
	Type         string    `json:"type"`
	Timestamp    string    `json:"timestamp"`
	FinishReason string    `json:"finishReason"`
	Usage        llm.Usage `json:"usage,omitempty"`
}

type ToolCallEvent struct {
	Type       string          `json:"type"`
	Timestamp  string          `json:"timestamp"`
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName"`
	Input      json.RawMessage `json:"input"`
}

type ToolResultEvent struct {
	Type        string          `json:"type"`
	Timestamp   string          `json:"timestamp"`
	ToolCallID  string          `json:"toolCallId"`
	ToolName    string          `json:"toolName"`
	Output      json.RawMessage `json:"output"`
	ExecutionID string          `json:"executionId,omitempty"`
}

type ApprovalRequestedEvent struct {
	Type        string           `json:"type"`
	Timestamp   string           `json:"timestamp"`
	ApprovalID  string           `json:"approvalId"`
	Request     approval.Request `json:"request"`
	ExecutionID string           `json:"executionId,omitempty"`
}

type ApprovalResolvedEvent struct {
	Type        string          `json:"type"`
	Timestamp   string          `json:"timestamp"`
	ApprovalID  string          `json:"approvalId"`
	Result      approval.Result `json:"result"`
	ExecutionID string          `json:"executionId,omitempty"`
}

type ExecutionStateChangedEvent struct {
	Type      string           `json:"type"`
	Sequence  int              `json:"sequence"`
	Timestamp string           `json:"timestamp"`
	Record    execution.Record `json:"record"`
}

var runIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// OpenExisting opens an existing run log for append-only writing.
func OpenExisting(path string) (*Logger, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open run log for append: %w", err)
	}

	runID := filepath.Base(path)
	runID = strings.TrimSuffix(runID, filepath.Ext(runID))
	if !runIDPattern.MatchString(runID) {
		_ = file.Close()
		return nil, fmt.Errorf("invalid run id from path: %s", runID)
	}

	return &Logger{
		file:    file,
		encoder: json.NewEncoder(file),
		path:    path,
		runID:   runID,
		now:     func() time.Time { return time.Now().UTC() },
	}, nil
}

func CreateRun(opts Options) (*Logger, error) {
	if opts.CWD == "" {
		return nil, errors.New("runlog cwd is required")
	}

	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	runID := opts.RunID
	if runID == "" {
		runID = CreateRunID(now())
	}
	if !runIDPattern.MatchString(runID) {
		return nil, fmt.Errorf("invalid run id: %s", runID)
	}

	rootDir := opts.RootDir
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		rootDir = filepath.Join(home, ".disco")
	}

	path := RunLogPath(rootDir, opts.CWD, runID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	logger := &Logger{
		file:    file,
		encoder: json.NewEncoder(file),
		path:    path,
		runID:   runID,
		now:     now,
	}

	startedAt := logger.timestamp()
	if err := logger.Append(SessionMetaEvent{
		Type:       "session_meta",
		Timestamp:  startedAt,
		RunID:      runID,
		StartedAt:  startedAt,
		CWD:        opts.CWD,
		UserPrompt: opts.UserPrompt,
		Status:     "running",
	}); err != nil {
		_ = file.Close()
		return nil, err
	}

	return logger, nil
}

func CreateRunID(now time.Time) string {
	ts := now.UTC().Format("20060102T150405000000000Z")
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	return "run_" + ts + "_" + suffix
}

func ProjectSlug(cwd string) string {
	name := sanitizeProjectName(filepath.Base(cwd))
	sum := sha256.Sum256([]byte(cwd))
	return name + "-" + hex.EncodeToString(sum[:])[:8]
}

func RunLogPath(rootDir, cwd, runID string) string {
	if rootDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			rootDir = filepath.Join(home, ".disco")
		}
	}
	return filepath.Join(rootDir, "projects", ProjectSlug(cwd), "runs", runID+".jsonl")
}

func (l *Logger) Append(event any) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		err := errors.New("run log is closed")
		l.recordErr(err)
		return err
	}
	if err := l.encoder.Encode(event); err != nil {
		l.recordErr(err)
		return err
	}
	return nil
}

func (l *Logger) Close() error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return l.err
	}
	l.closed = true
	if err := l.file.Close(); err != nil {
		l.recordErr(err)
		return err
	}
	return l.err
}

func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *Logger) RunID() string {
	if l == nil {
		return ""
	}
	return l.runID
}

func (l *Logger) Err() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

func (l *Logger) AppendRunStatus(status string) error {
	if l == nil {
		return nil
	}
	event := RunStatusChangedEvent{
		Type:      "run_status_changed",
		Timestamp: l.timestamp(),
		RunID:     l.runID,
		Status:    status,
	}
	if status == "completed" || status == "failed" || status == "interrupted" {
		event.CompletedAt = event.Timestamp
	}
	return l.Append(event)
}

func (l *Logger) AppendUserMessage(text string) error {
	if l == nil {
		return nil
	}
	return l.Append(UserMessageEvent{Type: "user_message", Timestamp: l.timestamp(), Text: text})
}

func (l *Logger) AppendConversationCleared() error {
	if l == nil {
		return nil
	}
	return l.Append(ConversationClearedEvent{Type: "conversation_cleared", Timestamp: l.timestamp()})
}

func (l *Logger) AppendModelStreamStarted() error {
	if l == nil {
		return nil
	}
	return l.Append(ModelStreamStartedEvent{Type: "model_stream_started", Timestamp: l.timestamp(), RunID: l.runID})
}

func (l *Logger) AppendModelText(text string) error {
	if l == nil {
		return nil
	}
	if text == "" {
		return nil
	}
	return l.Append(ModelTextEvent{Type: "model_text", Timestamp: l.timestamp(), Text: text})
}

func (l *Logger) AppendModelReasoning(text string) error {
	if l == nil {
		return nil
	}
	if text == "" {
		return nil
	}
	return l.Append(ModelReasoningEvent{Type: "model_reasoning", Timestamp: l.timestamp(), Text: text})
}

func (l *Logger) AppendModelStreamFinished(finishReason string, usage llm.Usage) error {
	if l == nil {
		return nil
	}
	return l.Append(ModelStreamFinishedEvent{
		Type:         "model_stream_finished",
		Timestamp:    l.timestamp(),
		FinishReason: finishReason,
		Usage:        usage,
	})
}

func (l *Logger) AppendToolCall(toolCallID, toolName string, input json.RawMessage) error {
	if l == nil {
		return nil
	}
	return l.Append(ToolCallEvent{
		Type:       "tool_call",
		Timestamp:  l.timestamp(),
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Input:      json.RawMessage(append([]byte(nil), input...)),
	})
}

func (l *Logger) AppendToolResult(toolCallID, toolName, content, executionID string) error {
	if l == nil {
		return nil
	}
	return l.Append(ToolResultEvent{
		Type:        "tool_result",
		Timestamp:   l.timestamp(),
		ToolCallID:  toolCallID,
		ToolName:    toolName,
		Output:      rawJSONOrString(content),
		ExecutionID: executionID,
	})
}

func (l *Logger) AppendApprovalRequested(req approval.Request, executionID string) (string, error) {
	if l == nil {
		return "", nil
	}
	approvalID := "approval_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	return approvalID, l.Append(ApprovalRequestedEvent{
		Type:        "approval_requested",
		Timestamp:   l.timestamp(),
		ApprovalID:  approvalID,
		Request:     req,
		ExecutionID: executionID,
	})
}

func (l *Logger) AppendApprovalResolved(approvalID string, result approval.Result, executionID string) error {
	if l == nil {
		return nil
	}
	if approvalID == "" {
		approvalID = "approval_unknown"
	}
	return l.Append(ApprovalResolvedEvent{
		Type:        "approval_resolved",
		Timestamp:   l.timestamp(),
		ApprovalID:  approvalID,
		Result:      result,
		ExecutionID: executionID,
	})
}

func (l *Logger) AppendExecutionEvent(ev execution.Event) error {
	if l == nil {
		return nil
	}
	return l.Append(ExecutionStateChangedEvent{
		Type:      "execution_state_changed",
		Sequence:  ev.Sequence,
		Timestamp: ev.At.UTC().Format(time.RFC3339Nano),
		Record:    ev.Record,
	})
}

func (l *Logger) timestamp() string {
	if l == nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return l.now().UTC().Format(time.RFC3339Nano)
}

func (l *Logger) recordErr(err error) {
	if l.err == nil {
		l.err = err
	}
}

func sanitizeProjectName(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "project"
	}
	return out
}

func rawJSONOrString(s string) json.RawMessage {
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	b, _ := json.Marshal(s)
	return b
}
