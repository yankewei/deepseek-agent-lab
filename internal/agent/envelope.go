package agent

// Result is the Go equivalent of AgentToolResult<T>.
type Result[T any] struct {
	OK    bool           `json:"ok"`
	Data  T              `json:"data,omitempty"`
	Error *AgentError    `json:"error,omitempty"`
	Meta  map[string]any `json:"meta,omitempty"`
}

// AgentError is the shared error shape for tools.
type AgentError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *AgentError) Error() string {
	return e.Message
}

// OK creates a successful result.
func OK[T any](data T, meta map[string]any) Result[T] {
	return Result[T]{
		OK:   true,
		Data: data,
		Meta: meta,
	}
}

// Err creates an error result.
func Err[T any](code, message string, meta map[string]any) Result[T] {
	return Result[T]{
		OK: false,
		Error: &AgentError{
			Code:    code,
			Message: message,
		},
		Meta: meta,
	}
}

// Wrap executes a function and wraps its result or error into a Result.
func Wrap[T any](fn func() (T, error), meta map[string]any) Result[T] {
	data, err := fn()
	if err != nil {
		return Err[T]("EXECUTION_FAILED", err.Error(), meta)
	}
	return OK(data, meta)
}
