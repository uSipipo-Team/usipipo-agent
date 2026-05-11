package logging

// EventType represents the type of security event
type EventType string

const (
	// AuthFailureEvent - Authentication failure
	AuthFailureEvent EventType = "auth_failure"

	// RateLimitExceededEvent - Rate limit triggered
	RateLimitExceededEvent EventType = "rate_limit_exceeded"

	// StartupEvent - Agent startup
	StartupEvent EventType = "startup"

	// ShutdownEvent - Agent shutdown
	ShutdownEvent EventType = "shutdown"

	// ConfigChangeEvent - Configuration change (future)
	ConfigChangeEvent EventType = "config_change"
)

// LogLevel represents the severity level of a log entry
type LogLevel string

const (
	// InfoLevel - Informational messages
	InfoLevel LogLevel = "INFO"

	// WarnLevel - Warning messages
	WarnLevel LogLevel = "WARN"

	// ErrorLevel - Error messages
	ErrorLevel LogLevel = "ERROR"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Event     EventType              `json:"event"`
	IP        string                 `json:"ip,omitempty"`
	Endpoint  string                 `json:"endpoint,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	UserAgent string                 `json:"user_agent,omitempty"`
	ServerID  string                 `json:"server_id,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}
