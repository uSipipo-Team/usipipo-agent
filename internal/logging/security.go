package logging

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// SecurityLogger handles structured security event logging
type SecurityLogger struct {
	mu        sync.Mutex
	out       io.Writer
	level     LogLevel
	serverID  string
	version   string
	startTime time.Time
}

// NewSecurityLogger creates a new security logger
func NewSecurityLogger(serverID, version string, level LogLevel) *SecurityLogger {
	return &SecurityLogger{
		out:       os.Stdout,
		level:     level,
		serverID:  serverID,
		version:   version,
		startTime: time.Now(),
	}
}

// log writes a log entry if the level is sufficient
func (sl *SecurityLogger) log(entry LogEntry) {
	// Check log level
	if !sl.shouldLog(entry.Level) {
		return
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()

	// Add timestamp
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)

	// Add server ID if not present
	if entry.ServerID == "" {
		entry.ServerID = sl.serverID
	}

	// Sanitize all string fields
	entry.IP = sanitizeString(entry.IP)
	entry.Endpoint = sanitizeString(entry.Endpoint)
	entry.Reason = sanitizeString(entry.Reason)
	entry.UserAgent = sanitizeString(entry.UserAgent)
	entry.Message = sanitizeString(entry.Message)

	// Sanitize details recursively
	for key, value := range entry.Details {
		entry.Details[key] = sanitizeValue(value)
	}

	// Encode as JSON
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("ERROR: Failed to marshal log entry: %v", err)
		return
	}
	// Write to output (ignore errors as logging failures shouldn't crash the app)
	_, _ = sl.out.Write(data)
	_, _ = sl.out.Write([]byte("\n"))
}

// shouldLog checks if the entry level should be logged
func (sl *SecurityLogger) shouldLog(level LogLevel) bool {
	// Order: INFO < WARN < ERROR
	levelOrder := map[LogLevel]int{
		InfoLevel:  0,
		WarnLevel:  1,
		ErrorLevel: 2,
	}

	entryLevel := levelOrder[level]
	minLevel := levelOrder[sl.level]

	return entryLevel >= minLevel
}

// LogAuthFailure logs an authentication failure event
func (sl *SecurityLogger) LogAuthFailure(ip, endpoint, reason, userAgent string) {
	sl.log(LogEntry{
		Level:     WarnLevel,
		Event:     AuthFailureEvent,
		IP:        ip,
		Endpoint:  endpoint,
		Reason:    reason,
		UserAgent: userAgent,
		Message:   "Authentication failed",
	})
}

// LogRateLimitExceeded logs a rate limit exceeded event
func (sl *SecurityLogger) LogRateLimitExceeded(ip, endpoint string, requestsPerSecond float64) {
	sl.log(LogEntry{
		Level:    WarnLevel,
		Event:    RateLimitExceededEvent,
		IP:       ip,
		Endpoint: endpoint,
		Reason:   "rate_limit",
		Message:  "Rate limit exceeded",
		Details: map[string]interface{}{
			"requests_per_second": requestsPerSecond,
		},
	})
}

// LogStartup logs an agent startup event
func (sl *SecurityLogger) LogStartup(configHash string) {
	sl.log(LogEntry{
		Level:   InfoLevel,
		Event:   StartupEvent,
		Message: "Agent started",
		Details: map[string]interface{}{
			"version":     sl.version,
			"config_hash": configHash,
			"go_version":  goVersion(),
		},
	})
}

// LogShutdown logs an agent shutdown event
func (sl *SecurityLogger) LogShutdown() {
	uptime := time.Since(sl.startTime).Seconds()
	sl.log(LogEntry{
		Level:   InfoLevel,
		Event:   ShutdownEvent,
		Message: "Agent shutting down",
		Details: map[string]interface{}{
			"uptime_seconds": uptime,
		},
	})
}

// LogConfigChange logs a configuration change event (future use)
func (sl *SecurityLogger) LogConfigChange(ip, endpoint, changedBy, field string) {
	sl.log(LogEntry{
		Level:     InfoLevel,
		Event:     ConfigChangeEvent,
		IP:        ip,
		Endpoint:  endpoint,
		Reason:    "config_modified",
		UserAgent: "",
		Message:   "Configuration changed",
		Details: map[string]interface{}{
			"changed_by": sanitizeString(changedBy),
			"field":      field,
		},
	})
}

// goVersion returns the Go version (placeholder)
func goVersion() string {
	return "go1.21+"
}

// ParseLogLevel parses a string into a LogLevel (case-insensitive)
func ParseLogLevel(level string) LogLevel {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "INFO":
		return InfoLevel
	case "WARN":
		return WarnLevel
	case "ERROR":
		return ErrorLevel
	default:
		if level != "" {
			log.Printf("Warning: Unknown log level %q, defaulting to INFO", level)
		}
		return InfoLevel
	}
}
