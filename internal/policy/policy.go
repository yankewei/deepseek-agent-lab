package policy

import (
	"strings"
)

// RiskLevel represents the risk classification of a command.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// CommandPolicyCode categorizes policy decisions.
type CommandPolicyCode string

const (
	CodeLowRiskCommandAllowed   CommandPolicyCode = "LOW_RISK_COMMAND_ALLOWED"
	CodeCommandRequiresApproval CommandPolicyCode = "COMMAND_REQUIRES_APPROVAL"
	CodeCommandEmpty            CommandPolicyCode = "COMMAND_EMPTY"
	CodeShellOperatorBlocked    CommandPolicyCode = "SHELL_OPERATOR_BLOCKED"
	CodeCommandNotAllowed       CommandPolicyCode = "COMMAND_NOT_ALLOWED"
)

// Decision is the result of policy evaluation.
type Decision struct {
	Type      string            `json:"type"` // "allow", "prompt", "forbidden"
	Code      CommandPolicyCode `json:"code"`
	Reason    string            `json:"reason"`
	Command   string            `json:"command"`
	RiskLevel RiskLevel         `json:"risk_level,omitempty"`
}

var (
	blockedShellTokens = []string{"&&", "||", ";", "|", ">", "<", "`", "$(", "$(("}
	allowedCommands    = map[string]struct{}{
		"pwd": {},
		// bun
		"bun --version": {},
		// npm
		"npm --version": {},
		// yarn
		"yarn --version": {},
		// pnpm
		"pnpm --version": {},
		// go
		"go version": {},
		// cargo
		"cargo --version": {},
	}
)

// Evaluate determines whether a command is allowed, requires approval, or is forbidden.
func Evaluate(command string) Decision {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return Decision{Type: "forbidden", Code: CodeCommandEmpty, Reason: "Command cannot be empty.", Command: ""}
	}

	if hasShellOperator(trimmed) {
		return Decision{
			Type:    "forbidden",
			Code:    CodeShellOperatorBlocked,
			Reason:  "Shell operator is not allowed in command: " + trimmed,
			Command: trimmed,
		}
	}

	normalized := normalize(trimmed)
	if _, ok := allowedCommands[normalized]; ok {
		return Decision{Type: "allow", Code: CodeLowRiskCommandAllowed, Reason: "Known low-risk read-only command.", Command: normalized}
	}

	return Decision{
		Type:      "prompt",
		Code:      CodeCommandRequiresApproval,
		Reason:    "Command requires user approval.",
		Command:   normalized,
		RiskLevel: RiskMedium,
	}
}

// RuntimePolicy allows dynamic exact-command allowances during a session.
type RuntimePolicy struct {
	allowedCommands map[string]struct{}
}

// NewRuntimePolicy creates a fresh runtime policy store.
func NewRuntimePolicy() *RuntimePolicy {
	return &RuntimePolicy{allowedCommands: make(map[string]struct{})}
}

// AllowCommand permanently allows one exact command for this session.
func (r *RuntimePolicy) AllowCommand(command string) {
	r.allowedCommands[normalize(command)] = struct{}{}
}

// IsAllowed checks if a command exactly matches a previously allowed command.
func (r *RuntimePolicy) IsAllowed(command string) bool {
	normalized := normalize(command)
	_, ok := r.allowedCommands[normalized]
	return ok
}

func normalize(cmd string) string {
	return strings.Join(strings.Fields(cmd), " ")
}

func hasShellOperator(cmd string) bool {
	for _, token := range blockedShellTokens {
		if strings.Contains(cmd, token) {
			return true
		}
	}
	return false
}

// GetApprovableCommand returns the normalized exact command that can be
// approved for the current session.
func GetApprovableCommand(cmd string) string {
	return normalize(cmd)
}
