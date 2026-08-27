package httpclient

type Logger interface {
	DebugF(format string, args ...any)
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

type NoopLogger struct{}

func (NoopLogger) DebugF(format string, args ...any) {}
func (NoopLogger) Debug(msg string)                  {}
func (NoopLogger) Info(msg string)                   {}
func (NoopLogger) Warn(msg string)                   {}
func (NoopLogger) Error(msg string)                  {}
