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
		{"prompt bun test", "bun test", "prompt", CodeCommandRequiresApproval},
		{"prompt go test", "go test", "prompt", CodeCommandRequiresApproval},
		{"prompt npm test", "npm test", "prompt", CodeCommandRequiresApproval},
		{"prompt make test", "make test", "prompt", CodeCommandRequiresApproval},
		{"prompt cargo build", "cargo build", "prompt", CodeCommandRequiresApproval},
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
		t.Error("should not be allowed before command is added")
	}
	rp.AllowCommand("bun add react")
	if !rp.IsAllowed("bun add react") {
		t.Error("should allow the exact command")
	}
	if rp.IsAllowed("bun add react-dom") {
		t.Error("different arguments should not be allowed")
	}
	if rp.IsAllowed("bun install") {
		t.Error("different command should not be allowed")
	}

	// Cross-ecosystem runtime policy.
	rp2 := NewRuntimePolicy()
	rp2.AllowCommand("go get github.com/foo/bar")
	if !rp2.IsAllowed("go get github.com/foo/bar") {
		t.Error("exact go get command should be allowed")
	}
	if rp2.IsAllowed("go mod download") {
		t.Error("different go command should not be allowed")
	}
}

func TestGetApprovableCommand(t *testing.T) {
	if got := GetApprovableCommand("bun add react"); got != "bun add react" {
		t.Errorf("GetApprovableCommand = %q, want bun add react", got)
	}
	if got := GetApprovableCommand("bun install"); got != "bun install" {
		t.Errorf("GetApprovableCommand = %q, want bun install", got)
	}
	if got := GetApprovableCommand("go get github.com/foo/bar"); got != "go get github.com/foo/bar" {
		t.Errorf("GetApprovableCommand = %q, want exact go get command", got)
	}
	if got := GetApprovableCommand("npm install"); got != "npm install" {
		t.Errorf("GetApprovableCommand = %q, want npm install", got)
	}
	if got := GetApprovableCommand("pwd"); got != "pwd" {
		t.Errorf("GetApprovableCommand = %q, want pwd", got)
	}
	if got := GetApprovableCommand("rm -rf /"); got != "rm -rf /" {
		t.Errorf("GetApprovableCommand = %q, want exact rm command", got)
	}
	if got := GetApprovableCommand(""); got != "" {
		t.Errorf("GetApprovableCommand = %q, want empty", got)
	}
}
