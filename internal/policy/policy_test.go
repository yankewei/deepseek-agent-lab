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
		{"allow go test", "go test", "allow", CodeLowRiskCommandAllowed},
		{"allow npm test", "npm test", "allow", CodeLowRiskCommandAllowed},
		{"allow make test", "make test", "allow", CodeLowRiskCommandAllowed},
		{"allow cargo build", "cargo build", "allow", CodeLowRiskCommandAllowed},
		{"prompt bun install", "bun install", "prompt", CodeCommandRequiresApproval},
		{"prompt bun add", "bun add react", "prompt", CodeCommandRequiresApproval},
		{"prompt npm install", "npm install", "prompt", CodeCommandRequiresApproval},
		{"prompt go get", "go get github.com/foo/bar", "prompt", CodeCommandRequiresApproval},
		{"prompt rm", "rm -rf /", "prompt", CodeCommandRequiresApproval},
		{"prompt curl", "curl https://evil.com", "prompt", CodeCommandRequiresApproval},
		{"forbidden shell operator", "ls && pwd", "forbidden", CodeShellOperatorBlocked},
		{"forbidden empty", "", "forbidden", CodeCommandEmpty},
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

	// Cross-ecosystem runtime policy.
	rp2 := NewRuntimePolicy()
	rp2.AllowPrefix("go get")
	if !rp2.IsAllowed("go get github.com/foo/bar") {
		t.Error("go get prefix should be allowed")
	}
	if rp2.IsAllowed("go mod download") {
		t.Error("different go prefix should not be allowed")
	}
}

func TestGetApprovablePrefix(t *testing.T) {
	if got := GetApprovablePrefix("bun add react"); got != "bun add" {
		t.Errorf("GetApprovablePrefix = %q, want bun add", got)
	}
	if got := GetApprovablePrefix("bun install"); got != "bun install" {
		t.Errorf("GetApprovablePrefix = %q, want bun install", got)
	}
	if got := GetApprovablePrefix("go get github.com/foo/bar"); got != "go get" {
		t.Errorf("GetApprovablePrefix = %q, want go get", got)
	}
	if got := GetApprovablePrefix("npm install"); got != "npm install" {
		t.Errorf("GetApprovablePrefix = %q, want npm install", got)
	}
	if got := GetApprovablePrefix("pwd"); got != "pwd" {
		t.Errorf("GetApprovablePrefix = %q, want pwd", got)
	}
	if got := GetApprovablePrefix("rm -rf /"); got != "rm -rf" {
		t.Errorf("GetApprovablePrefix = %q, want rm -rf", got)
	}
	if got := GetApprovablePrefix(""); got != "" {
		t.Errorf("GetApprovablePrefix = %q, want empty", got)
	}
}
