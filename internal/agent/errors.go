package agent

// ErrorCode categorizes tool and execution errors.
type ErrorCode string

const (
	ErrPolicyForbidden        ErrorCode = "POLICY_FORBIDDEN"
	ErrApprovalReasonRequired ErrorCode = "APPROVAL_REASON_REQUIRED"
	ErrExecutionFailed        ErrorCode = "EXECUTION_FAILED"
	ErrInvalidInput           ErrorCode = "INVALID_INPUT"
	ErrPathBlocked            ErrorCode = "PATH_BLOCKED"
	ErrPatchParseFailed       ErrorCode = "PATCH_PARSE_FAILED"
)
