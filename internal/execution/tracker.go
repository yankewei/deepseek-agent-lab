package execution

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle of an execution record.
type Status string

const (
	StatusCreated         Status = "created"
	StatusPolicyEvaluated Status = "policy_evaluated"
	StatusWaitingApproval Status = "waiting_for_approval"
	StatusApproved        Status = "approved"
	StatusDenied          Status = "denied"
	StatusRunning         Status = "running"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
)

// Record tracks a single tool or command execution.
type Record struct {
	ID                string         `json:"id"`
	Kind              string         `json:"kind"` // "command" or "tool"
	Command           string         `json:"command,omitempty"`
	ToolName          string         `json:"tool_name,omitempty"`
	Reason            string         `json:"reason,omitempty"`
	Status            Status         `json:"status"`
	StartedAt         time.Time      `json:"started_at"`
	CompletedAt       *time.Time     `json:"completed_at,omitempty"`
	DurationMs        *int64         `json:"duration_ms,omitempty"`
	PolicyDecision    string         `json:"policy_decision,omitempty"`
	PolicyCode        string         `json:"policy_code,omitempty"`
	PolicyReason      string         `json:"policy_reason,omitempty"`
	NormalizedCommand string         `json:"normalized_command,omitempty"`
	ExitCode          *int           `json:"exit_code,omitempty"`
	Error             string         `json:"error,omitempty"`
	History           []HistoryEntry `json:"history"`
}

// HistoryEntry is a single status transition.
type HistoryEntry struct {
	Status Status    `json:"status"`
	At     time.Time `json:"at"`
}

// Event is emitted when a record changes.
type Event struct {
	Type     string    `json:"type"`
	Sequence int       `json:"sequence"`
	At       time.Time `json:"at"`
	Record   Record    `json:"record"`
}

// Tracker manages execution records in memory.
type Tracker struct {
	mu       sync.RWMutex
	records  map[string]*Record
	sequence int
	onEvent  func(Event)
}

// NewTracker creates an execution tracker.
func NewTracker(onEvent func(Event)) *Tracker {
	return &Tracker{
		records: make(map[string]*Record),
		onEvent: onEvent,
	}
}

// CreateRecord initializes a new execution record.
func (t *Tracker) CreateRecord(kind, name, command, reason string) *Record {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC()
	rec := &Record{
		ID:        uuid.New().String(),
		Kind:      kind,
		StartedAt: now,
		Status:    StatusCreated,
		History:   []HistoryEntry{{Status: StatusCreated, At: now}},
	}
	if kind == "command" {
		rec.Command = command
		rec.Reason = reason
	} else {
		rec.ToolName = name
	}

	t.records[rec.ID] = rec
	t.emit(rec)
	return rec
}

// UpdateRecord mutates a record and emits an event.
func (t *Tracker) UpdateRecord(id string, updates map[string]any) *Record {
	t.mu.Lock()
	defer t.mu.Unlock()

	rec, ok := t.records[id]
	if !ok {
		return nil
	}

	// Apply simple updates via reflection helper or direct field mapping.
	// For now, use explicit type assertions for known fields.
	if v, ok := updates["status"]; ok {
		if s, ok := v.(Status); ok {
			rec.Status = s
			now := time.Now().UTC()
			rec.History = append(rec.History, HistoryEntry{Status: s, At: now})
			if isTerminal(s) {
				rec.CompletedAt = &now
				ms := now.Sub(rec.StartedAt).Milliseconds()
				rec.DurationMs = &ms
			}
		}
	}
	if v, ok := updates["policy_decision"]; ok {
		rec.PolicyDecision, _ = v.(string)
	}
	if v, ok := updates["policy_code"]; ok {
		rec.PolicyCode, _ = v.(string)
	}
	if v, ok := updates["policy_reason"]; ok {
		rec.PolicyReason, _ = v.(string)
	}
	if v, ok := updates["normalized_command"]; ok {
		rec.NormalizedCommand, _ = v.(string)
	}
	if v, ok := updates["exit_code"]; ok {
		if i, ok := v.(int); ok {
			rec.ExitCode = &i
		}
	}
	if v, ok := updates["error"]; ok {
		rec.Error, _ = v.(string)
	}

	t.emit(rec)
	return rec
}

// GetRecord returns a record by ID.
func (t *Tracker) GetRecord(id string) *Record {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return cloneRecord(t.records[id])
}

// ListRecords returns all records.
func (t *Tracker) ListRecords() []Record {
	t.mu.RLock()
	defer t.mu.RUnlock()

	out := make([]Record, 0, len(t.records))
	for _, rec := range t.records {
		out = append(out, *cloneRecord(rec))
	}
	return out
}

func (t *Tracker) emit(rec *Record) {
	if t.onEvent == nil {
		return
	}
	t.sequence++
	t.onEvent(Event{
		Type:     "execution_state_changed",
		Sequence: t.sequence,
		At:       time.Now().UTC(),
		Record:   *cloneRecord(rec),
	})
}

func cloneRecord(rec *Record) *Record {
	if rec == nil {
		return nil
	}
	cp := *rec
	cp.History = make([]HistoryEntry, len(rec.History))
	copy(cp.History, rec.History)
	return &cp
}

func isTerminal(s Status) bool {
	return s == StatusCompleted || s == StatusDenied || s == StatusFailed
}
