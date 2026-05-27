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
	CodeLowRiskCommandAllowed            CommandPolicyCode = "LOW_RISK_COMMAND_ALLOWED"
	CodeDependencyChangeRequiresApproval CommandPolicyCode = "DEPENDENCY_CHANGE_REQUIRES_APPROVAL"
	CodeCommandEmpty                     CommandPolicyCode = "COMMAND_EMPTY"
	CodeShellOperatorBlocked             CommandPolicyCode = "SHELL_OPERATOR_BLOCKED"
	CodeCommandNotAllowed                CommandPolicyCode = "COMMAND_NOT_ALLOWED"
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
		"pwd":               {},
		"bun test":          {},
		"bun run build:bin": {},
		"bun --version":     {},
	}
	approvablePrefixes = []string{"bun install", "bun add", "bun remove"}
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
		return Decision{Type: "allow", Code: CodeLowRiskCommandAllowed, Reason: "Known low-risk project command.", Command: normalized}
	}

	if prefix := GetApprovablePrefix(normalized); prefix != "" {
		return Decision{
			Type:      "prompt",
			Code:      CodeDependencyChangeRequiresApproval,
			Reason:    "Dependency command requires user approval.",
			Command:   normalized,
			RiskLevel: RiskMedium,
		}
	}

	return Decision{Type: "forbidden", Code: CodeCommandNotAllowed, Reason: "Command is not allowed: " + normalized, Command: normalized}
}

// RuntimePolicy allows dynamic command prefix allowances during a session.
type RuntimePolicy struct {
	allowedPrefixes map[string]struct{}
}

// NewRuntimePolicy creates a fresh runtime policy store.
func NewRuntimePolicy() *RuntimePolicy {
	return &RuntimePolicy{allowedPrefixes: make(map[string]struct{})}
}

// AllowPrefix permanently allows a command prefix for this session.
func (r *RuntimePolicy) AllowPrefix(prefix string) {
	r.allowedPrefixes[normalize(prefix)] = struct{}{}
}

// IsAllowed checks if a command matches a previously allowed prefix.
func (r *RuntimePolicy) IsAllowed(command string) bool {
	normalized := normalize(command)
	for prefix := range r.allowedPrefixes {
		if normalized == prefix || strings.HasPrefix(normalized, prefix+" ") {
			return true
		}
	}
	return false
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

// GetApprovablePrefix returns the matching approvable prefix for a command.
func GetApprovablePrefix(cmd string) string {
	for _, prefix := range approvablePrefixes {
		if cmd == prefix || strings.HasPrefix(cmd, prefix+" ") {
			return prefix
		}
	}
	return ""
}
