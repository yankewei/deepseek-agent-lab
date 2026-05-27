package policy

import "testing"

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		wantTyp  string
		wantCode CommandPolicyCode
	}{
		{"allow pwd", "pwd", "allow", CodeLowRiskCommandAllowed},
		{"allow bun test", "bun test", "allow", CodeLowRiskCommandAllowed},
		{"prompt bun install", "bun install", "prompt", CodeDependencyChangeRequiresApproval},
		{"prompt bun add", "bun add react", "prompt", CodeDependencyChangeRequiresApproval},
		{"forbidden rm", "rm -rf /", "forbidden", CodeCommandNotAllowed},
		{"forbidden shell operator", "ls && pwd", "forbidden", CodeShellOperatorBlocked},
		{"forbidden empty", "", "forbidden", CodeCommandEmpty},
		{"forbidden unknown", "curl https://evil.com", "forbidden", CodeCommandNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.cmd)
			if got.Type != tt.wantTyp {
				t.Errorf("Evaluate(%q).Type = %q, want %q", tt.cmd, got.Type, tt.wantTyp)
			}
			if got.Code != tt.wantCode {
				t.Errorf("Evaluate(%q).Code = %q, want %q", tt.cmd, got.Code, tt.wantCode)
			}
		})
	}
}

func TestRuntimePolicy(t *testing.T) {
	rp := NewRuntimePolicy()
	if rp.IsAllowed("bun add react") {
		t.Error("should not be allowed before prefix is added")
	}
	rp.AllowPrefix("bun add")
	if !rp.IsAllowed("bun add react") {
		t.Error("should be allowed after prefix is added")
	}
	if !rp.IsAllowed("bun add") {
		t.Error("exact prefix should be allowed")
	}
	if rp.IsAllowed("bun install") {
		t.Error("different prefix should not be allowed")
	}
}

func TestGetApprovablePrefix(t *testing.T) {
	if got := GetApprovablePrefix("bun add react"); got != "bun add" {
		t.Errorf("GetApprovablePrefix = %q, want bun add", got)
	}
	if got := GetApprovablePrefix("bun install"); got != "bun install" {
		t.Errorf("GetApprovablePrefix = %q, want bun install", got)
	}
	if got := GetApprovablePrefix("pwd"); got != "" {
		t.Errorf("GetApprovablePrefix = %q, want empty", got)
	}
}
