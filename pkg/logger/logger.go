package logger

// Logger is the structured logging interface. Implementations must be safe
// for concurrent use. With returns a new Logger carrying the extra field.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Fatal(msg string, args ...any)
	With(key string, value any) Logger
}

// Config controls logger output format and level.
type Config struct {
	// Env selects output format: "DEV" (colorized text) or "PROD" (JSON).
	Env string
	// Level sets minimum log level: "debug", "info", "warn", "error".
	Level string
}
